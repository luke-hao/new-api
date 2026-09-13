package common

import (
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestVideoContractsMatchUpstreamPricing(t *testing.T) {
	data, err := os.ReadFile("testdata/aicopy_pricing_20260913.json")
	require.NoError(t, err)
	var snapshot struct {
		Data []struct {
			Model string `json:"model_name"`
			Unit  string `json:"price_unit"`
		}
	}
	require.NoError(t, Unmarshal(data, &snapshot))
	require.Len(t, snapshot.Data, 45)
	require.Len(t, videoModelContracts, 45)
	units := map[string]int{}
	for _, row := range snapshot.Data {
		contract, ok := GetVideoModelContract(row.Model)
		require.True(t, ok, row.Model)
		unit := row.Unit
		if unit == "" {
			unit = "次"
		}
		require.Equal(t, unit, contract.PriceUnit, row.Model)
		units[unit]++
	}
	require.Equal(t, map[string]int{"秒": 24, "次": 21}, units)
	require.False(t, UsesAICopyVideoProtocol("https://api.aicopy.top.attacker.example", snapshot.Data[0].Model))
	require.False(t, UsesAICopyVideoProtocol("https://api.aicopy.top", "gpt-4o"))
}
