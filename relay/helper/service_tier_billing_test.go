package helper

import (
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServiceTierRetryReplacesOutboundProbe(t *testing.T) {
	expr := `param("service_tier") == "fast" ? tier("fast",p*4) : tier("normal",p*2)`
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{OriginModelName: "fixture", UsingGroup: "default", UserGroup: "default", TieredBillingSnapshot: &billingexpr.BillingSnapshot{BillingMode: "tiered_expr", ExprString: expr, QuotaPerUnit: 500000, EstimatedPromptTokens: 100}}
	require.NoError(t, PrepareServiceTierBilling(c, info, []byte(`{"service_tier":"fast"}`), http.Header{}))
	require.Equal(t, 200, info.PriceData.QuotaToPreConsume)
	require.Nil(t, ObserveServiceTierBilling(info, []byte(`{"service_tier":"priority"}`)))
	require.NoError(t, PrepareServiceTierBilling(c, info, []byte(`{}`), http.Header{}))
	require.Empty(t, info.UpstreamServiceTier)
	require.Empty(t, info.RequestedServiceTier)
	require.Equal(t, 100, info.PriceData.QuotaToPreConsume)
	require.Equal(t, `{}`, string(info.OutboundBillingRequestInput.Body))
}
func TestUnexpectedUnpricedFastResponse(t *testing.T) {
	info := &relaycommon.RelayInfo{OriginModelName: "fixture"}
	err := ObserveServiceTierBilling(info, []byte(`{"response":{"service_tier":"priority"}}`))
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "fast 价格未配置")
	require.Nil(t, ObserveServiceTierBilling(info, []byte(`{"service_tier":"default"}`)))
}
