package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSpeedtestPing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/speedtest/ping", nil)

	SpeedtestPing(ctx)

	require.Equal(t, http.StatusNoContent, ctx.Writer.Status())
	require.Contains(t, response.Header().Get("Cache-Control"), "no-store")
}

func TestSpeedtestUploadReportsReceivedBytes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	constant.MaxRequestBodyMB = 128
	body := bytes.Repeat([]byte("a"), 4096)
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/speedtest/upload", bytes.NewReader(body))
	ctx.Set(common.RequestIdKey, "request-test")

	SpeedtestUpload(ctx)

	require.Equal(t, http.StatusOK, response.Code)
	var payload struct {
		ReceivedBytes int64   `json:"received_bytes"`
		ServerReadMs  float64 `json:"server_read_ms"`
		RequestID     string  `json:"request_id"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
	require.Equal(t, int64(len(body)), payload.ReceivedBytes)
	require.GreaterOrEqual(t, payload.ServerReadMs, float64(0))
	require.Equal(t, "request-test", payload.RequestID)
}

func TestSpeedtestUploadRejectsOversizedBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previous := constant.MaxRequestBodyMB
	constant.MaxRequestBodyMB = 1
	t.Cleanup(func() { constant.MaxRequestBodyMB = previous })
	body := bytes.Repeat([]byte("a"), (1<<20)+1)
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/speedtest/upload", bytes.NewReader(body))

	SpeedtestUpload(ctx)

	require.Equal(t, http.StatusRequestEntityTooLarge, response.Code)
}
