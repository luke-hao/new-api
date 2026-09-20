package service

import (
	"errors"
	"github.com/QuantumNous/new-api/types"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

var recoveryStatusCode = regexp.MustCompile(`(?i)status(?: code)?[: =]+(\d{3})`)

// Balance takes precedence over authentication: several providers return 403
// for exhausted credit. This classifier does not expand auto-disable triggers.
func ChannelDisableReason(err *types.NewAPIError) string {
	if IsUpstreamBalanceError(err) {
		return "balance"
	}
	if err == nil {
		return "other"
	}
	msg := strings.ToLower(err.Error())
	if err.StatusCode == http.StatusTooManyRequests || containsRecoveryPhrase(msg, "rate limit", "rate_limit", "too many requests", "限流", "请求过于频繁") {
		return "rate_limit"
	}
	if err.StatusCode == 408 || err.StatusCode == 504 || err.StatusCode == 524 || err.GetErrorCode() == types.ErrorCodeChannelResponseTimeExceeded || containsRecoveryPhrase(msg, "timeout", "timed out", "deadline exceeded", "超时", "响应时间") {
		return "timeout"
	}
	if err.StatusCode == 401 || err.StatusCode == 403 || containsRecoveryPhrase(msg, "invalid api key", "invalid_api_key", "invalid key", "unauthorized", "authentication", "permission denied", "令牌无效", "密钥无效") {
		return "authentication"
	}
	return "other"
}
func ChannelDisableReasonFromText(message string) string {
	status := 0
	if match := recoveryStatusCode.FindStringSubmatch(message); len(match) > 1 {
		status, _ = strconv.Atoi(match[1])
	}
	return ChannelDisableReason(types.NewOpenAIError(errors.New(message), types.ErrorCodeBadResponseStatusCode, status))
}
func containsRecoveryPhrase(message string, phrases ...string) bool {
	for _, phrase := range phrases {
		if strings.Contains(message, phrase) {
			return true
		}
	}
	return false
}
