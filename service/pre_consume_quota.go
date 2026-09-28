package service

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
)

func ReturnPreConsumedQuota(c *gin.Context, relayInfo *relaycommon.RelayInfo) {
	if relayInfo.FinalPreConsumedQuota < 0 {
		common.SysLog(fmt.Sprintf("skip returning negative pre-consumed quota: userId=%d quota=%d", relayInfo.UserId, relayInfo.FinalPreConsumedQuota))
		return
	}
	if relayInfo.FinalPreConsumedQuota > 0 {
		logger.LogInfo(c, fmt.Sprintf("用户 %d 请求失败, 返还预扣费额度 %s", relayInfo.UserId, logger.FormatQuota(relayInfo.FinalPreConsumedQuota)))
		finishRefund := refundWork.begin()
		relayInfoCopy := *relayInfo
		gopool.Go(func() {
			refundErr := errors.New("refund task did not complete")
			defer func() { finishRefund(refundErr) }()

			err := refundPostConsumeQuota(&relayInfoCopy, relayInfoCopy.FinalPreConsumedQuota)
			refundErr = err
			if err != nil {
				common.SysLog("error return pre-consumed quota: " + err.Error())
			}
		})
	}
}

// PreConsumeQuota checks if the user has enough quota to pre-consume.
// It returns the pre-consumed quota if successful, or an error if not.
func PreConsumeQuota(c *gin.Context, preConsumedQuota int, relayInfo *relaycommon.RelayInfo) *types.NewAPIError {
	if preConsumedQuota < 0 {
		return types.NewErrorWithStatusCode(fmt.Errorf("pre-consumed quota cannot be negative"), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
	}
	userQuota, err := model.GetUserQuota(relayInfo.UserId, true)
	if err != nil {
		return types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
	}
	if userQuota <= 0 {
		return types.NewErrorWithStatusCode(fmt.Errorf("您在本站点的所余资金不足够了, 剩余额度: %s", logger.FormatQuota(userQuota)), types.ErrorCodeInsufficientUserQuota, http.StatusForbidden, types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
	}
	if preConsumedQuota > userQuota {
		return types.NewErrorWithStatusCode(fmt.Errorf("您在本站点的所余资金不足够了, 剩余额度: %s, 需要预扣费额度: %s", logger.FormatQuota(userQuota), logger.FormatQuota(preConsumedQuota)), types.ErrorCodeInsufficientUserQuota, http.StatusForbidden, types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
	}

	relayInfo.UserQuota = userQuota
	if err := model.ReserveWalletQuota(relayInfo.UserId, relayInfo.TokenId, relayInfo.TokenKey, preConsumedQuota, relayInfo.IsPlayground); err != nil {
		return walletReservationError(err)
	}
	relayInfo.FinalPreConsumedQuota = preConsumedQuota
	return nil
}
