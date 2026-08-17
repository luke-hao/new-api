package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setAffiliateRebateTestPercent(t *testing.T, value string) {
	t.Helper()
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	previous, existed := common.OptionMap[AffiliateTopupRebatePercentOption]
	common.OptionMap[AffiliateTopupRebatePercentOption] = value
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		if existed {
			common.OptionMap[AffiliateTopupRebatePercentOption] = previous
		} else {
			delete(common.OptionMap, AffiliateTopupRebatePercentOption)
		}
		common.OptionMapRWMutex.Unlock()
	})
}

func createAffiliateRebateUsers(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.Create(&User{Id: 1, Username: "inviter", AffCode: "invite-a", Status: common.UserStatusEnabled}).Error)
	require.NoError(t, DB.Create(&User{Id: 2, Username: "zhangsan", AffCode: "invite-b", Status: common.UserStatusEnabled, InviterId: 1}).Error)
}

func TestRechargeEpayAppliesAffiliateRebateOnce(t *testing.T) {
	truncateTables(t)
	setAffiliateRebateTestPercent(t, "5")
	previousQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500000
	t.Cleanup(func() { common.QuotaPerUnit = previousQuotaPerUnit })
	createAffiliateRebateUsers(t)

	topUp := &TopUp{
		UserId:          2,
		Amount:          100,
		Money:           100,
		TradeNo:         "affiliate-epay-once",
		PaymentMethod:   "alipay",
		PaymentProvider: PaymentProviderEpay,
		Status:          common.TopUpStatusPending,
	}
	require.NoError(t, topUp.Insert())

	require.NoError(t, RechargeEpay(topUp.TradeNo, "alipay", "127.0.0.1"))
	require.NoError(t, RechargeEpay(topUp.TradeNo, "alipay", "127.0.0.1"))

	var inviter, invitee User
	require.NoError(t, DB.First(&inviter, 1).Error)
	require.NoError(t, DB.First(&invitee, 2).Error)
	assert.Equal(t, 2500000, inviter.AffQuota)
	assert.Equal(t, 2500000, inviter.AffHistoryQuota)
	assert.Equal(t, 50000000, invitee.Quota)

	var rebates []AffiliateRebate
	require.NoError(t, DB.Find(&rebates).Error)
	require.Len(t, rebates, 1)
	assert.Equal(t, 5.0, rebates[0].RebatePercent)
	assert.Equal(t, 2500000, rebates[0].RebateQuota)
}

func TestAffiliateRebateUsesPaymentTimePercentAndFloors(t *testing.T) {
	truncateTables(t)
	setAffiliateRebateTestPercent(t, "5.25")
	previousQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 100
	t.Cleanup(func() { common.QuotaPerUnit = previousQuotaPerUnit })
	createAffiliateRebateUsers(t)

	topUp := &TopUp{UserId: 2, Amount: 1, Money: 1.99, TradeNo: "affiliate-floor", PaymentProvider: PaymentProviderWaffo, Status: common.TopUpStatusSuccess}
	require.NoError(t, topUp.Insert())
	var rebate *AffiliateRebate
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		var err error
		rebate, err = applyAffiliateTopupRebate(tx, topUp)
		return err
	}))
	require.NotNil(t, rebate)
	expected := decimal.NewFromFloat(1.99).Mul(decimal.NewFromFloat(5.25)).Div(decimal.NewFromInt(100)).Mul(decimal.NewFromInt(100)).IntPart()
	assert.Equal(t, int(expected), rebate.RebateQuota)
}

func TestAffiliateRebateSkipsIneligibleTopups(t *testing.T) {
	truncateTables(t)
	setAffiliateRebateTestPercent(t, "5")
	createAffiliateRebateUsers(t)

	tests := []TopUp{
		{UserId: 2, Amount: 100, Money: 100, TradeNo: "pending", PaymentProvider: PaymentProviderEpay, Status: common.TopUpStatusPending},
		{UserId: 2, Amount: 0, Money: 100, TradeNo: "subscription", PaymentProvider: PaymentProviderEpay, Status: common.TopUpStatusSuccess},
		{UserId: 2, Amount: 100, Money: 100, TradeNo: "balance", PaymentProvider: PaymentProviderBalance, Status: common.TopUpStatusSuccess},
	}
	for i := range tests {
		require.NoError(t, tests[i].Insert())
		require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
			rebate, err := applyAffiliateTopupRebate(tx, &tests[i])
			assert.Nil(t, rebate)
			return err
		}))
	}
	var count int64
	require.NoError(t, DB.Model(&AffiliateRebate{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestAffiliateRebateAllWalletProvidersAreIdempotent(t *testing.T) {
	truncateTables(t)
	setAffiliateRebateTestPercent(t, "5")
	previousQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500000
	t.Cleanup(func() { common.QuotaPerUnit = previousQuotaPerUnit })
	createAffiliateRebateUsers(t)

	topUps := []*TopUp{
		{UserId: 2, Amount: 100, Money: 100, TradeNo: "stripe-rebate", PaymentMethod: PaymentMethodStripe, PaymentProvider: PaymentProviderStripe, Status: common.TopUpStatusPending},
		{UserId: 2, Amount: 50000000, Money: 100, TradeNo: "creem-rebate", PaymentMethod: PaymentMethodCreem, PaymentProvider: PaymentProviderCreem, Status: common.TopUpStatusPending},
		{UserId: 2, Amount: 100, Money: 100, TradeNo: "waffo-rebate", PaymentMethod: PaymentMethodWaffo, PaymentProvider: PaymentProviderWaffo, Status: common.TopUpStatusPending},
		{UserId: 2, Amount: 100, Money: 100, TradeNo: "pancake-rebate", PaymentMethod: PaymentMethodWaffoPancake, PaymentProvider: PaymentProviderWaffoPancake, Status: common.TopUpStatusPending},
	}
	for _, topUp := range topUps {
		require.NoError(t, topUp.Insert())
	}

	for i := 0; i < 2; i++ {
		require.NoError(t, Recharge(topUps[0].TradeNo, "cus_test", "127.0.0.1"))
		require.NoError(t, RechargeCreem(topUps[1].TradeNo, "", "", "127.0.0.1"))
		require.NoError(t, RechargeWaffo(topUps[2].TradeNo, "127.0.0.1"))
		require.NoError(t, RechargeWaffoPancake(topUps[3].TradeNo))
	}

	var inviter User
	require.NoError(t, DB.First(&inviter, 1).Error)
	assert.Equal(t, 10000000, inviter.AffQuota)
	assert.Equal(t, 10000000, inviter.AffHistoryQuota)
	var count int64
	require.NoError(t, DB.Model(&AffiliateRebate{}).Count(&count).Error)
	assert.EqualValues(t, 4, count)
}

func TestAffiliateRebateFailureRollsBackTopup(t *testing.T) {
	truncateTables(t)
	setAffiliateRebateTestPercent(t, "100")
	previousQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500000
	t.Cleanup(func() { common.QuotaPerUnit = previousQuotaPerUnit })
	createAffiliateRebateUsers(t)

	topUp := &TopUp{UserId: 2, Amount: 1, Money: 1e20, TradeNo: "rebate-overflow", PaymentMethod: "alipay", PaymentProvider: PaymentProviderEpay, Status: common.TopUpStatusPending}
	require.NoError(t, topUp.Insert())
	require.Error(t, RechargeEpay(topUp.TradeNo, "alipay", "127.0.0.1"))

	stored := GetTopUpByTradeNo(topUp.TradeNo)
	require.NotNil(t, stored)
	assert.Equal(t, common.TopUpStatusPending, stored.Status)
	var invitee User
	require.NoError(t, DB.First(&invitee, 2).Error)
	assert.Zero(t, invitee.Quota)
}

func TestManualCompleteTopUpAppliesAffiliateRebateOnce(t *testing.T) {
	truncateTables(t)
	setAffiliateRebateTestPercent(t, "5")
	previousQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500000
	t.Cleanup(func() { common.QuotaPerUnit = previousQuotaPerUnit })
	createAffiliateRebateUsers(t)

	topUp := &TopUp{UserId: 2, Amount: 20, Money: 20, TradeNo: "manual-rebate", PaymentMethod: "alipay", PaymentProvider: PaymentProviderEpay, Status: common.TopUpStatusPending}
	require.NoError(t, topUp.Insert())
	require.NoError(t, ManualCompleteTopUp(topUp.TradeNo, "127.0.0.1"))
	require.NoError(t, ManualCompleteTopUp(topUp.TradeNo, "127.0.0.1"))

	var inviter User
	require.NoError(t, DB.First(&inviter, 1).Error)
	assert.Equal(t, 500000, inviter.AffQuota)
	var count int64
	require.NoError(t, DB.Model(&AffiliateRebate{}).Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

func TestGetUserAffiliateRebatesMasksInvitee(t *testing.T) {
	truncateTables(t)
	createAffiliateRebateUsers(t)
	topUpId := 11
	require.NoError(t, DB.Create(&AffiliateRebate{TopUpId: &topUpId, SourceType: AffiliateRebateSourceTopup, InviterId: 1, InviteeId: 2, PaidMoney: 20, RebatePercent: 5, RebateQuota: 500000, CreatedAt: 123}).Error)

	pageInfo := &common.PageInfo{Page: 1, PageSize: 10}
	items, total, err := GetUserAffiliateRebates(1, pageInfo)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	assert.Equal(t, "z***n", items[0].Invitee)

	require.NoError(t, DB.Delete(&User{}, 2).Error)
	items, _, err = GetUserAffiliateRebates(1, pageInfo)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Empty(t, items[0].Invitee)
}

func TestRedeemAppliesQuotaBasedAffiliateRebateOnce(t *testing.T) {
	truncateTables(t)
	setAffiliateRebateTestPercent(t, "5")
	createAffiliateRebateUsers(t)

	redemption := &Redemption{
		Key:    "affiliate-redemption-code",
		Name:   "affiliate redemption",
		Status: common.RedemptionCodeStatusEnabled,
		Quota:  10000000,
	}
	require.NoError(t, redemption.Insert())

	quota, err := Redeem(redemption.Key, 2)
	require.NoError(t, err)
	assert.Equal(t, 10000000, quota)
	_, err = Redeem(redemption.Key, 2)
	assert.Error(t, err)

	var inviter, invitee User
	require.NoError(t, DB.First(&inviter, 1).Error)
	require.NoError(t, DB.First(&invitee, 2).Error)
	assert.Equal(t, 500000, inviter.AffQuota)
	assert.Equal(t, 500000, inviter.AffHistoryQuota)
	assert.Equal(t, 10000000, invitee.Quota)

	var rebates []AffiliateRebate
	require.NoError(t, DB.Find(&rebates).Error)
	require.Len(t, rebates, 1)
	assert.Equal(t, AffiliateRebateSourceRedemption, rebates[0].SourceType)
	assert.Equal(t, 10000000, rebates[0].SourceQuota)
	assert.Equal(t, 500000, rebates[0].RebateQuota)
}
