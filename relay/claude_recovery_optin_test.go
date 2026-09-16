package relay

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestClaudeThinkingRecoveryRequiresChannelOptIn(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldRedis, oldLog := common.RedisEnabled, common.LogConsumeEnabled
	global := model_setting.GetGlobalSettings()
	oldPass := global.PassThroughRequestEnabled
	common.RedisEnabled, common.LogConsumeEnabled, global.PassThroughRequestEnabled = false, false, false
	t.Cleanup(func() {
		common.RedisEnabled, common.LogConsumeEnabled, global.PassThroughRequestEnabled = oldRedis, oldLog, oldPass
	})
	service.InitHttpClient()
	for _, tt := range []struct {
		name, setting                       string
		warmChannel, wantCalls              int
		alwaysBad, wantError, wantPreflight bool
	}{
		{name: "missing setting preserves errors", setting: `{}`, wantCalls: 1, wantError: true},
		{name: "disabled ignores same channel cache", setting: `{"claude_thinking_recovery_enabled":false}`, warmChannel: 901, wantCalls: 1, wantError: true},
		{name: "enabled recovers once", setting: `{"claude_thinking_recovery_enabled":true}`, wantCalls: 2},
		{name: "enabled precleans own history", setting: `{"claude_thinking_recovery_enabled":true}`, warmChannel: 901, wantCalls: 1, wantPreflight: true},
		{name: "enabled ignores another channel history", setting: `{"claude_thinking_recovery_enabled":true}`, warmChannel: 902, wantCalls: 2},
		{name: "failed retry stops after two attempts", setting: `{"claude_thinking_recovery_enabled":true}`, alwaysBad: true, wantCalls: 2, wantError: true},
	} {
		for _, passthrough := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/passthrough=%v", tt.name, passthrough), func(t *testing.T) {
				signature := "fixture-" + strings.ReplaceAll(t.Name(), "/", "-")
				body := []byte(fmt.Sprintf(`{"model":"claude-test","max_tokens":128,"messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"history","signature":%q},{"type":"text","text":"keep me"}]},{"role":"user","content":"continue"}],"extra_fixture":false}`, signature))
				if tt.warmChannel > 0 {
					scan, err := service.SanitizeAllClaudeThinking(body)
					require.NoError(t, err)
					service.RememberInvalidClaudeThinking(scan.Fingerprints, tt.warmChannel)
				}
				var sent [][]byte
				var mu sync.Mutex
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					data, err := io.ReadAll(r.Body)
					require.NoError(t, err)
					mu.Lock()
					sent = append(sent, data)
					mu.Unlock()
					w.Header().Set("Content-Type", "application/json")
					if tt.alwaysBad || bytes.Contains(data, []byte(signature)) {
						w.WriteHeader(400)
						_, _ = io.WriteString(w, `{"type":"error","error":{"type":"invalid_request_error","message":"Invalid signature in thinking block"}}`)
						return
					}
					_, _ = io.WriteString(w, `{"id":"msg-ok","type":"message","role":"assistant","model":"claude-test","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{"input_tokens":0,"output_tokens":0}}`)
				}))
				defer upstream.Close()
				var request dto.ClaudeRequest
				require.NoError(t, common.Unmarshal(body, &request))
				var setting dto.ChannelSettings
				require.NoError(t, common.UnmarshalJsonStr(tt.setting, &setting))
				setting.PassThroughBodyEnabled = passthrough
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest("POST", "/v1/messages", bytes.NewReader(body))
				c.Request.Header.Set("Content-Type", "application/json")
				common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeAnthropic)
				common.SetContextKey(c, constant.ContextKeyChannelId, 901)
				common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstream.URL)
				common.SetContextKey(c, constant.ContextKeyChannelKey, "fixture-key")
				common.SetContextKey(c, constant.ContextKeyOriginalModel, request.Model)
				common.SetContextKey(c, constant.ContextKeyChannelSetting, setting)
				common.SetContextKey(c, constant.ContextKeyChannelOtherSetting, dto.ChannelOtherSettings{})
				defer common.CleanupBodyStorage(c)
				info := &relaycommon.RelayInfo{Request: &request, OriginModelName: request.Model, RelayFormat: types.RelayFormatClaude, StartTime: time.Now()}
				err := ClaudeHelper(c, info)
				if tt.wantError {
					require.NotNil(t, err)
					require.Equal(t, 400, err.StatusCode)
				} else {
					require.Nil(t, err)
				}
				mu.Lock()
				defer mu.Unlock()
				require.Len(t, sent, tt.wantCalls)
				require.Equal(t, !tt.wantPreflight, bytes.Contains(sent[0], []byte(signature)))
				if passthrough && !tt.wantPreflight {
					require.Equal(t, body, sent[0])
				}
				for _, data := range sent {
					require.Contains(t, string(data), "keep me")
				}
				if len(sent) == 2 {
					require.NotContains(t, string(sent[1]), signature)
				}
				admin := map[string]interface{}{}
				service.AppendClaudeThinkingRecoveryAdminInfo(c, admin)
				if !setting.ClaudeThinkingRecoveryEnabled {
					require.Empty(t, admin)
				} else {
					require.Contains(t, admin, "thinking_signature_recovery")
				}
			})
		}
	}
}
