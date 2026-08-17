package service

import (
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestMaskPublicUpstreamTopologyError(t *testing.T) {
	t.Parallel()

	raw := types.WithOpenAIError(types.OpenAIError{
		Message:  "No available channel for model claude-fable-5 under group F4-AWS-Kiro-2 (distributor)",
		Type:     "upstream_error",
		Code:     "server_error",
		Metadata: []byte(`{"group":"F4-AWS-Kiro-2"}`),
	}, http.StatusServiceUnavailable)

	publicErr, masked := MaskPublicUpstreamTopologyError(raw)
	require.True(t, masked)
	require.NotSame(t, raw, publicErr)
	require.Equal(t, http.StatusServiceUnavailable, publicErr.StatusCode)
	require.Equal(t, common.PublicPoolChannelUnavailableMessage, publicErr.Error())
	require.Equal(t, types.ErrorCodePoolChannelUnavailable, publicErr.GetErrorCode())
	require.Equal(t, types.ErrorTypeNewAPIError, publicErr.GetErrorType())

	oai := publicErr.ToOpenAIError()
	require.Equal(t, common.PublicPoolChannelUnavailableMessage, oai.Message)
	require.Equal(t, "new_api_error", oai.Type)
	require.Equal(t, types.ErrorCodePoolChannelUnavailable, oai.Code)
	require.Empty(t, oai.Metadata)

	// The raw error is retained for administrator logs and diagnostics.
	require.Contains(t, raw.Error(), "F4-AWS-Kiro-2")
}

func TestMaskPublicUpstreamTopologyErrorLeavesOrdinaryErrors(t *testing.T) {
	t.Parallel()

	raw := types.WithOpenAIError(types.OpenAIError{
		Message: "rate limit reached for requests per minute",
		Type:    "rate_limit_error",
		Code:    "rate_limit_exceeded",
	}, http.StatusTooManyRequests)

	publicErr, masked := MaskPublicUpstreamTopologyError(raw)
	require.False(t, masked)
	require.Same(t, raw, publicErr)
}

func TestTaskErrorWrapperSanitizesTopologyDetails(t *testing.T) {
	t.Parallel()

	raw := errors.New("No available channel for model m under group secret (distributor)")
	wrapped := TaskErrorWrapper(raw, "server_error", http.StatusServiceUnavailable)
	require.Equal(t, common.PublicPoolChannelUnavailableMessage, wrapped.Message)
	require.Equal(t, common.PublicPoolChannelUnavailableCode, wrapped.Code)
	require.Equal(t, raw, wrapped.Error)
}
