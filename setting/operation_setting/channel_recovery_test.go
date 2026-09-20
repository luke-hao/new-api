package operation_setting

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestChannelRecoveryPolicyValidation(t *testing.T) {
	policy := DefaultChannelRecoveryPolicy()
	data, err := common.Marshal(policy)
	require.NoError(t, err)
	parsed, err := ParseChannelRecoveryPolicy(string(data))
	require.NoError(t, err)
	require.Equal(t, policy, parsed)
	for _, raw := range []string{`{}`, `{"enabled":true,"rules":{}}`, `invalid`} {
		_, err := ParseChannelRecoveryPolicy(raw)
		require.Error(t, err)
	}
	for _, minutes := range []int{0, 10081} {
		rule := policy.Rules["balance"]
		rule.IntervalMinutes = minutes
		policy.Rules["balance"] = rule
		data, _ = common.Marshal(policy)
		_, err = ParseChannelRecoveryPolicy(string(data))
		require.Error(t, err)
	}
}
