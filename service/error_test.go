package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestResetStatusCode(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		statusCode       int
		statusCodeConfig string
		expectedCode     int
	}{
		{
			name:             "map string value",
			statusCode:       429,
			statusCodeConfig: `{"429":"503"}`,
			expectedCode:     503,
		},
		{
			name:             "map int value",
			statusCode:       429,
			statusCodeConfig: `{"429":503}`,
			expectedCode:     503,
		},
		{
			name:             "skip invalid string value",
			statusCode:       429,
			statusCodeConfig: `{"429":"bad-code"}`,
			expectedCode:     429,
		},
		{
			name:             "skip status code 200",
			statusCode:       200,
			statusCodeConfig: `{"200":503}`,
			expectedCode:     200,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			newAPIError := &types.NewAPIError{
				StatusCode: tc.statusCode,
			}
			ResetStatusCode(newAPIError, tc.statusCodeConfig)
			require.Equal(t, tc.expectedCode, newAPIError.StatusCode)
		})
	}
}

func TestRelayErrorHandlerTruncatesInvalidJSONBodyInLog(t *testing.T) {
	withDebugEnabled(t, false)

	body := strings.Repeat("b", common.LocalLogContentLimit+256)
	var logBuffer bytes.Buffer

	common.LogWriterMu.Lock()
	oldWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &logBuffer
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultErrorWriter = oldWriter
		common.LogWriterMu.Unlock()
	})

	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, false)

	require.NotNil(t, newAPIError)
	require.Equal(t, "bad response status code 500", newAPIError.Error())
	require.Contains(t, logBuffer.String(), "[truncated")
	require.Contains(t, logBuffer.String(), fmt.Sprintf("original_length=%d", len(body)))
	require.NotContains(t, logBuffer.String(), strings.Repeat("b", common.LocalLogContentLimit+1))
}

func TestRelayErrorHandlerKeepsStructuredErrorMessage(t *testing.T) {
	message := strings.Repeat("c", common.LocalLogContentLimit+256)
	body := `{"message":"` + message + `"}`
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, false)

	require.NotNil(t, newAPIError)
	require.Equal(t, message, newAPIError.Error())
}

func TestRelayErrorHandlerKeepsOpenAIErrorMessage(t *testing.T) {
	message := strings.Repeat("d", common.LocalLogContentLimit+256)
	body := `{"error":{"message":"` + message + `","type":"server_error","code":"server_error"}}`
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, false)

	require.NotNil(t, newAPIError)
	require.Equal(t, message, newAPIError.Error())
}

func TestRelayErrorHandlerKeepsInvalidJSONBodyInDebugLog(t *testing.T) {
	withDebugEnabled(t, true)

	body := strings.Repeat("e", common.LocalLogContentLimit+256)
	var logBuffer bytes.Buffer

	common.LogWriterMu.Lock()
	oldWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &logBuffer
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultErrorWriter = oldWriter
		common.LogWriterMu.Unlock()
	})

	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, false)

	require.NotNil(t, newAPIError)
	require.NotContains(t, logBuffer.String(), "[truncated")
	require.Contains(t, logBuffer.String(), body)
}

func TestIsUpstreamBalanceError(t *testing.T) {
	tests := []struct {
		name string
		err  *types.NewAPIError
		want bool
	}{
		{
			name: "payment required status",
			err:  types.NewOpenAIError(fmt.Errorf("payment is required"), types.ErrorCodeBadResponseStatusCode, http.StatusPaymentRequired),
			want: true,
		},
		{
			name: "OpenAI insufficient quota code",
			err: types.WithOpenAIError(types.OpenAIError{
				Message: "billing quota exhausted",
				Type:    "insufficient_quota",
				Code:    "insufficient_quota",
			}, http.StatusTooManyRequests),
			want: true,
		},
		{
			name: "known credit message",
			err:  types.NewOpenAIError(fmt.Errorf("Your credit balance is too low to access this resource"), types.ErrorCodeBadResponseStatusCode, http.StatusForbidden),
			want: true,
		},
		{
			name: "Chinese balance message",
			err:  types.NewOpenAIError(fmt.Errorf("上游账户余额不足"), types.ErrorCodeBadResponseStatusCode, http.StatusForbidden),
			want: true,
		},
		{
			name: "local user quota stays local",
			err: types.NewErrorWithStatusCode(
				fmt.Errorf("用户额度不足, 剩余额度: $0.00"),
				types.ErrorCodeInsufficientUserQuota,
				http.StatusForbidden,
			),
			want: false,
		},
		{
			name: "ordinary rate limit",
			err: types.WithOpenAIError(types.OpenAIError{
				Message: "rate limit reached for requests per minute",
				Type:    "rate_limit_error",
				Code:    "rate_limit_exceeded",
			}, http.StatusTooManyRequests),
			want: false,
		},
		{
			name: "generic forbidden",
			err:  types.NewOpenAIError(fmt.Errorf("permission denied"), types.ErrorCodeBadResponseStatusCode, http.StatusForbidden),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, IsUpstreamBalanceError(tt.err))
		})
	}
}

func TestShouldDisableChannelForUpstreamBalanceWithoutGlobalAutoDisable(t *testing.T) {
	original := common.AutomaticDisableChannelEnabled
	common.AutomaticDisableChannelEnabled = false
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = original
	})

	upstreamBalanceError := types.WithOpenAIError(types.OpenAIError{
		Message: "Your credit balance is too low",
		Type:    "insufficient_quota",
		Code:    "insufficient_quota",
	}, http.StatusTooManyRequests)
	require.True(t, ShouldDisableChannel(upstreamBalanceError))

	localUserQuotaError := types.NewErrorWithStatusCode(
		fmt.Errorf("用户额度不足"),
		types.ErrorCodeInsufficientUserQuota,
		http.StatusForbidden,
	)
	require.False(t, ShouldDisableChannel(localUserQuotaError))
}

func withDebugEnabled(t *testing.T, enabled bool) {
	t.Helper()

	oldDebug := common.DebugEnabled
	common.DebugEnabled = enabled
	t.Cleanup(func() {
		common.DebugEnabled = oldDebug
	})
}
