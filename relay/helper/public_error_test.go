package helper

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newPublicErrorTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	return c, recorder
}

func TestStringDataSanitizesSensitiveErrorPayload(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	c, recorder := newPublicErrorTestContext()
	err := StringData(c, `{"type":"error","error":{"message":"No available channel for model m under group secret (distributor)","type":"upstream_error","code":"server_error"}}`)
	require.NoError(t, err)
	require.Contains(t, recorder.Body.String(), common.PublicPoolChannelUnavailableMessage)
	require.Contains(t, recorder.Body.String(), `"type":"new_api_error"`)
	require.Contains(t, recorder.Body.String(), `"code":"pool_channel_unavailable"`)
	require.NotContains(t, recorder.Body.String(), "secret")
	require.NotContains(t, recorder.Body.String(), "distributor")
}

func TestResponseChunkDataNormalizesSensitiveEventType(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	c, recorder := newPublicErrorTestContext()
	ResponseChunkData(c, dto.ResponsesStreamResponse{Type: "upstream_error"}, `{"type":"upstream_error","error":{"message":"relay server unavailable"}}`)
	require.Contains(t, recorder.Body.String(), "event: error")
	require.Contains(t, recorder.Body.String(), common.PublicPoolChannelUnavailableMessage)
	require.NotContains(t, recorder.Body.String(), "event: upstream_error")
	require.NotContains(t, recorder.Body.String(), "relay server")
}

func TestStringDataLeavesNormalChunkUntouched(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	c, recorder := newPublicErrorTestContext()
	payload := `{"object":"chat.completion.chunk","choices":[{"delta":{"content":"explain the upstream group architecture"}}]}`
	require.NoError(t, StringData(c, payload))
	require.Contains(t, recorder.Body.String(), payload)
}
