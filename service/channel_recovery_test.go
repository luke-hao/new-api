package service

import (
	"errors"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestChannelDisableReason(t *testing.T) {
	for _, tt := range []struct {
		message string
		status  int
		reason  string
	}{
		{"Insufficient account balance", 403, "balance"}, {"rate limit exceeded", 429, "rate_limit"},
		{"context deadline exceeded", 500, "timeout"}, {"invalid api key", 401, "authentication"},
		{"bad gateway", 502, "other"}, {"响应时间 10s 超过阈值 5s", 408, "timeout"},
	} {
		t.Run(tt.reason, func(t *testing.T) {
			err := types.NewOpenAIError(errors.New(tt.message), types.ErrorCodeBadResponseStatusCode, tt.status)
			require.Equal(t, tt.reason, ChannelDisableReason(err))
		})
	}
	require.Equal(t, "balance", ChannelDisableReasonFromText("status code: 403, Insufficient account balance"))
	require.Equal(t, "rate_limit", ChannelDisableReasonFromText("status code: 429, busy"))
	require.Equal(t, "other", ChannelDisableReason(nil))
}
