package service

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// BillingSession — 统一计费会话
// ---------------------------------------------------------------------------

// BillingSession 封装单次请求的预扣费/结算/退款生命周期。
// 实现 relaycommon.BillingSettler 接口。
type BillingSession struct {
	operationID      string
	finalQuota       *int
	incrementalQuota int
	relayInfo        *relaycommon.RelayInfo
	funding          FundingSource
	preConsumedQuota int  // 实际预扣额度（信任用户可能为 0）
	tokenConsumed    int  // 令牌额度实际扣减量
	extraReserved    int  // 发送前补充预扣的额度（订阅退款时需要单独回滚）
	trusted          bool // 是否命中信任额度旁路
	fundingSettled   bool // funding.Settle 已成功，资金来源已提交
	settled          bool // Settle 全部完成（资金 + 令牌）
	refunded         bool // Refund 已调用
	mu               sync.Mutex
}

// Settle 根据实际消耗额度进行结算。
// 钱包或订阅与令牌在同一个事务中结算，失败意图保留并重试。
func (s *BillingSession) Settle(actualQuota int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.settled {
		return nil
	}
	if s.refunded {
		return fmt.Errorf("cannot settle a refunded billing session")
	}
	if actualQuota < 0 {
		return fmt.Errorf("actual quota cannot be negative")
	}
	if s.preConsumedQuota < 0 {
		return fmt.Errorf("pre-consumed quota cannot be negative")
	}

	if s.finalQuota == nil {
		chosen := actualQuota
		s.finalQuota = &chosen
	}
	if *s.finalQuota != actualQuota {
		return fmt.Errorf("billing final quota already chosen")
	}
	if s.fundingSettled {
		err := model.ApplyBillingAdjustment(s.operationID)
		s.settled = err == nil
		return err
	}
	delta, err := common.SafeAddInt("billing quota delta", actualQuota, -s.preConsumedQuota)
	if err != nil {
		return err
	}
	if s.operationID == "" {
		s.operationID = "session:" + uuid.NewString()
	}
	a := s.adjustment(delta)
	if s.relayInfo.ChannelMeta != nil {
		a.ChannelID = s.relayInfo.ChannelId
	}
	a.UsedQuota = actualQuota
	if actualQuota > 0 {
		a.RequestCount = 1
	}
	if err := model.QueueBillingAdjustment(a); err != nil {
		return err
	}
	// A durable final intent must never be followed by a second refund.
	s.fundingSettled = true
	if s.funding.Source() == BillingSourceSubscription {
		s.relayInfo.SubscriptionPostDelta += int64(delta)
	}
	err = model.ApplyBillingAdjustment(a.ID)
	s.settled = err == nil
	return err
}

func (s *BillingSession) adjustment(delta int) *model.BillingAdjustment {
	tokenID := s.relayInfo.TokenId
	if s.relayInfo.IsPlayground {
		tokenID = 0
	}
	subID := 0
	if s.funding.Source() == BillingSourceSubscription {
		subID = s.relayInfo.SubscriptionId
	}
	return &model.BillingAdjustment{ID: s.operationID, UserID: s.relayInfo.UserId, TokenID: tokenID, SubscriptionID: subID, Delta: delta}
}

func (s *BillingSession) Refund(c *gin.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.settled || s.refunded || !s.needsRefundLocked() {
		return
	}
	if s.operationID == "" {
		s.operationID = "session:" + uuid.NewString()
	}
	a := s.adjustment(-s.preConsumedQuota)
	if err := model.QueueBillingAdjustment(a); err != nil {
		common.SysLog("cannot persist refund intent: " + err.Error())
		return
	}
	s.refunded = true
	if err := model.ApplyBillingAdjustment(a.ID); err != nil {
		common.SysLog("refund pending durable retry: " + err.Error())
	}
}

// NeedsRefund 返回是否存在需要退还的预扣状态。
func (s *BillingSession) NeedsRefund() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.needsRefundLocked()
}

func (s *BillingSession) needsRefundLocked() bool {
	if s.settled || s.refunded || s.fundingSettled || s.finalQuota != nil {
		// fundingSettled 时资金来源已提交结算，不能再退预扣费
		return false
	}
	if s.tokenConsumed > 0 {
		return true
	}
	// 订阅可能在 tokenConsumed=0 时仍预扣了额度
	if sub, ok := s.funding.(*SubscriptionFunding); ok && sub.preConsumed > 0 {
		return true
	}
	return false
}

// GetPreConsumedQuota 返回实际预扣的额度。
func (s *BillingSession) GetPreConsumedQuota() int {
	return s.preConsumedQuota
}

func (s *BillingSession) Reserve(targetQuota int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reserveLocked(targetQuota)
}

// ReserveIncremental extends the reservation for usage reported during a
// realtime session. The final settlement accounts for these funds exactly once.
func (s *BillingSession) ReserveIncremental(amount int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if amount < 0 {
		return fmt.Errorf("negative incremental quota")
	}
	target, err := common.SafeAddInt("incremental quota", s.incrementalQuota, amount)
	if err != nil {
		return err
	}
	if err = s.reserveLocked(target); err != nil {
		return err
	}
	s.incrementalQuota = target
	return nil
}

func (s *BillingSession) reserveLocked(targetQuota int) error {
	if targetQuota < 0 {
		return fmt.Errorf("target quota cannot be negative")
	}
	if s.settled || s.refunded || s.finalQuota != nil {
		return fmt.Errorf("billing session is already final")
	}
	if targetQuota <= s.preConsumedQuota {
		return nil
	}

	delta := targetQuota - s.preConsumedQuota
	if delta <= 0 {
		return nil
	}

	newPreConsumedQuota, err := common.SafeAddInt("pre-consumed quota", s.preConsumedQuota, delta)
	if err != nil {
		return err
	}
	newTokenConsumed, err := common.SafeAddInt("token consumed quota", s.tokenConsumed, delta)
	if err != nil {
		return err
	}
	newExtraReserved, err := common.SafeAddInt("extra reserved quota", s.extraReserved, delta)
	if err != nil {
		return err
	}

	if funding, ok := s.funding.(*WalletFunding); ok {
		if err := model.ReserveWalletQuota(funding.userId, s.relayInfo.TokenId, s.relayInfo.TokenKey, delta, s.relayInfo.IsPlayground); err != nil {
			return walletReservationError(err)
		}
		funding.consumed += delta
		s.preConsumedQuota = newPreConsumedQuota
		s.tokenConsumed = newTokenConsumed
		s.extraReserved = newExtraReserved
		s.syncRelayInfo()
		return nil
	}

	if err := model.ReserveSubscriptionAdditional(s.relayInfo.UserId, s.relayInfo.TokenId, s.relayInfo.TokenKey, s.relayInfo.SubscriptionId, delta, s.relayInfo.IsPlayground); err != nil {
		return walletReservationError(err)
	}

	s.preConsumedQuota = newPreConsumedQuota
	s.tokenConsumed = newTokenConsumed
	s.extraReserved = newExtraReserved
	s.syncRelayInfo()
	return nil
}

// ---------------------------------------------------------------------------
// PreConsume — 统一预扣费入口（含信任额度旁路）
// ---------------------------------------------------------------------------

// preConsume 执行预扣费：信任检查 -> 令牌预扣 -> 资金来源预扣。
// 任一步骤失败时原子回滚已完成的步骤。
func (s *BillingSession) preConsume(c *gin.Context, quota int) *types.NewAPIError {
	if quota < 0 {
		return types.NewErrorWithStatusCode(fmt.Errorf("pre-consumed quota cannot be negative"), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
	}

	// Wallet reservations commit both balances before any upstream dispatch.
	if funding, ok := s.funding.(*WalletFunding); ok {
		if err := model.ReserveWalletQuota(funding.userId, s.relayInfo.TokenId, s.relayInfo.TokenKey, quota, s.relayInfo.IsPlayground); err != nil {
			return walletReservationError(err)
		}
		funding.consumed = quota
		s.tokenConsumed = quota
		s.preConsumedQuota = quota
		s.syncRelayInfo()
		logger.LogInfo(c, fmt.Sprintf("用户 %d 已预扣费 %s (funding=wallet)", s.relayInfo.UserId, logger.FormatQuota(quota)))
		return nil
	}

	sub, ok := s.funding.(*SubscriptionFunding)
	if !ok {
		return types.NewError(fmt.Errorf("unsupported funding source"), types.ErrorCodeUpdateDataError)
	}
	tokenID := s.relayInfo.TokenId
	if s.relayInfo.IsPlayground {
		tokenID = 0
	}
	res, err := model.ReserveSubscriptionQuota(sub.requestId, sub.userId, sub.modelName, sub.amount, tokenID)
	if err != nil {
		if errors.Is(err, model.ErrTokenReservation) {
			return walletReservationError(err)
		}
		if strings.Contains(err.Error(), "no active subscription") || strings.Contains(err.Error(), "subscription quota insufficient") {
			return types.NewErrorWithStatusCode(err, types.ErrorCodeInsufficientUserQuota, http.StatusForbidden, types.ErrOptionWithSkipRetry())
		}
		return types.NewError(err, types.ErrorCodeUpdateDataError, types.ErrOptionWithSkipRetry())
	}
	sub.subscriptionId = res.UserSubscriptionId
	sub.preConsumed = res.PreConsumed
	sub.AmountTotal = res.AmountTotal
	sub.AmountUsedAfter = res.AmountUsedAfter
	if plan, err := model.GetSubscriptionPlanInfoByUserSubscriptionId(res.UserSubscriptionId); err == nil {
		sub.PlanId = plan.PlanId
		sub.PlanTitle = plan.PlanTitle
	}
	s.preConsumedQuota = int(res.PreConsumed)
	s.tokenConsumed = s.preConsumedQuota
	s.syncRelayInfo()
	return nil
}

// syncRelayInfo 将 BillingSession 的状态同步到 RelayInfo 的兼容字段上。
func (s *BillingSession) syncRelayInfo() {
	info := s.relayInfo
	info.FinalPreConsumedQuota = s.preConsumedQuota
	info.BillingSource = s.funding.Source()

	if sub, ok := s.funding.(*SubscriptionFunding); ok {
		info.SubscriptionId = sub.subscriptionId
		info.SubscriptionPreConsumed = sub.preConsumed + int64(s.extraReserved)
		info.SubscriptionPostDelta = 0
		info.SubscriptionAmountTotal = sub.AmountTotal
		info.SubscriptionAmountUsedAfterPreConsume = sub.AmountUsedAfter + int64(s.extraReserved)
		info.SubscriptionPlanId = sub.PlanId
		info.SubscriptionPlanTitle = sub.PlanTitle
	} else {
		info.SubscriptionId = 0
		info.SubscriptionPreConsumed = 0
	}
}

// ---------------------------------------------------------------------------
// NewBillingSession 工厂 — 根据计费偏好创建会话并处理回退
// ---------------------------------------------------------------------------

// NewBillingSession 根据用户计费偏好创建 BillingSession，处理 subscription_first / wallet_first 的回退。
func NewBillingSession(c *gin.Context, relayInfo *relaycommon.RelayInfo, preConsumedQuota int) (*BillingSession, *types.NewAPIError) {
	if relayInfo == nil {
		return nil, types.NewError(fmt.Errorf("relayInfo is nil"), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	if preConsumedQuota < 0 {
		return nil, types.NewErrorWithStatusCode(fmt.Errorf("pre-consumed quota cannot be negative"), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
	}

	pref := common.NormalizeBillingPreference(relayInfo.UserSetting.BillingPreference)

	// 钱包路径需要先检查用户额度
	tryWallet := func() (*BillingSession, *types.NewAPIError) {
		userQuota, err := model.GetUserQuota(relayInfo.UserId, true)
		if err != nil {
			return nil, types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
		}
		if userQuota <= 0 {
			return nil, types.NewErrorWithStatusCode(
				fmt.Errorf("您在本站点的所余资金不足够了, 剩余额度: %s", logger.FormatQuota(userQuota)),
				types.ErrorCodeInsufficientUserQuota, http.StatusForbidden,
				types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
		}
		if preConsumedQuota > userQuota {
			return nil, types.NewErrorWithStatusCode(
				fmt.Errorf("您在本站点的所余资金不足够了, 剩余额度: %s, 需要预扣费额度: %s", logger.FormatQuota(userQuota), logger.FormatQuota(preConsumedQuota)),
				types.ErrorCodeInsufficientUserQuota, http.StatusForbidden,
				types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
		}
		relayInfo.UserQuota = userQuota

		session := &BillingSession{
			relayInfo: relayInfo,
			funding:   &WalletFunding{userId: relayInfo.UserId},
		}
		if apiErr := session.preConsume(c, preConsumedQuota); apiErr != nil {
			return nil, apiErr
		}
		return session, nil
	}

	trySubscription := func() (*BillingSession, *types.NewAPIError) {
		subConsume := int64(preConsumedQuota)
		if subConsume <= 0 {
			subConsume = 1
		}
		session := &BillingSession{
			relayInfo: relayInfo,
			funding: &SubscriptionFunding{
				requestId: relayInfo.RequestId,
				userId:    relayInfo.UserId,
				modelName: relayInfo.OriginModelName,
				amount:    subConsume,
			},
		}
		// 必须传 subConsume 而非 preConsumedQuota，保证 SubscriptionFunding.amount、
		// preConsume 参数和 FinalPreConsumedQuota 三者一致，避免订阅多扣费。
		if apiErr := session.preConsume(c, int(subConsume)); apiErr != nil {
			return nil, apiErr
		}
		return session, nil
	}

	switch pref {
	case "subscription_only":
		return trySubscription()
	case "wallet_only":
		return tryWallet()
	case "wallet_first":
		session, err := tryWallet()
		if err != nil {
			if err.GetErrorCode() == types.ErrorCodeInsufficientUserQuota {
				return trySubscription()
			}
			return nil, err
		}
		return session, nil
	case "subscription_first":
		fallthrough
	default:
		hasSub, subCheckErr := model.HasActiveUserSubscription(relayInfo.UserId)
		if subCheckErr != nil {
			return nil, types.NewError(subCheckErr, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
		}
		if !hasSub {
			return tryWallet()
		}
		session, apiErr := trySubscription()
		if apiErr != nil {
			if apiErr.GetErrorCode() == types.ErrorCodeInsufficientUserQuota {
				return tryWallet()
			}
			return nil, apiErr
		}
		return session, nil
	}
}

// walletReservationError keeps local quota rejection distinct from supplier
// failures so a concurrent balance rejection never triggers upstream retries.
func walletReservationError(err error) *types.NewAPIError {
	if errors.Is(err, model.ErrWalletReservation) {
		return types.NewErrorWithStatusCode(err, types.ErrorCodeInsufficientUserQuota, http.StatusForbidden, types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
	}
	if errors.Is(err, model.ErrTokenReservation) {
		return types.NewErrorWithStatusCode(err, types.ErrorCodePreConsumeTokenQuotaFailed, http.StatusForbidden, types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
	}
	return types.NewError(err, types.ErrorCodeUpdateDataError, types.ErrOptionWithSkipRetry())
}
