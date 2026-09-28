package service

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestWalletHighBalanceAlwaysReservesAndSettlesOnce(t *testing.T) {
	truncate(t)
	const balance = 100000000
	seedUser(t, 95001, balance)
	seedToken(t, 95002, 95001, "reserve-test", balance)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	info := &relaycommon.RelayInfo{UserId: 95001, TokenId: 95002, TokenKey: "reserve-test", TokenUnlimited: true}
	info.UserSetting.BillingPreference = "wallet_only"
	session, apiErr := NewBillingSession(c, info, 1000)
	require.Nil(t, apiErr)
	require.False(t, session.trusted)
	require.Equal(t, balance-1000, getUserQuota(t, info.UserId))
	require.Equal(t, balance-1000, getTokenRemainQuota(t, info.TokenId))
	require.NoError(t, session.Reserve(1500))
	require.Equal(t, balance-1500, getUserQuota(t, info.UserId))
	require.NoError(t, session.Settle(800))
	require.NoError(t, session.Settle(800))
	session.Refund(c)
	require.Equal(t, balance-800, getUserQuota(t, info.UserId))
	require.Equal(t, balance-800, getTokenRemainQuota(t, info.TokenId))
}

func TestWalletRefundOnceAndRejectLateSettlement(t *testing.T) {
	truncate(t)
	seedUser(t, 95001, 10000)
	seedToken(t, 95002, 95001, "refund-test", 10000)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	info := &relaycommon.RelayInfo{UserId: 95001, TokenId: 95002, TokenKey: "refund-test"}
	info.UserSetting.BillingPreference = "wallet_only"
	session, apiErr := NewBillingSession(c, info, 1000)
	require.Nil(t, apiErr)
	session.Refund(c)
	session.Refund(c)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, DrainBillingForMigration(ctx))
	require.Equal(t, 10000, getUserQuota(t, info.UserId))
	require.Equal(t, 10000, getTokenRemainQuota(t, info.TokenId))
	require.Error(t, session.Settle(1500))
}

func TestWalletReserveRejectsTokenRaceWithoutDebitingWallet(t *testing.T) {
	truncate(t)
	seedUser(t, 95001, 10000)
	seedToken(t, 95002, 95001, "race-test", 500)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	info := &relaycommon.RelayInfo{UserId: 95001, TokenId: 95002, TokenKey: "race-test"}
	info.UserSetting.BillingPreference = "wallet_only"
	session, apiErr := NewBillingSession(c, info, 200)
	require.Nil(t, apiErr)
	require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", 95002).Update("status", common.TokenStatusDisabled).Error)
	require.Error(t, session.Reserve(300))
	require.Equal(t, 9800, getUserQuota(t, info.UserId))
	require.Equal(t, 300, getTokenRemainQuota(t, info.TokenId))
	require.Equal(t, 200, session.GetPreConsumedQuota())
}

func TestRealtimeIncrementalReservationIsIncludedInFinalSettlement(t *testing.T) {
	truncate(t)
	seedUser(t, 95001, 10000)
	seedToken(t, 95002, 95001, "realtime-test", 10000)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/v1/realtime", nil)
	info := &relaycommon.RelayInfo{UserId: 95001, TokenId: 95002, TokenKey: "realtime-test"}
	info.UserSetting.BillingPreference = "wallet_only"
	session, apiErr := NewBillingSession(c, info, 500)
	require.Nil(t, apiErr)
	require.NoError(t, session.ReserveIncremental(400))
	require.Equal(t, 9500, getUserQuota(t, info.UserId))
	require.NoError(t, session.ReserveIncremental(300))
	require.Equal(t, 9300, getUserQuota(t, info.UserId))
	require.NoError(t, session.Settle(650))
	require.NoError(t, session.Settle(650))
	require.Equal(t, 9350, getUserQuota(t, info.UserId))
	require.Equal(t, 9350, getTokenRemainQuota(t, info.TokenId))
	require.Error(t, session.ReserveIncremental(1))
}
