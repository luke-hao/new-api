package claude

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestClaudeStreamPassthroughHandlerPreservesRawSSE(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	raw := strings.Join([]string{
		"event: message_start\r\n",
		`data: {"type":"message_start","message":{"type":"message","model":"claude-opus-4-8","usage":{"input_tokens":3,"output_tokens":0},"role":"assistant","id":"msg_test","content":[]}}` + "\r\n\r\n",
		": upstream-extension keep-this\r\n\r\n",
		"event: message_delta\r\n",
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}` + "\r\n\r\n",
		"event: message_stop\r\n",
		`data: {"type":"message_stop"}` + "\r\n\r\n",
	}, "")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(raw)),
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "claude-opus-4-8",
		},
		RelayFormat: types.RelayFormatClaude,
		IsStream:    true,
	}

	usage, apiErr := ClaudeStreamPassthroughHandler(ctx, resp, info)

	require.Nil(t, apiErr)
	require.Equal(t, raw, recorder.Body.String())
	require.Equal(t, 3, usage.PromptTokens)
	require.Equal(t, 2, usage.CompletionTokens)
	require.Equal(t, "claude-opus-4-8", info.UpstreamModelName)
}
