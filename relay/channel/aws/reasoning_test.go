package aws

import (
	"bytes"
	appcommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAWSReasoningMatchesFinalBody(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		t.Run(map[bool]string{false: "converted", true: "passthrough"}[passthrough], func(t *testing.T) {
			raw := []byte(`{"model":"claude-test","stream":true,"thinking":{"type":"adaptive"},"output_config":{"effort":"max"},"messages":[]}`)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(raw))
			defer appcommon.CleanupBodyStorage(c)
			info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelSetting: dto.ChannelSettings{PassThroughBodyEnabled: passthrough}}}
			converted := map[string]interface{}{"thinking": map[string]interface{}{"type": "enabled", "budget_tokens": 2048}}
			body, err := buildAwsRequestBody(c, info, converted)
			require.NoError(t, err)
			if passthrough {
				require.NotContains(t, string(body), `"model"`)
				require.NotContains(t, string(body), `"stream"`)
				require.Equal(t, "max", info.ReasoningEffort)
				require.Equal(t, relaycommon.ReasoningSpecified, info.ReasoningStatus)
			} else {
				require.Equal(t, relaycommon.ReasoningEnabled, info.ReasoningStatus)
				require.Equal(t, appcommon.GetPointer(2048), info.ThinkingBudgetTokens)
			}
		})
	}
}
