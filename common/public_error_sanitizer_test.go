package common

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSanitizePublicErrorText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
		mask bool
	}{
		{
			name: "english distributor group",
			in:   "status_code=503, No available channel for model claude-fable-5 under group F4-AWS-Kiro-2 (distributor) (request id: upstream-id)",
			want: "status_code=503, " + PublicPoolChannelUnavailableMessage,
			mask: true,
		},
		{
			name: "plural upstreams",
			in:   "status_code=503, All upstreams are temporarily unavailable, please retry later.",
			want: "status_code=503, " + PublicPoolChannelUnavailableMessage,
			mask: true,
		},
		{
			name: "upstream request id snake case",
			in:   "status_code=503, request failed, upstream_request_id=req-123",
			want: "status_code=503, " + PublicPoolChannelUnavailableMessage,
			mask: true,
		},
		{
			name: "upstream request id camel case",
			in:   "status_code=503, request failed, upstreamRequestId=req-456",
			want: "status_code=503, " + PublicPoolChannelUnavailableMessage,
			mask: true,
		},
		{
			name: "simplified chinese group",
			in:   "获取分组 F4-AWS-Kiro-2 下模型 claude-fable-5 的可用渠道失败（distributor）",
			want: PublicPoolChannelUnavailableMessage,
			mask: true,
		},
		{
			name: "traditional chinese group",
			in:   "分組 F4-AWS-Kiro-2 下模型 claude-fable-5 無可用管道（distributor）",
			want: PublicPoolChannelUnavailableMessage,
			mask: true,
		},
		{name: "ordinary rate limit", in: "rate limit reached for requests per minute", want: "rate limit reached for requests per minute"},
		{name: "content moderation", in: "content moderation failed", want: "content moderation failed"},
		{name: "local user quota", in: "用户额度不足, 剩余额度: $0.00", want: "用户额度不足, 剩余额度: $0.00"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, masked := SanitizePublicErrorText(tt.in)
			require.Equal(t, tt.mask, masked)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestSanitizePublicErrorPayload(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		masked      bool
		assertValue func(t *testing.T, value map[string]any)
	}{
		{
			name:   "openai upstream error",
			input:  `{"error":{"message":"No available channel for model m under group secret (distributor)","type":"upstream_error","code":"server_error","metadata":{"group":"secret"},"upstream_request_id":"up-1"}}`,
			masked: true,
			assertValue: func(t *testing.T, value map[string]any) {
				err := value["error"].(map[string]any)
				require.Equal(t, PublicPoolChannelUnavailableMessage, err["message"])
				require.Equal(t, "new_api_error", err["type"])
				require.Equal(t, PublicPoolChannelUnavailableCode, err["code"])
				require.NotContains(t, err, "metadata")
				require.NotContains(t, err, "upstream_request_id")
			},
		},
		{
			name:   "realtime upstream event",
			input:  `{"type":"upstream_error","error":{"message":"relay server unavailable","type":"upstream_error","code":"relay_error"}}`,
			masked: true,
			assertValue: func(t *testing.T, value map[string]any) {
				require.Equal(t, "error", value["type"])
				err := value["error"].(map[string]any)
				require.Equal(t, PublicPoolChannelUnavailableMessage, err["message"])
			},
		},
		{
			name:   "responses failed event",
			input:  `{"type":"response.failed","response":{"status":"failed","error":{"message":"upstream channel #12 failed","code":"server_error"}}}`,
			masked: true,
			assertValue: func(t *testing.T, value map[string]any) {
				require.Equal(t, "response.failed", value["type"])
				response := value["response"].(map[string]any)
				err := response["error"].(map[string]any)
				require.Equal(t, PublicPoolChannelUnavailableCode, err["code"])
			},
		},
		{
			name:   "safe moderation error",
			input:  `{"error":{"message":"content moderation failed","type":"server_error","code":"content_moderation_failed"}}`,
			masked: false,
		},
		{
			name:   "normal chat chunk mentioning group",
			input:  `{"object":"chat.completion.chunk","choices":[{"delta":{"content":"explain the upstream group architecture"}}]}`,
			masked: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, masked := SanitizePublicErrorPayload([]byte(tt.input))
			require.Equal(t, tt.masked, masked)
			if !tt.masked {
				require.Equal(t, tt.input, string(got))
				return
			}
			var value map[string]any
			require.NoError(t, json.Unmarshal(got, &value))
			tt.assertValue(t, value)
		})
	}
}
