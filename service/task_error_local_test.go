package service

import (
	"errors"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
)

func TestTaskPrechargeFailureStaysLocal(t *testing.T) {
	apiErr := types.NewErrorWithStatusCode(errors.New("insufficient balance"), types.ErrorCodeInsufficientUserQuota, http.StatusForbidden, types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
	taskErr := TaskErrorFromAPIError(apiErr)
	require.True(t, taskErr.LocalError)
	require.Equal(t, http.StatusForbidden, taskErr.StatusCode)
	require.Equal(t, string(types.ErrorCodeInsufficientUserQuota), taskErr.Code)
}
