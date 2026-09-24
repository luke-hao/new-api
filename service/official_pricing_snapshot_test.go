package service

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestOfficialPricingSnapshot20260924(t *testing.T) {
	data, err := os.ReadFile("../docs/pricing/openai-anthropic-2026-09-24.json")
	require.NoError(t, err)
	var catalog struct {
		Models map[string]struct {
			Expression string `json:"expression"`
		}
	}
	require.NoError(t, common.Unmarshal(data, &catalog))
	require.Len(t, catalog.Models, 22)
	for model, row := range catalog.Models {
		t.Run(model, func(t *testing.T) {
			_, _, err := billingexpr.RunExprWithRequest(row.Expression, billingexpr.TokenParams{P: 1000, C: 100, Len: 1000}, billingexpr.RequestInput{Body: []byte(`{}`)})
			require.NoError(t, err)
		})
	}
	cases := []struct {
		name, model, body string
		length, want      float64
	}{
		{"sol_boundary", "gpt-5.6-sol", `{}`, 272000, 5115},
		{"sol_long", "gpt-5.6-sol", `{}`, 272001, 9730},
		{"sol_fast", "gpt-5.6-sol", `{"service_tier":"fast"}`, 272000, 10230},
		{"sol_priority_long", "gpt-5.6-sol", `{"service_tier":"priority"}`, 272001, 19460},
		{"sol_flex_long", "gpt-5.6-sol", `{"service_tier":"flex"}`, 272001, 4865},
		{"alias", "codex-auto-review", `{"service_tier":"fast"}`, 272001, 19460},
		{"luna", "gpt-6-luna", `{}`, 1000, 127.875},
		{"old_fast_flat", "gpt-5.5", `{"service_tier":"fast"}`, 272001, 16375},
		{"claude_ttls", "claude-opus-5-5", `{}`, 1000, 5135},
		{"claude_fast_us", "claude-opus-5-5", `{"speed":"fast","inference_geo":"us"}`, 1000, 11297},
		{"claude_standard_fallback", "claude-opus-4-6", `{"speed":"fast"}`, 1000, 6443.75},
		{"claude_fable_cache", "claude-fable-5-1", `{}`, 1000, 12812.5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cost, _, err := billingexpr.RunExprWithRequest(catalog.Models[tc.model].Expression, billingexpr.TokenParams{P: 1000, C: 50, CR: 100, CC: 15, CC1h: 5, Len: tc.length}, billingexpr.RequestInput{Body: []byte(tc.body)})
			require.NoError(t, err)
			require.InDelta(t, tc.want, cost, 1e-8)
		})
	}
	// Actual provider downgrades override the final request only for billing.
	for _, tc := range []struct {
		model, body, tier, speed string
		want                     int
	}{
		{"gpt-5.6-sol", `{"service_tier":"fast"}`, "default", "", 1250},
		{"claude-opus-5-5", `{"speed":"fast"}`, "", "standard", 1250},
	} {
		info := &relaycommon.RelayInfo{TieredBillingSnapshot: &billingexpr.BillingSnapshot{BillingMode: "tiered_expr", ExprString: catalog.Models[tc.model].Expression, QuotaPerUnit: 500000, GroupRatio: .5}, OutboundBillingRequestInput: &billingexpr.RequestInput{Body: []byte(tc.body)}, UpstreamServiceTier: tc.tier, UpstreamClaudeSpeed: tc.speed}
		ok, quota, _ := TryTieredSettle(info, billingexpr.TokenParams{P: 1000, C: 50})
		require.True(t, ok)
		require.Equal(t, tc.want, quota)
		require.Equal(t, tc.body, string(info.OutboundBillingRequestInput.Body))
	}
}
