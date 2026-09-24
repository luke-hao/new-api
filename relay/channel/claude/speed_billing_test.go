package claude

import (
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClaudeActualSpeedControlsSettlement(t *testing.T) {
	old := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = old })
	for _, mode := range []string{"json", "stream", "passthrough"} {
		for _, speed := range []string{"standard", "fast"} {
			t.Run(mode+"/"+speed, func(t *testing.T) {
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest("POST", "/v1/messages", nil)
				info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatClaude, ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "claude-opus-5"}, TieredBillingSnapshot: &billingexpr.BillingSnapshot{BillingMode: "tiered_expr", ExprString: `(p*5+c*25+cr*0.5+cc*6.25+cc1h*10)*(param("speed")=="fast"?2:1)`, QuotaPerUnit: 500000, GroupRatio: 1}, OutboundBillingRequestInput: &billingexpr.RequestInput{Body: []byte(`{"speed":"fast"}`)}}
				msg := `{"id":"msg_speed","type":"message","role":"assistant","model":"claude-opus-5","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{"input_tokens":100,"output_tokens":20,"speed":"` + speed + `"}}`
				body := msg
				if mode != "json" {
					body = "data: {\"type\":\"message_start\",\"message\":" + msg + "}\n\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":20}}\n\ndata: {\"type\":\"message_stop\"}\n\n"
					info.IsStream = true
				}
				resp := &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
				var usage *dto.Usage
				var err *types.NewAPIError
				switch mode {
				case "json":
					usage, err = ClaudeHandler(c, resp, info)
				case "stream":
					usage, err = ClaudeStreamHandler(c, resp, info)
				default:
					usage, err = ClaudeStreamPassthroughHandler(c, resp, info)
				}
				require.Nil(t, err)
				require.NotNil(t, usage)
				require.Equal(t, speed, info.UpstreamClaudeSpeed)
				ok, quota, _ := service.TryTieredSettle(info, billingexpr.TokenParams{P: 100, C: 20})
				require.True(t, ok)
				expected := 500
				if speed == "fast" {
					expected = 1000
				}
				require.Equal(t, expected, quota)
				require.Equal(t, `{"speed":"fast"}`, string(info.OutboundBillingRequestInput.Body))
			})
		}
	}
}
