package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertInviteeStatsUser(t *testing.T, user *User) {
	t.Helper()
	if user.AffCode == "" {
		user.AffCode = fmt.Sprintf("aff-%d", user.Id)
	}
	require.NoError(t, DB.Create(user).Error)
}

func insertInviteeStatsTopUp(t *testing.T, topUp *TopUp) {
	t.Helper()
	if topUp.CreateTime == 0 {
		topUp.CreateTime = time.Now().Unix()
	}
	require.NoError(t, DB.Create(topUp).Error)
}

func TestGetUserInviteeStats(t *testing.T) {
	truncateTables(t)

	insertInviteeStatsUser(t, &User{Id: 1, Username: "inviter-a", Password: "hashed", Status: common.UserStatusEnabled})
	insertInviteeStatsUser(t, &User{Id: 2, Username: "invitee-b", Password: "hashed", Status: common.UserStatusEnabled, InviterId: 1, UsedQuota: 100})
	insertInviteeStatsUser(t, &User{Id: 3, Username: "invitee-c", Password: "hashed", Status: common.UserStatusEnabled, InviterId: 1, UsedQuota: 200})
	insertInviteeStatsUser(t, &User{Id: 4, Username: "invitee-d", Password: "hashed", Status: common.UserStatusEnabled, InviterId: 1, UsedQuota: 300})
	insertInviteeStatsUser(t, &User{Id: 5, Username: "unrelated", Password: "hashed", Status: common.UserStatusEnabled, InviterId: 2, UsedQuota: 400})
	require.NoError(t, DB.Delete(&User{}, 3).Error)

	insertInviteeStatsTopUp(t, &TopUp{UserId: 2, Amount: 10, Money: 7.50, TradeNo: "b-success", Status: common.TopUpStatusSuccess})
	insertInviteeStatsTopUp(t, &TopUp{UserId: 2, Amount: 10, Money: 100, TradeNo: "b-pending", Status: common.TopUpStatusPending})
	insertInviteeStatsTopUp(t, &TopUp{UserId: 2, Amount: 10, Money: 100, TradeNo: "b-failed", Status: common.TopUpStatusFailed})
	insertInviteeStatsTopUp(t, &TopUp{UserId: 2, Amount: 0, Money: 100, TradeNo: "b-subscription", Status: common.TopUpStatusSuccess})
	insertInviteeStatsTopUp(t, &TopUp{UserId: 3, Amount: 20, Money: 2.25, TradeNo: "c-success", Status: common.TopUpStatusSuccess})
	insertInviteeStatsTopUp(t, &TopUp{UserId: 5, Amount: 30, Money: 999, TradeNo: "unrelated-success", Status: common.TopUpStatusSuccess})

	stats, err := GetUserInviteeStats(1)
	require.NoError(t, err)
	assert.Equal(t, 3, stats.InviteeCount)
	assert.InDelta(t, 9.75, stats.InviteeTotalRechargeMoney, 0.0001)
	assert.Equal(t, 600, stats.InviteeTotalUsedQuota)

	emptyStats, err := GetUserInviteeStats(999)
	require.NoError(t, err)
	assert.Equal(t, UserInviteeStats{}, emptyStats)
}
