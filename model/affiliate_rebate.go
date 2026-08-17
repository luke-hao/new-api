package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const AffiliateTopupRebatePercentOption = "AffiliateTopupRebatePercent"

const (
	AffiliateRebateSourceTopup      = "topup"
	AffiliateRebateSourceRedemption = "redemption"
)

type AffiliateRebate struct {
	Id            int     `json:"id"`
	TopUpId       *int    `json:"-" gorm:"uniqueIndex"`
	RedemptionId  *int    `json:"-" gorm:"uniqueIndex"`
	SourceType    string  `json:"source_type" gorm:"type:varchar(20);not null;default:'topup';index"`
	SourceQuota   int     `json:"source_quota" gorm:"not null;default:0"`
	InviterId     int     `json:"-" gorm:"index"`
	InviteeId     int     `json:"-" gorm:"index"`
	PaidMoney     float64 `json:"paid_money" gorm:"type:decimal(20,8);not null"`
	RebatePercent float64 `json:"rebate_percent" gorm:"type:decimal(5,2);not null"`
	RebateQuota   int     `json:"rebate_quota" gorm:"not null"`
	CreatedAt     int64   `json:"created_at" gorm:"autoCreateTime"`
}

type AffiliateRebateItem struct {
	Id            int     `json:"id"`
	Invitee       string  `json:"invitee"`
	SourceType    string  `json:"source_type"`
	SourceQuota   int     `json:"source_quota"`
	PaidMoney     float64 `json:"paid_money"`
	RebatePercent float64 `json:"rebate_percent"`
	RebateQuota   int     `json:"rebate_quota"`
	CreatedAt     int64   `json:"created_at"`
}

func GetAffiliateTopupRebatePercent() decimal.Decimal {
	common.OptionMapRWMutex.RLock()
	raw := common.OptionMap[AffiliateTopupRebatePercentOption]
	common.OptionMapRWMutex.RUnlock()

	percent, err := decimal.NewFromString(strings.TrimSpace(raw))
	if err != nil || percent.IsNegative() || percent.GreaterThan(decimal.NewFromInt(100)) {
		return decimal.Zero
	}
	return percent
}

func isAffiliateRebateProvider(provider string) bool {
	switch provider {
	case PaymentProviderEpay, PaymentProviderStripe, PaymentProviderCreem,
		PaymentProviderWaffo, PaymentProviderWaffoPancake:
		return true
	default:
		return false
	}
}

func applyAffiliateTopupRebate(tx *gorm.DB, topUp *TopUp) (*AffiliateRebate, error) {
	if topUp == nil || topUp.Id <= 0 || topUp.Status != common.TopUpStatusSuccess || topUp.Amount <= 0 || topUp.Money <= 0 || !isAffiliateRebateProvider(topUp.PaymentProvider) {
		return nil, nil
	}

	percent := GetAffiliateTopupRebatePercent()
	if !percent.IsPositive() {
		return nil, nil
	}

	var invitee User
	if err := tx.Unscoped().Select("id", "inviter_id").Where("id = ?", topUp.UserId).First(&invitee).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if invitee.InviterId <= 0 || invitee.InviterId == invitee.Id {
		return nil, nil
	}

	var inviter User
	if err := tx.Select("id").Where("id = ?", invitee.InviterId).First(&inviter).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	rebateValue := decimal.NewFromFloat(topUp.Money).
		Mul(percent).
		Div(decimal.NewFromInt(100)).
		Mul(decimal.NewFromFloat(common.QuotaPerUnit))
	rebateQuota, err := common.SafeDecimalToNonNegativeInt("affiliate topup rebate quota", rebateValue)
	if err != nil {
		return nil, err
	}
	if rebateQuota <= 0 {
		return nil, nil
	}

	rebate := &AffiliateRebate{
		TopUpId:       &topUp.Id,
		SourceType:    AffiliateRebateSourceTopup,
		InviterId:     inviter.Id,
		InviteeId:     invitee.Id,
		PaidMoney:     topUp.Money,
		RebatePercent: percent.InexactFloat64(),
		RebateQuota:   rebateQuota,
		CreatedAt:     common.GetTimestamp(),
	}
	if err := tx.Create(rebate).Error; err != nil {
		return nil, err
	}
	result := tx.Model(&User{}).Where("id = ?", inviter.Id).Updates(map[string]interface{}{
		"aff_quota":   gorm.Expr("aff_quota + ?", rebateQuota),
		"aff_history": gorm.Expr("aff_history + ?", rebateQuota),
	})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, fmt.Errorf("affiliate inviter %d not found", inviter.Id)
	}
	return rebate, nil
}

func applyAffiliateRedemptionRebate(tx *gorm.DB, redemption *Redemption, inviteeId int) (*AffiliateRebate, error) {
	if redemption == nil || redemption.Id <= 0 || redemption.Quota <= 0 || redemption.Status != common.RedemptionCodeStatusUsed {
		return nil, nil
	}
	percent := GetAffiliateTopupRebatePercent()
	if !percent.IsPositive() {
		return nil, nil
	}

	var invitee User
	if err := tx.Unscoped().Select("id", "inviter_id").Where("id = ?", inviteeId).First(&invitee).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if invitee.InviterId <= 0 || invitee.InviterId == invitee.Id {
		return nil, nil
	}

	var inviter User
	if err := tx.Select("id").Where("id = ?", invitee.InviterId).First(&inviter).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	rebateQuota, err := common.SafeDecimalToNonNegativeInt(
		"affiliate redemption rebate quota",
		decimal.NewFromInt(int64(redemption.Quota)).Mul(percent).Div(decimal.NewFromInt(100)),
	)
	if err != nil {
		return nil, err
	}
	if rebateQuota <= 0 {
		return nil, nil
	}

	rebate := &AffiliateRebate{
		RedemptionId:  &redemption.Id,
		SourceType:    AffiliateRebateSourceRedemption,
		SourceQuota:   redemption.Quota,
		InviterId:     inviter.Id,
		InviteeId:     invitee.Id,
		RebatePercent: percent.InexactFloat64(),
		RebateQuota:   rebateQuota,
		CreatedAt:     common.GetTimestamp(),
	}
	if err := tx.Create(rebate).Error; err != nil {
		return nil, err
	}
	result := tx.Model(&User{}).Where("id = ?", inviter.Id).Updates(map[string]interface{}{
		"aff_quota":   gorm.Expr("aff_quota + ?", rebateQuota),
		"aff_history": gorm.Expr("aff_history + ?", rebateQuota),
	})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, fmt.Errorf("affiliate inviter %d not found", inviter.Id)
	}
	return rebate, nil
}

func recordAffiliateRebateLog(rebate *AffiliateRebate) {
	if rebate == nil {
		return
	}
	content := fmt.Sprintf("邀请用户充值返利到账：实付 %.2f，返利比例 %.2f%%，获得 %s", rebate.PaidMoney, rebate.RebatePercent, logger.LogQuota(rebate.RebateQuota))
	if rebate.SourceType == AffiliateRebateSourceRedemption {
		content = fmt.Sprintf("邀请用户兑换码返利到账：兑换 %s，返利比例 %.2f%%，获得 %s", logger.LogQuota(rebate.SourceQuota), rebate.RebatePercent, logger.LogQuota(rebate.RebateQuota))
	}
	RecordLog(rebate.InviterId, LogTypeSystem, content)
}

func maskAffiliateUsername(username string) string {
	runes := []rune(strings.TrimSpace(username))
	if len(runes) <= 2 {
		return "***"
	}
	return string(runes[0]) + "***" + string(runes[len(runes)-1])
}

func GetUserAffiliateRebates(inviterId int, pageInfo *common.PageInfo) ([]AffiliateRebateItem, int64, error) {
	if inviterId <= 0 {
		return []AffiliateRebateItem{}, 0, nil
	}

	query := DB.Model(&AffiliateRebate{}).Where("inviter_id = ?", inviterId)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rebates []AffiliateRebate
	if err := DB.Where("inviter_id = ?", inviterId).Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&rebates).Error; err != nil {
		return nil, 0, err
	}

	inviteeIds := make([]int, 0, len(rebates))
	for _, rebate := range rebates {
		inviteeIds = append(inviteeIds, rebate.InviteeId)
	}
	invitees := make(map[int]User, len(inviteeIds))
	if len(inviteeIds) > 0 {
		var users []User
		if err := DB.Unscoped().Select("id", "username", "deleted_at").Where("id IN ?", inviteeIds).Find(&users).Error; err != nil {
			return nil, 0, err
		}
		for _, user := range users {
			invitees[user.Id] = user
		}
	}

	items := make([]AffiliateRebateItem, 0, len(rebates))
	for _, rebate := range rebates {
		invitee := ""
		if user, ok := invitees[rebate.InviteeId]; ok && !user.DeletedAt.Valid {
			invitee = maskAffiliateUsername(user.Username)
		}
		items = append(items, AffiliateRebateItem{
			Id:            rebate.Id,
			Invitee:       invitee,
			SourceType:    rebate.SourceType,
			SourceQuota:   rebate.SourceQuota,
			PaidMoney:     rebate.PaidMoney,
			RebatePercent: rebate.RebatePercent,
			RebateQuota:   rebate.RebateQuota,
			CreatedAt:     rebate.CreatedAt,
		})
	}
	return items, total, nil
}
