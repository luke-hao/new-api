package controller

import (
	"encoding/json"
	"errors"
	"math"
	"testing"
)

func TestGenericBalancePathsIncludeAppUsageEndpoints(t *testing.T) {
	want := map[string]bool{"/v1/usage": false, "/v1/sub2api/billing": false}
	for _, path := range genericBalancePaths {
		if _, ok := want[path]; ok {
			want[path] = true
		}
	}
	for path, found := range want {
		if !found {
			t.Fatalf("generic balance paths missing %s", path)
		}
	}
}

func TestFindGenericBalanceAppUsagePayloads(t *testing.T) {
	cases := []struct {
		name string
		body string
		want float64
	}{
		{name: "finite", body: `{"balance":123.45,"daily_usage":[]}`, want: 123.45},
		{name: "nested", body: `{"data":{"result":{"available_balance":"9,876.5"}}}`, want: 9876.5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var payload any
			if err := json.Unmarshal([]byte(tc.body), &payload); err != nil {
				t.Fatal(err)
			}
			got, ok := findGenericBalance(payload, 0)
			if !ok || math.Abs(got-tc.want) > 1e-9 {
				t.Fatalf("findGenericBalance() = %v, %v; want %v", got, ok, tc.want)
			}
		})
	}
}

func TestValidateAutomaticBalanceRejectsUnlimitedSentinel(t *testing.T) {
	if err := validateAutomaticBalance(999993106.6213171); !errors.Is(err, errBalanceOutOfRange) {
		t.Fatalf("validateAutomaticBalance() error = %v, want %v", err, errBalanceOutOfRange)
	}
	if err := validateAutomaticBalance(42.5); err != nil {
		t.Fatalf("validateAutomaticBalance(finite) = %v", err)
	}
}

