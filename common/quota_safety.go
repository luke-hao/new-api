package common

import (
	"fmt"
	"math"

	"github.com/shopspring/decimal"
)

const (
	MaxRequestTokens        uint = math.MaxInt32 / 2
	MaxImageGenerationCount uint = 10
	MaxVideoDurationSeconds int  = 600
	MaxCompletionChoices    int  = 16
	MaxToolCalls            uint = 1024
)

func MaxSafeInt() int {
	return int(^uint(0) >> 1)
}

func ValidateOptionalUintLimit(field string, value *uint, max uint) error {
	if value == nil {
		return nil
	}
	if *value > max {
		return fmt.Errorf("%s is invalid, maximum is %d", field, max)
	}
	if *value > uint(MaxSafeInt()) {
		return fmt.Errorf("%s exceeds integer range", field)
	}
	return nil
}

func ValidateOptionalIntRange(field string, value *int, min int, max int) error {
	if value == nil {
		return nil
	}
	if *value < min || *value > max {
		return fmt.Errorf("%s is invalid, valid range is %d-%d", field, min, max)
	}
	return nil
}

func ValidateIntRange(field string, value int, min int, max int) error {
	if value < min || value > max {
		return fmt.Errorf("%s is invalid, valid range is %d-%d", field, min, max)
	}
	return nil
}

func BoundedUintToInt(value uint, max uint) int {
	if value > max {
		return int(max)
	}
	if value > uint(MaxSafeInt()) {
		return MaxSafeInt()
	}
	return int(value)
}

func SafeAddInt(field string, left int, right int) (int, error) {
	if right > 0 && left > MaxSafeInt()-right {
		return 0, fmt.Errorf("%s exceeds integer range", field)
	}
	if right < 0 && left < -MaxSafeInt()-right {
		return 0, fmt.Errorf("%s exceeds integer range", field)
	}
	return left + right, nil
}

func SafeNonNegativeFloatToInt(field string, value float64) (int, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, fmt.Errorf("%s is not finite", field)
	}
	if value < 0 {
		return 0, fmt.Errorf("%s cannot be negative", field)
	}
	if value >= float64(MaxSafeInt()) {
		return 0, fmt.Errorf("%s exceeds integer range", field)
	}
	return int(value), nil
}

func SafeMultiplyQuotaByRatio(quota int, ratio float64, field string) (int, error) {
	if quota < 0 {
		return 0, fmt.Errorf("%s base quota cannot be negative", field)
	}
	if math.IsNaN(ratio) || math.IsInf(ratio, 0) {
		return 0, fmt.Errorf("%s ratio is not finite", field)
	}
	if ratio < 0 {
		return 0, fmt.Errorf("%s ratio cannot be negative", field)
	}
	return SafeNonNegativeFloatToInt(field, float64(quota)*ratio)
}

func SafeDivideQuotaByRatio(quota int, ratio float64, field string) (int, error) {
	if quota < 0 {
		return 0, fmt.Errorf("%s base quota cannot be negative", field)
	}
	if math.IsNaN(ratio) || math.IsInf(ratio, 0) {
		return 0, fmt.Errorf("%s ratio is not finite", field)
	}
	if ratio <= 0 {
		return 0, fmt.Errorf("%s ratio must be positive", field)
	}
	return SafeNonNegativeFloatToInt(field, float64(quota)/ratio)
}

func SafeDecimalToNonNegativeInt(field string, value decimal.Decimal) (int, error) {
	if value.IsNegative() {
		return 0, fmt.Errorf("%s cannot be negative", field)
	}
	max := decimal.NewFromInt(int64(MaxSafeInt()))
	if value.GreaterThan(max) {
		return 0, fmt.Errorf("%s exceeds integer range", field)
	}
	return int(value.IntPart()), nil
}
