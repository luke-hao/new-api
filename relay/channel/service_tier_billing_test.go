package channel

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const fastBillingTestExpression = `param("service_tier") == "fast" ? tier("fast",p*4+c*20) : tier("normal",p*2+c*10)`

type tierBillingReserve struct {
	target int
	err    error
}

func (b *tierBillingReserve) Settle(int) error         { return nil }
func (b *tierBillingReserve) Refund(*gin.Context)      {}
func (b *tierBillingReserve) NeedsRefund() bool        { return false }
func (b *tierBillingReserve) GetPreConsumedQuota() int { return 0 }
func (b *tierBillingReserve) Reserve(target int) error { b.target = target; return b.err }

func TestServiceTierChecksFinalOutboundRequest(t *testing.T) {
	service.InitHttpClient()
	original := model_setting.GetGlobalSettings().PassThroughRequestEnabled
	model_setting.GetGlobalSettings().PassThroughRequestEnabled = false
	t.Cleanup(func() { model_setting.GetGlobalSettings().PassThroughRequestEnabled = original })
	for _, mode := range []int{relayconstant.RelayModeChatCompletions, relayconstant.RelayModeResponses} {
		for _, configured := range []bool{false, true} {
			for _, tc := range []struct {
				name                         string
				allow, passthrough           bool
				incoming, override, wantTier string
			}{
				{name: "filtered", incoming: "fast"},
				{name: "allow_tier", allow: true, incoming: "fast", wantTier: "fast"},
				{name: "raw_passthrough", passthrough: true, incoming: "fast", wantTier: "fast"},
				{name: "priority_alias", allow: true, incoming: "priority", wantTier: "priority"},
				{name: "override_adds", override: "fast", wantTier: "fast"},
				{name: "override_removes", allow: true, incoming: "fast", override: "default", wantTier: "default"},
				{name: "raw_skips_override", passthrough: true, incoming: "fast", override: "default", wantTier: "fast"},
			} {
				t.Run(fmt.Sprintf("%d/%t/%s", mode, configured, tc.name), func(t *testing.T) {
					calls := 0
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						calls++
						body, err := io.ReadAll(r.Body)
						require.NoError(t, err)
						require.Equal(t, tc.wantTier, gjson.GetBytes(body, "service_tier").String())
						io.WriteString(w, `{}`)
					}))
					defer server.Close()
					c, _ := gin.CreateTestContext(httptest.NewRecorder())
					c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
					reserve := &tierBillingReserve{}
					info := &relaycommon.RelayInfo{
						OriginModelName: "fixture", UsingGroup: "default", UserGroup: "default", RelayMode: mode,
						Billing: reserve, ChannelMeta: &relaycommon.ChannelMeta{
							ChannelOtherSettings: dto.ChannelOtherSettings{AllowServiceTier: tc.allow},
							ChannelSetting:       dto.ChannelSettings{PassThroughBodyEnabled: tc.passthrough},
						},
					}
					if configured {
						info.TieredBillingSnapshot = &billingexpr.BillingSnapshot{BillingMode: "tiered_expr", ExprString: fastBillingTestExpression, ExprHash: billingexpr.ExprHashString(fastBillingTestExpression), QuotaPerUnit: 500000, GroupRatio: 1, EstimatedPromptTokens: 100}
						info.BillingRequestInput = &billingexpr.RequestInput{Body: []byte(`{"service_tier":"fast"}`)}
					}
					data := []byte(fmt.Sprintf(`{"model":"fixture","service_tier":%q}`, tc.incoming))
					var err error
					if !tc.passthrough {
						data, err = relaycommon.RemoveDisabledFields(data, info.ChannelOtherSettings, false)
						require.NoError(t, err)
						if tc.override != "" {
							info.ParamOverride = map[string]any{"service_tier": tc.override}
							data, err = relaycommon.ApplyParamOverrideWithRelayInfo(data, info)
							require.NoError(t, err)
						}
					}
					req, _ := http.NewRequest(http.MethodPost, server.URL, strings.NewReader(string(data)))
					req.Header.Set("Authorization", "upstream-fixture")
					req.Header.Set("X-Fixture", "final-header")
					resp, err := DoRequest(c, req, info)
					if billingexpr.IsFastServiceTier(tc.wantTier) && !configured {
						require.Error(t, err)
						var apiErr *types.NewAPIError
						require.ErrorAs(t, err, &apiErr)
						require.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
						require.Equal(t, types.ErrorCodeModelPriceError, apiErr.GetErrorCode())
						require.Contains(t, err.Error(), "fast 价格未配置")
						require.Equal(t, 0, calls)
						return
					}
					require.NoError(t, err)
					resp.Body.Close()
					require.Equal(t, 1, calls)
					if configured {
						expected := 100
						if billingexpr.IsFastServiceTier(tc.wantTier) {
							expected = 200
						}
						require.Equal(t, expected, reserve.target)
						ok, quota, _ := service.TryTieredSettle(info, billingexpr.TokenParams{P: 100})
						require.True(t, ok)
						require.Equal(t, expected, quota)
						require.Empty(t, info.OutboundBillingRequestInput.Headers["Authorization"])
						require.Equal(t, "final-header", info.OutboundBillingRequestInput.Headers["X-Fixture"])
					}
				})
			}
		}
	}
}

func TestServiceTierReserveFailureStopsBeforeUpstream(t *testing.T) {
	service.InitHttpClient()
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
	defer server.Close()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
	info := &relaycommon.RelayInfo{OriginModelName: "fixture", UsingGroup: "default", UserGroup: "default", RelayMode: relayconstant.RelayModeResponses, ChannelMeta: &relaycommon.ChannelMeta{}, Billing: &tierBillingReserve{err: fmt.Errorf("insufficient quota")}, TieredBillingSnapshot: &billingexpr.BillingSnapshot{BillingMode: "tiered_expr", ExprString: fastBillingTestExpression, EstimatedPromptTokens: 100, QuotaPerUnit: 500000}}
	req, _ := http.NewRequest(http.MethodPost, server.URL, strings.NewReader(`{"service_tier":"fast"}`))
	_, err := DoRequest(c, req, info)
	require.ErrorContains(t, err, "insufficient quota")
	require.Zero(t, calls)
}
