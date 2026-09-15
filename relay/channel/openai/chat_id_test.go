package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func chatIDContext(body string, stream bool) (*gin.Context, *httptest.ResponseRecorder, *http.Response, *relaycommon.RelayInfo) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	contentType := "application/json"
	if stream {
		contentType = "text/event-stream"
	}
	resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {contentType}}, Body: io.NopCloser(strings.NewReader(body))}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenAI, UpstreamModelName: "gpt-4o"},
		RelayFormat: types.RelayFormatOpenAI, RelayMode: relayconstant.RelayModeChatCompletions,
		IsStream: stream, ShouldIncludeUsage: true, DisablePing: true, StartTime: time.Now(),
	}
	return c, recorder, resp, info
}

func TestChatIDNonStreamCompatibility(t *testing.T) {
	const body = `{"id":"resp_example","object":"chat.completion","created":1,"model":"gpt-4o","choices":[{"index":0,"message":{"role":"assistant","content":"resp_example","tool_calls":[{"id":"call_example","type":"function","function":{"name":"lookup","arguments":"{}"}}]},"finish_reason":"stop"}],"usage":{"prompt_tokens":11,"completion_tokens":5,"total_tokens":16,"prompt_tokens_details":{"cached_tokens":4}},"vendor_extra":{"large":9007199254740993,"id":"resp_nested"}}`
	for _, force := range []bool{false, true} {
		t.Run(map[bool]string{false: "passthrough", true: "force_format"}[force], func(t *testing.T) {
			c, recorder, resp, info := chatIDContext(body, false)
			info.ChannelSetting.ForceFormat = force
			usage, err := OpenaiHandler(c, info, resp)
			require.Nil(t, err)
			require.Equal(t, 11, usage.PromptTokens)
			require.Equal(t, 5, usage.CompletionTokens)
			out := recorder.Body.String()
			require.Equal(t, "chatcmpl-resp_example", gjson.Get(out, "id").String())
			require.Equal(t, "call_example", gjson.Get(out, "choices.0.message.tool_calls.0.id").String())
			require.Equal(t, "resp_example", gjson.Get(out, "choices.0.message.content").String())
			if !force {
				require.Equal(t, strings.Replace(body, `"id":"resp_example"`, `"id":"chatcmpl-resp_example"`, 1), out)
			}
		})
	}
}

func TestChatIDUnrelatedResponsesStayUnchanged(t *testing.T) {
	const body = `{"id":"ID_VALUE","object":"OBJECT_VALUE","created":1,"model":"gpt-4o","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":11,"completion_tokens":5,"total_tokens":16}}`
	for _, tc := range []struct {
		name, id, object string
		mode             int
	}{
		{"native_chat", "chatcmpl_native", "chat.completion", relayconstant.RelayModeChatCompletions},
		{"valid_chat", "chatcmpl-native", "chat.completion", relayconstant.RelayModeChatCompletions},
		{"already_mapped", "chatcmpl-resp_example", "chat.completion", relayconstant.RelayModeChatCompletions},
		{"responses_object", "resp_example", "response", relayconstant.RelayModeChatCompletions},
		{"legacy_completion", "resp_example", "text_completion", relayconstant.RelayModeCompletions},
		{"other_mode", "resp_example", "chat.completion", relayconstant.RelayModeCompletions},
		{"empty_suffix", "resp_", "chat.completion", relayconstant.RelayModeChatCompletions},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := strings.NewReplacer("ID_VALUE", tc.id, "OBJECT_VALUE", tc.object).Replace(body)
			c, recorder, resp, info := chatIDContext(input, false)
			info.RelayMode = tc.mode
			_, err := OpenaiHandler(c, info, resp)
			require.Nil(t, err)
			require.Equal(t, input, recorder.Body.String())
		})
	}
}

func TestChatIDStreamCompatibility(t *testing.T) {
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })
	for _, tc := range []struct {
		name                   string
		usage, force, thinking bool
	}{
		{"upstream_usage", true, false, false},
		{"generated_usage", false, false, false},
		{"force_format", true, true, false},
		{"thinking_conversion", true, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			events := []string{
				`{"id":"resp_stream","object":"chat.completion.chunk","created":1,"model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":"ok"}}],"vendor_extra":9007199254740993}`,
				`{"id":"resp_stream","object":"chat.completion.chunk","created":1,"model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
			}
			if tc.usage {
				events = append(events, `{"id":"resp_stream","object":"chat.completion.chunk","created":1,"model":"gpt-4o","choices":[],"usage":{"prompt_tokens":11,"completion_tokens":5,"total_tokens":16}}`)
			}
			var wire strings.Builder
			for _, event := range events {
				wire.WriteString("data: " + event + "\n\n")
			}
			wire.WriteString("data: [DONE]\n\n")
			c, recorder, resp, info := chatIDContext(wire.String(), true)
			info.ChannelSetting.ForceFormat = tc.force
			info.ChannelSetting.ThinkingToContent = tc.thinking
			usage, err := OaiStreamHandler(c, info, resp)
			require.Nil(t, err)
			require.Contains(t, recorder.Body.String(), "data: [DONE]")
			count := 0
			for _, line := range strings.Split(recorder.Body.String(), "\n") {
				if !strings.HasPrefix(line, "data: {") {
					continue
				}
				payload := strings.TrimPrefix(line, "data: ")
				var parsed dto.ChatCompletionsStreamResponse
				require.NoError(t, common.UnmarshalJsonStr(payload, &parsed))
				require.Equal(t, "chatcmpl-resp_stream", parsed.Id)
				count++
			}
			require.Equal(t, 3, count)
			if tc.usage {
				require.Equal(t, 11, usage.PromptTokens)
				require.Equal(t, 5, usage.CompletionTokens)
			}
			if !tc.force && !tc.thinking {
				require.Contains(t, recorder.Body.String(), `"vendor_extra":9007199254740993`)
			}
		})
	}
}
