package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
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

func TestUpstreamNewAPIQuotaOrigin(t *testing.T) {
	message := "用户额度不足, 剩余额度: ＄-0.977194"
	tests := []struct {
		name      string
		makeError func() *types.NewAPIError
		want      bool
	}{
		{"upstream Claude response without code", func() *types.NewAPIError {
			resp := &http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(strings.NewReader(
				`{"error":{"type":"new_api_error","message":"用户额度不足, 剩余额度: ＄-0.977194"},"type":"error"}`))}
			return RelayErrorHandler(context.Background(), resp, false)
		}, true},
		{"upstream test response with body", func() *types.NewAPIError {
			resp := &http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(strings.NewReader(
				`{"error":{"type":"new_api_error","message":"用户额度不足, 剩余额度: ＄-0.977194"},"type":"error"}`))}
			return RelayErrorHandler(context.Background(), resp, true)
		}, true},
		{"upstream account balance message", func() *types.NewAPIError {
			resp := &http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(strings.NewReader(
				`{"message":"Insufficient account balance"}`))}
			return RelayErrorHandler(context.Background(), resp, false)
		}, true},
		{"upstream account balance in structured error", func() *types.NewAPIError {
			return types.WithOpenAIError(types.OpenAIError{Message: "INSUFFICIENT ACCOUNT BALANCE", Type: "invalid_request_error"}, http.StatusForbidden)
		}, true},
		{"local account balance message", func() *types.NewAPIError {
			return types.NewErrorWithStatusCode(fmt.Errorf("Insufficient account balance"), types.ErrorCodeInsufficientUserQuota, http.StatusForbidden)
		}, false},
		{"upstream shares local quota code", func() *types.NewAPIError {
			return types.WithOpenAIError(types.OpenAIError{Message: "account quota exhausted", Type: "new_api_error", Code: "insufficient_user_quota"}, http.StatusForbidden)
		}, true},
		{"upstream native Claude quota", func() *types.NewAPIError {
			return types.WithClaudeError(types.ClaudeError{Message: message, Type: "new_api_error"}, http.StatusForbidden)
		}, true},
		{"upstream precharge rejected", func() *types.NewAPIError {
			return types.WithOpenAIError(types.OpenAIError{Message: "预扣费额度失败, 用户剩余额度: $0.01, 需要预扣费额度: $0.10", Type: "new_api_error"}, http.StatusForbidden)
		}, true},
		{"local wallet quota", func() *types.NewAPIError {
			return types.NewErrorWithStatusCode(fmt.Errorf("%s", message), types.ErrorCodeInsufficientUserQuota, http.StatusForbidden, types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
		}, false},
		{"local precharge rejected", func() *types.NewAPIError {
			return types.NewErrorWithStatusCode(fmt.Errorf("预扣费额度失败, 用户剩余额度: $0.01, 需要预扣费额度: $0.10"), types.ErrorCodeInsufficientUserQuota, http.StatusForbidden, types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
		}, false},
		{"local token quota", func() *types.NewAPIError {
			return types.NewErrorWithStatusCode(fmt.Errorf("余额不足"), types.ErrorCodePreConsumeTokenQuotaFailed, http.StatusForbidden, types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
		}, false},
		{"ordinary upstream forbidden", func() *types.NewAPIError {
			return types.WithOpenAIError(types.OpenAIError{Message: "permission denied", Type: "permission_error"}, http.StatusForbidden)
		}, false},
		{"ordinary upstream rate limit", func() *types.NewAPIError {
			return types.WithOpenAIError(types.OpenAIError{Message: "requests per minute exceeded", Code: "rate_limit_exceeded"}, http.StatusTooManyRequests)
		}, false},
		{"nil error", func() *types.NewAPIError { return nil }, false},
	}
	original := common.AutomaticDisableChannelEnabled
	t.Cleanup(func() { common.AutomaticDisableChannelEnabled = original })
	for _, globalEnabled := range []bool{false, true} {
		common.AutomaticDisableChannelEnabled = globalEnabled
		for _, tt := range tests {
			t.Run(fmt.Sprintf("global=%v/%s", globalEnabled, tt.name), func(t *testing.T) {
				err := tt.makeError()
				require.Equal(t, tt.want, IsUpstreamBalanceError(err), "balance classification")
				// The existing global permission-denied rule remains independent of billing.
				wantDisable := tt.want || (globalEnabled && tt.name == "ordinary upstream forbidden")
				require.Equal(t, wantDisable, ShouldDisableChannel(err), "automatic disable decision")
				if err != nil {
					t.Logf("origin=%s code=%s status=%d balance=%v disable=%v message=%s", err.GetErrorType(), err.GetErrorCode(), err.StatusCode, IsUpstreamBalanceError(err), ShouldDisableChannel(err), err.Error())
				}
			})
		}
	}
}

func TestLocalWalletInsufficientFundsMessage(t *testing.T) {
	for _, entry := range []string{"billing_session", "legacy_preconsume"} {
		for _, quota := range []int{-500, 0, 1000} {
			t.Run(fmt.Sprintf("%s/quota=%d", entry, quota), func(t *testing.T) {
				truncate(t)
				seedUser(t, 93001, quota)
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
				info := &relaycommon.RelayInfo{UserId: 93001, TokenUnlimited: true}
				info.UserSetting.BillingPreference = "wallet_only"
				const pre = 2000
				var apiErr *types.NewAPIError
				if entry == "billing_session" {
					apiErr = PreConsumeBilling(c, pre, info)
				} else {
					apiErr = PreConsumeQuota(c, pre, info)
				}
				require.NotNil(t, apiErr)
				want := "您在本站点的所余资金不足够了, 剩余额度: " + logger.FormatQuota(quota)
				if quota > 0 {
					want += ", 需要预扣费额度: " + logger.FormatQuota(pre)
				}
				require.Equal(t, want, apiErr.Error())
				require.Equal(t, "status_code=403, "+want, apiErr.ErrorWithStatusCode())
				require.Equal(t, want, apiErr.ToOpenAIError().Message)
				require.Equal(t, want, apiErr.ToClaudeError().Message)
				require.Equal(t, types.ErrorCodeInsufficientUserQuota, apiErr.GetErrorCode())
				require.True(t, types.IsSkipRetryError(apiErr))
				require.False(t, IsUpstreamBalanceError(apiErr))
				require.False(t, ShouldDisableChannel(apiErr))
				require.Nil(t, info.Billing)
				require.Equal(t, quota, getUserQuota(t, 93001))
				t.Logf("%s; local=true disable=false balance_unchanged=true", apiErr.ErrorWithStatusCode())
			})
		}
	}
}
