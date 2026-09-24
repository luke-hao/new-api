package service

import (
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestServiceTierSettlementUsesActualTier(t *testing.T) {
	for _, tc := range []struct {
		tier  string
		quota int
	}{{"", 200}, {"priority", 200}, {"fast", 200}, {"default", 100}} {
		t.Run(tc.tier, func(t *testing.T) {
			info := makeRelayInfo(probeExpr, 1, 100, 0)
			info.BillingRequestInput = &billingexpr.RequestInput{Body: []byte(`{"service_tier":"default"}`)}
			info.OutboundBillingRequestInput = &billingexpr.RequestInput{Body: []byte(`{"service_tier":"fast"}`)}
			info.UpstreamServiceTier = tc.tier
			ok, quota, result := TryTieredSettle(info, billingexpr.TokenParams{P: 100})
			require.True(t, ok)
			require.Equal(t, tc.quota, quota)
			require.NotNil(t, result)
			require.Equal(t, `{"service_tier":"fast"}`, string(info.OutboundBillingRequestInput.Body))
		})
	}
}
