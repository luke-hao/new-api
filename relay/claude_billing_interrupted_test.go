package relay

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type interruptedRelayAdaptor struct {
	channel.Adaptor
	response *http.Response
	calls    int
}

func (a *interruptedRelayAdaptor) DoRequest(_ *gin.Context, _ *relaycommon.RelayInfo, _ io.Reader) (any, error) {
	a.calls++
	return a.response, nil
}
func (a *interruptedRelayAdaptor) DoResponse(c *gin.Context, r *http.Response, i *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	return claude.ClaudeStreamPassthroughHandler(c, r, i)
}

type interruptedRelayBilling struct{ calls, actual int }

func (b *interruptedRelayBilling) Settle(q int) error       { b.calls++; b.actual = q; return nil }
func (b *interruptedRelayBilling) Refund(*gin.Context)      {}
func (b *interruptedRelayBilling) NeedsRefund() bool        { return false }
func (b *interruptedRelayBilling) GetPreConsumedQuota() int { return 0 }
func (b *interruptedRelayBilling) Reserve(int) error        { return nil }

type interruptedRelayReader struct {
	io.Reader
	cancel context.CancelFunc
}

func (r *interruptedRelayReader) Read(p []byte) (int, error) {
	n, e := r.Reader.Read(p)
	if n > 0 {
		r.cancel()
	}
	return n, e
}

func TestInterruptedClaudeAttemptPreservesUsageThroughSettlement(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldDB, oldLog := model.DB, model.LOG_DB
	oldSQLite, oldRedis, oldBatch, oldConsume := common.UsingSQLite, common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	conn, err := db.DB()
	require.NoError(t, err)
	conn.SetMaxOpenConns(1)
	model.DB = db
	model.LOG_DB = db
	common.UsingSQLite = true
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	common.LogConsumeEnabled = true
	t.Cleanup(func() {
		model.DB = oldDB
		model.LOG_DB = oldLog
		common.UsingSQLite = oldSQLite
		common.RedisEnabled = oldRedis
		common.BatchUpdateEnabled = oldBatch
		common.LogConsumeEnabled = oldConsume
		_ = conn.Close()
	})
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Channel{}, &model.Log{}))
	require.NoError(t, db.Create(&model.User{Id: 92001, Username: "relay_fixture", Quota: 100000000}).Error)
	require.NoError(t, db.Create(&model.Channel{Id: 92002, Name: "fixture", Key: "fixture"}).Error)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(ctx)
	c.Set("username", "relay_fixture")
	c.Set(common.RequestIdKey, "relay-interrupted-fixture")
	info := &relaycommon.RelayInfo{UserId: 92001, UserQuota: 100000000, IsPlayground: true, StartTime: time.Now(), OriginModelName: "claude-fixture", IsStream: true, RelayFormat: types.RelayFormatClaude, FinalRequestRelayFormat: types.RelayFormatClaude, ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 92002}, PriceData: types.PriceData{ModelRatio: 5, CompletionRatio: 5, GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 0.85}}}
	billing := &interruptedRelayBilling{}
	info.Billing = billing
	raw := "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"model\":\"claude-fixture\",\"usage\":{\"input_tokens\":366640,\"output_tokens\":2}}}\n\n"
	adaptor := &interruptedRelayAdaptor{response: &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(&interruptedRelayReader{Reader: strings.NewReader(raw), cancel: cancel})}}
	usage, apiErr := executeClaudeAttempt(c, info, adaptor, &dto.ClaudeRequest{}, true, []byte(`{}`))
	require.NotNil(t, apiErr)
	require.NotNil(t, usage)
	require.Equal(t, 366640, usage.PromptTokens)
	result := finishClaudeAttempt(c, info, usage, apiErr, "")
	require.Same(t, apiErr, result)
	require.True(t, errors.Is(result, context.Canceled))
	require.True(t, types.IsSkipRetryError(result))
	require.Equal(t, 1, adaptor.calls)
	require.Equal(t, 1, billing.calls)
	require.Equal(t, 1558263, billing.actual)
	require.Same(t, apiErr, finishClaudeAttempt(c, info, usage, apiErr, ""))
	require.Equal(t, 1, billing.calls)
	var logs []model.Log
	require.NoError(t, db.Where("request_id = ?", "relay-interrupted-fixture").Find(&logs).Error)
	require.Len(t, logs, 1)
	require.Equal(t, 1558263, logs[0].Quota)
	t.Logf("upstream_calls=%d settlements=%d quota=%d consume_logs=%d skip_retry=%v", adaptor.calls, billing.calls, billing.actual, len(logs), types.IsSkipRetryError(result))
}
