package billingexpr

import (
	"fmt"
	"math"
)

// QuotaRound converts a float64 quota value to int using half-away-from-zero
// rounding. Every tiered billing path (pre-consume, settlement, breakdown
// validation, log fields) MUST use this function to avoid +-1 discrepancies.
func QuotaRound(f float64) int {
	rounded := math.Round(f)
	if math.IsNaN(rounded) || math.IsInf(rounded, 0) {
		return 0
	}
	maxInt := int(^uint(0) >> 1)
	if rounded <= -float64(maxInt) || rounded >= float64(maxInt) {
		return 0
	}
	return int(rounded)
}

func QuotaRoundSafe(f float64) (int, error) {
	rounded := math.Round(f)
	if math.IsNaN(rounded) || math.IsInf(rounded, 0) {
		return 0, fmt.Errorf("quota is not finite")
	}
	if rounded < 0 {
		return 0, fmt.Errorf("quota cannot be negative")
	}
	maxInt := int(^uint(0) >> 1)
	if rounded >= float64(maxInt) {
		return 0, fmt.Errorf("quota exceeds integer range")
	}
	return int(rounded), nil
}
