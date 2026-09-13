package controller

import (
	"errors"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVideoTasksNeverRetry(t *testing.T) {
	previous := common.RetryTimes
	common.RetryTimes = 9
	t.Cleanup(func() { common.RetryTimes = previous })
	require.Zero(t, taskRelayRetryLimit(relayconstant.RelayModeVideoSubmit))
	require.Zero(t, taskRelayRetryLimit(relayconstant.RelayModeUnknown))
	require.Equal(t, 9, taskRelayRetryLimit(relayconstant.RelayModeSunoSubmit))
	for _, path := range []string{"/pg/videos", "/v1/videos", "/v1/videos/task_original/remix"} {
		for _, status := range []int{307, 400, 401, 403, 404, 408, 429, 500, 502, 503, 504} {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, path, nil)
			require.False(t, shouldRetryTaskRelay(c, 151, &dto.TaskError{StatusCode: status, Error: errors.New("fixture")}, 9), "%s %d", path, status)
		}
	}
}
func TestVideoModelWrongEndpointRejectedBeforeBilling(t *testing.T) {
	for _, path := range []string{"/v1/chat/completions", "/v1/responses"} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, path, nil)
		c.Set("original_model", "【稳定】sd2.5-720p（按秒）")
		Relay(c, types.RelayFormatOpenAI)
		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "/v1/videos")
	}
}
