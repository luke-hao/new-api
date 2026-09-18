package service

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestResponsesFailoverCooldownScope(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	responsesFailures.Lock()
	old := responsesFailures.until
	responsesFailures.until = make(map[responsesFailureKey]time.Time)
	responsesFailures.Unlock()
	t.Cleanup(func() { responsesFailures.Lock(); responsesFailures.until = old; responsesFailures.Unlock() })
	info := &relaycommon.RelayInfo{OriginModelName: "astra", ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 156}}
	info.UsingGroup = "group-a"
	err := types.NewErrorWithStatusCode(errors.New("broken stream"), types.ErrorCodeIncompleteStream, 502)
	RecordResponsesChannelFailure(c, info, err)
	require.True(t, IsResponsesChannelCoolingDown(c, "group-a", "astra", 156))
	require.False(t, IsResponsesChannelCoolingDown(c, "group-b", "astra", 156))
	require.False(t, IsResponsesChannelCoolingDown(c, "group-a", "luna", 156))
	require.True(t, c.GetBool(ResponsesStreamFailedKey))
	responsesFailures.Lock()
	responsesFailures.until[responsesFailureKey{"group-a", "astra", 156}] = time.Now().Add(-time.Second)
	responsesFailures.Unlock()
	require.False(t, IsResponsesChannelCoolingDown(c, "group-a", "astra", 156))
	ctx, cancel := context.WithCancel(c.Request.Context())
	cancel()
	c.Request = c.Request.WithContext(ctx)
	RecordResponsesChannelFailure(c, info, err)
	require.False(t, IsResponsesChannelCoolingDown(c, "group-a", "astra", 156))
}
