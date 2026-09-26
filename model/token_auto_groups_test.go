package model

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestTokenAutoGroupsJSONContract(t *testing.T) {
	var token Token
	require.NoError(t, common.Unmarshal([]byte("{\"auto_groups\":[\"second\",\"first\"]}"), &token))
	require.Equal(t, []string{"second", "first"}, token.AutoGroups.Groups())
	data, err := common.Marshal(token)
	require.NoError(t, err)
	require.Contains(t, string(data), "\"auto_groups\":[\"second\",\"first\"]")
	for _, input := range []string{"null", "{}", "\"a\"", "[1]"} {
		var value TokenAutoGroups
		require.Error(t, common.Unmarshal([]byte(input), &value))
	}
}
