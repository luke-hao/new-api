package ratio_setting

import (
	"math"
	"testing"
)

func TestOfficialModelPricingDefaults(t *testing.T) {
	tests := []struct {
		model           string
		inputRatio      float64
		completionRatio float64
		cacheRatio      float64
	}{
		{model: "kimi-k3", inputRatio: 0.02 * RMB, completionRatio: 5, cacheRatio: 0.1},
		{model: "grok-4.5", inputRatio: 1, completionRatio: 3, cacheRatio: 0.15},
		{model: "grok-4.5-latest", inputRatio: 1, completionRatio: 3, cacheRatio: 0.15},
		{model: "grok-build-latest", inputRatio: 1, completionRatio: 3, cacheRatio: 0.15},
		{model: "glm-5.2", inputRatio: 0.7, completionRatio: 4.4 / 1.4, cacheRatio: 0.26 / 1.4},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			if got := defaultModelRatio[tt.model]; math.Abs(got-tt.inputRatio) > 1e-12 {
				t.Fatalf("input ratio = %v, want %v", got, tt.inputRatio)
			}
			if got := defaultCompletionRatio[tt.model]; math.Abs(got-tt.completionRatio) > 1e-12 {
				t.Fatalf("completion ratio = %v, want %v", got, tt.completionRatio)
			}
			if got := defaultCacheRatio[tt.model]; math.Abs(got-tt.cacheRatio) > 1e-12 {
				t.Fatalf("cache ratio = %v, want %v", got, tt.cacheRatio)
			}
		})
	}
}
