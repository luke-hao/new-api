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

func TestEnsureClaudeAdaptiveThinkingDisplay(t *testing.T) {
	tests := []struct {
		name        string
		model       string
		thinking    *dto.Thinking
		wantDisplay string
	}{
		{
			name:        "opus 4.8 defaults to summarized",
			model:       "claude-opus-4-8",
			thinking:    &dto.Thinking{Type: "adaptive"},
			wantDisplay: "summarized",
		},
		{
			name:        "opus 4.7 defaults to summarized",
			model:       "claude-opus-4-7",
			thinking:    &dto.Thinking{Type: "adaptive"},
			wantDisplay: "summarized",
		},
		{
			name:        "explicit omitted is preserved",
			model:       "claude-opus-4-8",
			thinking:    &dto.Thinking{Type: "adaptive", Display: "omitted"},
			wantDisplay: "omitted",
		},
		{
			name:        "enabled thinking is unchanged",
			model:       "claude-opus-4-8",
			thinking:    &dto.Thinking{Type: "enabled"},
			wantDisplay: "",
		},
		{
			name:        "opus 4.6 adaptive default is unchanged",
			model:       "claude-opus-4-6",
			thinking:    &dto.Thinking{Type: "adaptive"},
			wantDisplay: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &dto.ClaudeRequest{Model: tt.model, Thinking: tt.thinking}
			ensureClaudeAdaptiveThinkingDisplay(request)
			if request.Thinking.Display != tt.wantDisplay {
				t.Fatalf("display = %q, want %q", request.Thinking.Display, tt.wantDisplay)
			}
		})
	}
}

func TestShouldUseRawClaudeBody(t *testing.T) {
	tests := []struct {
		name               string
		passThroughEnabled bool
		upstreamAPIType    int
		want               bool
	}{
		{
			name:               "disabled never passes through",
			passThroughEnabled: false,
			upstreamAPIType:    constant.APITypeAnthropic,
			want:               false,
		},
		{
			name:               "anthropic upstream keeps native body",
			passThroughEnabled: true,
			upstreamAPIType:    constant.APITypeAnthropic,
			want:               true,
		},
		{
			name:               "openai upstream requires conversion",
			passThroughEnabled: true,
			upstreamAPIType:    constant.APITypeOpenAI,
			want:               false,
		},
		{
			name:               "codex upstream requires conversion",
			passThroughEnabled: true,
			upstreamAPIType:    constant.APITypeCodex,
			want:               false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldUseRawClaudeBody(tt.passThroughEnabled, tt.upstreamAPIType); got != tt.want {
				t.Fatalf("shouldUseRawClaudeBody(%v, %d) = %v, want %v", tt.passThroughEnabled, tt.upstreamAPIType, got, tt.want)
			}
		})
	}
}

func TestClaudeUsageDiagnosticChannelEnabled(t *testing.T) {
	require.True(t, claudeUsageDiagnosticChannelEnabled("148", 148))
	require.True(t, claudeUsageDiagnosticChannelEnabled(" 147, 148,149 ", 148))
	require.True(t, claudeUsageDiagnosticChannelEnabled("*", 148))
	require.False(t, claudeUsageDiagnosticChannelEnabled("", 148))
	require.False(t, claudeUsageDiagnosticChannelEnabled("14,8148", 148))
}

func TestBuildClaudeUsageDiagnostic(t *testing.T) {
	body := []byte(`{"model":"claude-opus-4-6","system":"system","messages":[{"role":"user","content":"hello"}],"tools":[{"name":"lookup","description":"test","input_schema":{"type":"object"}}]}`)
	var request dto.ClaudeRequest
	require.NoError(t, common.Unmarshal(body, &request))

	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 148}}
	info.SetEstimatePromptTokens(190000)
	usage := &dto.Usage{
		PromptTokens: 2,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens:         273256,
			CachedCreationTokens: 46772,
		},
	}

	diagnostic := buildClaudeUsageDiagnostic(info, &request, usage, body)
	require.Contains(t, diagnostic, "claude_usage_diagnostic channel_id=148")
	require.Contains(t, diagnostic, fmt.Sprintf("body_bytes=%d", len(body)))
	require.Contains(t, diagnostic, "body_sha256=")
	require.Contains(t, diagnostic, "prompt_sha256=")
	require.Contains(t, diagnostic, "local_estimate=190000")
	require.Contains(t, diagnostic, "messages=1 tools=1")
	require.Contains(t, diagnostic, "system_type=string system_blocks=1")
	require.Contains(t, diagnostic, "upstream_input=320030 input=2 cache_read=273256 cache_creation=46772")
}

func TestClaudeHelperRecoversInvalidThinkingSignatureOnSameChannel(t *testing.T) {
	signature := "integration-" + strings.ReplaceAll(t.Name(), "/", "-")
	requestBody := []byte(fmt.Sprintf(`{
		"model":"claude-test",
		"max_tokens":128,
		"thinking":{"type":"enabled","budget_tokens":1024},
		"messages":[
			{"role":"user","content":[{"type":"text","text":"start"}]},
			{"role":"assistant","content":[
				{"type":"thinking","thinking":"hidden","signature":%q},
				{"type":"redacted_thinking","data":%q},
				{"type":"text","text":"visible"},
				{"type":"tool_use","id":"tool-1","name":"lookup","input":{"q":"x"}}
			]},
			{"role":"user","content":[
				{"type":"tool_result","tool_use_id":"tool-1","content":"ok"},
				{"type":"text","text":"continue"}
			]}
		]
	}`, signature, "opaque-"+signature))

	type upstreamAttempt struct {
		path   string
		apiKey string
		body   []byte
	}
	var (
		attemptsMu sync.Mutex
		attempts   []upstreamAttempt
	)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		attemptsMu.Lock()
		attempts = append(attempts, upstreamAttempt{
			path:   r.URL.Path,
			apiKey: r.Header.Get("x-api-key"),
			body:   append([]byte(nil), body...),
		})
		attempt := len(attempts)
		attemptsMu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set(common.RequestIdKey, fmt.Sprintf("upstream-%d", attempt))
		if attempt == 1 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"type":"error","error":{"type":"invalid_request_error","message":"thinking: Invalid signature in thinking block (request id: deep) (request id: proxy)"}}`)
			return
		}
		_, _ = io.WriteString(w, `{"id":"msg-recovered","type":"message","role":"assistant","model":"claude-test","content":[{"type":"thinking","thinking":"fresh","signature":"fresh-signature"},{"type":"text","text":"recovered"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":0,"output_tokens":0}}`)
	}))
	defer upstream.Close()

	var request dto.ClaudeRequest
	require.NoError(t, common.Unmarshal(requestBody, &request))

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(requestBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(common.RequestIdKey, "local-request")
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeAnthropic)
	common.SetContextKey(c, constant.ContextKeyChannelId, 777)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstream.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "same-channel-key")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, request.Model)
	common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{})
	common.SetContextKey(c, constant.ContextKeyChannelOtherSetting, dto.ChannelOtherSettings{})
	defer common.CleanupBodyStorage(c)

	globalSettings := model_setting.GetGlobalSettings()
	oldPassThrough := globalSettings.PassThroughRequestEnabled
	oldLogConsume := common.LogConsumeEnabled
	globalSettings.PassThroughRequestEnabled = false
	common.LogConsumeEnabled = false
	t.Cleanup(func() {
		globalSettings.PassThroughRequestEnabled = oldPassThrough
		common.LogConsumeEnabled = oldLogConsume
	})

	service.InitHttpClient()
	info := &relaycommon.RelayInfo{
		Request:         &request,
		OriginModelName: request.Model,
		RelayFormat:     types.RelayFormatClaude,
		StartTime:       time.Now(),
	}
	relayErr := ClaudeHelper(c, info)
	require.Nil(t, relayErr)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"id":"msg-recovered"`)
	require.Contains(t, recorder.Body.String(), `"text":"recovered"`)
	require.NotContains(t, recorder.Body.String(), "Invalid signature")
	require.Equal(t, "upstream-2", c.GetString(common.UpstreamRequestIdKey))

	attemptsMu.Lock()
	recordedAttempts := append([]upstreamAttempt(nil), attempts...)
	attemptsMu.Unlock()
	require.Len(t, recordedAttempts, 2)
	for _, attempt := range recordedAttempts {
		require.Equal(t, "/v1/messages", attempt.path)
		require.Equal(t, "same-channel-key", attempt.apiKey)
	}
	require.Contains(t, string(recordedAttempts[0].body), `"type":"thinking"`)
	require.Contains(t, string(recordedAttempts[0].body), `"type":"redacted_thinking"`)
	require.NotContains(t, string(recordedAttempts[1].body), `"type":"thinking"`)
	require.NotContains(t, string(recordedAttempts[1].body), `"type":"redacted_thinking"`)
	require.Contains(t, string(recordedAttempts[1].body), `"type":"tool_use"`)
	require.Contains(t, string(recordedAttempts[1].body), `"type":"tool_result"`)
	require.Contains(t, string(recordedAttempts[1].body), `"thinking":{"type":"enabled"`)

	preflight, err := service.SanitizeKnownInvalidClaudeThinking(requestBody)
	require.NoError(t, err)
	require.Equal(t, 2, preflight.RemovedBlocks)
	require.NotContains(t, string(preflight.Body), signature)

	adminInfo := map[string]interface{}{}
	service.AppendClaudeThinkingRecoveryAdminInfo(c, adminInfo)
	recovery, ok := adminInfo["thinking_signature_recovery"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, true, recovery["attempted"])
	require.Equal(t, 2, recovery["removed_blocks"])
	require.Equal(t, true, recovery["success"])
}
