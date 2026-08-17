package billing_setting

import (
	"math"
	"testing"

	"github.com/QuantumNous/new-api/pkg/billingexpr"
)

func TestGrok45OfficialTieredPricing(t *testing.T) {
	if err := SmokeTestExpr(grok45BillingExpr); err != nil {
		t.Fatalf("SmokeTestExpr() error = %v", err)
	}

	tests := []struct {
		name string
		len  float64
		want float64
	}{
		{name: "short context", len: 199999, want: 8.3},
		{name: "long context threshold", len: 200000, want: 16.6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := billingexpr.RunExpr(grok45BillingExpr, billingexpr.TokenParams{
				P:   1,
				C:   1,
				CR:  1,
				Len: tt.len,
			})
			if err != nil {
				t.Fatalf("RunExpr() error = %v", err)
			}
			if math.Abs(got-tt.want) > 1e-12 {
				t.Fatalf("cost = %v, want %v", got, tt.want)
			}
		})
	}
}
