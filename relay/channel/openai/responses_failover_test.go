package openai

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const prefaceFixture = "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_failed\",\"model\":\"test-model\"}}\n\n"
const completedFixture = "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_winner\",\"usage\":{\"input_tokens\":12,\"output_tokens\":3,\"total_tokens\":15}}}\n\n"

type failureReader struct{ io.Reader }

func (r failureReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if err == io.EOF {
		return n, errors.New("HTTP/2 INTERNAL_ERROR")
	}
	return n, err
}
func responsesFailoverFixture(t *testing.T) (*gin.Context, *httptest.ResponseRecorder, *relaycommon.RelayInfo) {
	t.Helper()
	old := constant.StreamingTimeout
	constant.StreamingTimeout = 2
	t.Cleanup(func() { constant.StreamingTimeout = old })
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	info := &relaycommon.RelayInfo{IsStream: true, DisablePing: true, StartTime: time.Now(), ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "test-model"}}
	return c, recorder, info
}
func TestResponsesFailoverFailureBoundaries(t *testing.T) {
	cases := []struct {
		name, body               string
		readError, skip, visible bool
		input, output            int
	}{
		{"empty_eof", "", false, false, false, 0, 0},
		{"metadata_eof", prefaceFixture, false, false, false, 0, 0},
		{"http2_error", prefaceFixture, true, false, false, 0, 0},
		{"failed_event", prefaceFixture + "data: {\"type\":\"response.failed\",\"response\":{\"error\":{\"message\":\"unavailable\"}}}\n\n", false, false, false, 0, 0},
		{"top_level_error", prefaceFixture + "data: {\"error\":{\"message\":\"unavailable\"}}\n\n", false, false, false, 0, 0},
		{"malformed", prefaceFixture + "data: {broken\n\n", false, false, false, 0, 0},
		{"text_already_sent", prefaceFixture + "data: {\"type\":\"response.output_text.delta\",\"delta\":\"answer\"}\n\n", false, true, true, 0, 0},
		{"tool_already_sent", prefaceFixture + "data: {\"type\":\"response.output_item.added\",\"item\":{\"type\":\"function_call\",\"name\":\"send_email\"}}\n\n", false, true, true, 0, 0},
		{"confirmed_usage", prefaceFixture + "data: {\"type\":\"response.failed\",\"response\":{\"usage\":{\"input_tokens\":12,\"output_tokens\":3,\"total_tokens\":15},\"error\":{\"message\":\"failed\"}}}\n\n", false, true, false, 12, 3},
		{"preface_limit", strings.Repeat(prefaceFixture, 65), false, true, true, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, w, info := responsesFailoverFixture(t)
			var reader io.Reader = strings.NewReader(tc.body)
			if tc.readError {
				reader = failureReader{reader}
			}
			usage, err := OaiResponsesStreamHandler(c, info, &http.Response{StatusCode: 200, Body: io.NopCloser(reader)})
			require.NotNil(t, err)
			require.Equal(t, types.ErrorCodeIncompleteStream, err.GetErrorCode())
			require.Equal(t, tc.skip, types.IsSkipRetryError(err))
			require.Equal(t, tc.visible, w.Body.Len() > 0)
			require.Equal(t, tc.input, usage.PromptTokens)
			require.Equal(t, tc.output, usage.CompletionTokens)
		})
	}
}
func TestResponsesFailoverKeepsOnlyWinningPreface(t *testing.T) {
	c, w, info := responsesFailoverFixture(t)
	_, err := OaiResponsesStreamHandler(c, info, &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(prefaceFixture))})
	require.NotNil(t, err)
	require.False(t, types.IsSkipRetryError(err))
	winner := strings.ReplaceAll(prefaceFixture, "resp_failed", "resp_winner") + completedFixture
	usage, err := OaiResponsesStreamHandler(c, info, &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(winner))})
	require.Nil(t, err)
	require.NotContains(t, w.Body.String(), "resp_failed")
	require.Contains(t, w.Body.String(), "resp_winner")
	require.Equal(t, 15, usage.TotalTokens)
	require.Less(t, strings.Index(w.Body.String(), "response.created"), strings.Index(w.Body.String(), "response.completed"))
}
func TestResponsesFailoverTerminalAndCancellation(t *testing.T) {
	for _, terminal := range []string{"response.completed", "response.incomplete"} {
		t.Run(terminal, func(t *testing.T) {
			c, w, info := responsesFailoverFixture(t)
			body := prefaceFixture + "data: {\"type\":\"" + terminal + "\",\"response\":{\"usage\":{\"input_tokens\":0,\"output_tokens\":0}}}\n\n"
			_, err := OaiResponsesStreamHandler(c, info, &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))})
			require.Nil(t, err)
			require.Contains(t, w.Body.String(), terminal)
		})
	}
	t.Run("canceled", func(t *testing.T) {
		c, _, info := responsesFailoverFixture(t)
		ctx, cancel := context.WithCancel(c.Request.Context())
		cancel()
		c.Request = c.Request.WithContext(ctx)
		_, err := OaiResponsesStreamHandler(c, info, &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(prefaceFixture))})
		require.NotNil(t, err)
		require.True(t, types.IsSkipRetryError(err))
	})
}
func TestResponsesFailoverPrefaceClassification(t *testing.T) {
	require.True(t, isResponsesPreface(dto.ResponsesStreamResponse{Type: "response.output_item.added", Item: &dto.ResponsesOutput{Type: "message"}}))
	require.False(t, isResponsesPreface(dto.ResponsesStreamResponse{Type: "response.output_item.added", Item: &dto.ResponsesOutput{Type: "web_search_call"}}))
	require.False(t, isResponsesPreface(dto.ResponsesStreamResponse{Type: "provider.unknown"}))
}
