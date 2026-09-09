package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func interruptedBillingFixture(t *testing.T, source string, pre int) (*gin.Context, *relaycommon.RelayInfo, *BillingSession) {
	t.Helper()
	truncate(t)
	const userID, tokenID, channelID, subscriptionID = 91001, 91002, 91003, 91004
	const balance = 100000000
	wallet := balance
	if source == BillingSourceWallet {
		wallet -= pre
	}
	seedUser(t, userID, wallet)
	seedToken(t, tokenID, userID, "interrupted-fixture-token", balance-pre)
	seedChannel(t, channelID)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	reqCtx, cancel := context.WithCancel(context.Background())
	cancel()
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(reqCtx)
	c.Set("username", "test_user")
	c.Set("token_name", "fixture")
	c.Set(common.RequestIdKey, "interrupted-fixture-request")
	c.Set(common.UpstreamRequestIdKey, "interrupted-fixture-upstream")
	info := &relaycommon.RelayInfo{
		UserId: userID, UserQuota: balance, TokenId: tokenID, TokenKey: "interrupted-fixture-token",
		OriginModelName: "claude-fixture", UsingGroup: "default", BillingSource: source, StartTime: time.Now(),
		IsStream: true, RelayFormat: types.RelayFormatClaude, FinalRequestRelayFormat: types.RelayFormatClaude,
		ChannelMeta:  &relaycommon.ChannelMeta{ChannelId: channelID, UpstreamModelName: "claude-fixture"},
		StreamStatus: relaycommon.NewStreamStatus(),
		PriceData:    types.PriceData{ModelRatio: 5, CompletionRatio: 5, CacheRatio: 0.025, CacheCreationRatio: 1.25, CacheCreation5mRatio: 1.25, CacheCreation1hRatio: 2, GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 0.85}},
	}
	info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonClientGone, context.Canceled)
	var funding FundingSource = &WalletFunding{userId: userID, consumed: pre}
	if source == BillingSourceSubscription {
		seedSubscription(t, subscriptionID, userID, balance, int64(pre))
		info.SubscriptionId = subscriptionID
		funding = &SubscriptionFunding{userId: userID, subscriptionId: subscriptionID, preConsumed: int64(pre)}
	}
	session := &BillingSession{relayInfo: info, funding: funding, preConsumedQuota: pre, tokenConsumed: pre, trusted: pre == 0}
	info.Billing = session
	return c, info, session
}

func TestInterruptedTextBillingSettlesOnceAndSurvivesRefund(t *testing.T) {
	for _, source := range []string{BillingSourceWallet, BillingSourceSubscription} {
		for _, pre := range []int{0, 50000, 2000000} {
			t.Run(source+"_"+strconv.Itoa(pre), func(t *testing.T) {
				c, info, session := interruptedBillingFixture(t, source, pre)
				usage := &dto.Usage{PromptTokens: 366640, CompletionTokens: 2, UsageSemantic: "anthropic"}
				const want = 1558263
				require.NoError(t, PostInterruptedTextConsumeQuota(c, info, usage))
				require.NoError(t, PostInterruptedTextConsumeQuota(c, info, usage))
				session.Refund(c)
				require.True(t, session.settled)
				require.False(t, session.refunded)
				require.False(t, session.NeedsRefund())
				require.Equal(t, 100000000-want, getTokenRemainQuota(t, info.TokenId))
				if source == BillingSourceWallet {
					require.Equal(t, 100000000-want, getUserQuota(t, info.UserId))
				} else {
					require.Equal(t, 100000000, getUserQuota(t, info.UserId))
					var sub model.UserSubscription
					require.NoError(t, model.DB.First(&sub, info.SubscriptionId).Error)
					require.Equal(t, int64(want), sub.AmountUsed)
				}
				var logs []model.Log
				require.NoError(t, model.LOG_DB.Where("request_id = ?", "interrupted-fixture-request").Find(&logs).Error)
				require.Len(t, logs, 1)
				require.Equal(t, want, logs[0].Quota)
				require.Equal(t, 366640, logs[0].PromptTokens)
				require.Equal(t, 2, logs[0].CompletionTokens)
				require.Equal(t, "interrupted-fixture-upstream", logs[0].UpstreamRequestId)
				var other map[string]any
				require.NoError(t, common.UnmarshalJsonStr(logs[0].Other, &other))
				require.Equal(t, "stream_interrupted", other["billing_reason"])
				require.Equal(t, true, other["usage_partial"])
				var u model.User
				require.NoError(t, model.DB.First(&u, info.UserId).Error)
				require.Equal(t, want, u.UsedQuota)
				require.Equal(t, 1, u.RequestCount)
				t.Logf("source=%s pre=%d actual=%d token_remaining=%d consume_records=%d refund_after_settle=%v", source, pre, want, getTokenRemainQuota(t, info.TokenId), len(logs), session.refunded)
			})
		}
	}
}

func TestInterruptedTextBillingWithoutUsageRemainsRefundable(t *testing.T) {
	c, info, session := interruptedBillingFixture(t, BillingSourceWallet, 50000)
	require.NoError(t, PostInterruptedTextConsumeQuota(c, info, &dto.Usage{}))
	require.False(t, session.settled)
	session.Refund(c)
	require.Eventually(t, func() bool {
		return getUserQuota(t, info.UserId) == 100000000 && getTokenRemainQuota(t, info.TokenId) == 100000000
	}, 3*time.Second, 10*time.Millisecond)
	var n int64
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Where("request_id = ?", "interrupted-fixture-request").Count(&n).Error)
	require.Zero(t, n)
}

type failedInterruptedFunding struct{}

func (*failedInterruptedFunding) Source() string       { return BillingSourceWallet }
func (*failedInterruptedFunding) PreConsume(int) error { return nil }
func (*failedInterruptedFunding) Settle(int) error     { return errors.New("fixture funding unavailable") }
func (*failedInterruptedFunding) Refund() error        { return nil }

func TestInterruptedTextBillingSettlementFailureDoesNotRecordSuccess(t *testing.T) {
	c, info, session := interruptedBillingFixture(t, BillingSourceWallet, 0)
	session.funding = &failedInterruptedFunding{}
	require.ErrorContains(t, PostInterruptedTextConsumeQuota(c, info, &dto.Usage{PromptTokens: 100, UsageSemantic: "anthropic"}), "fixture funding unavailable")
	require.False(t, session.settled)
	require.False(t, c.GetBool("claude_interrupted_usage_settled"))
	require.Equal(t, 100000000, getUserQuota(t, info.UserId))
	require.Equal(t, 100000000, getTokenRemainQuota(t, info.TokenId))
	var n int64
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Where("request_id = ?", "interrupted-fixture-request").Count(&n).Error)
	require.Zero(t, n)
}

func TestInterruptedTextBillingCacheOnly(t *testing.T) {
	for _, tc := range []struct {
		name  string
		usage dto.Usage
		want  int
	}{
		{"read", dto.Usage{UsageSemantic: "anthropic", PromptTokensDetails: dto.InputTokenDetails{CachedTokens: 1000}}, 106},
		{"write_split", dto.Usage{UsageSemantic: "anthropic", PromptTokensDetails: dto.InputTokenDetails{CachedCreationTokens: 30}, ClaudeCacheCreation5mTokens: 10, ClaudeCacheCreation1hTokens: 20}, 223},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, info, _ := interruptedBillingFixture(t, BillingSourceWallet, 0)
			require.True(t, HasBillableClaudeUsage(&tc.usage))
			require.NoError(t, PostInterruptedTextConsumeQuota(c, info, &tc.usage))
			require.Equal(t, 100000000-tc.want, getTokenRemainQuota(t, info.TokenId))
		})
	}
}
