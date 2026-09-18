package controller

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestResponsesFailoverRetryPolicy(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	failure := types.NewErrorWithStatusCode(errors.New("incomplete stream"), types.ErrorCodeIncompleteStream, 502)
	require.True(t, shouldRetry(c, failure, 9))
	require.False(t, shouldRetry(c, failure, 0))
	for _, code := range []int{408, 504, 524} {
		timeout := types.NewErrorWithStatusCode(errors.New("upstream timeout"), types.ErrorCodeDoRequestFailed, code)
		require.True(t, shouldRetry(c, timeout, 9))
	}
	types.ErrOptionWithSkipRetry()(failure)
	require.False(t, shouldRetry(c, failure, 9))
	ctx, cancel := context.WithCancel(c.Request.Context())
	cancel()
	c.Request = c.Request.WithContext(ctx)
	failure = types.NewErrorWithStatusCode(errors.New("incomplete stream"), types.ErrorCodeIncompleteStream, 502)
	require.False(t, shouldRetry(c, failure, 9))
}
