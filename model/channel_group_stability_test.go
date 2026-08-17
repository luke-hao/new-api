package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func clearChannelGroupStabilityPolicies(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.Exec("DELETE FROM channel_group_stability_policies").Error)
	t.Cleanup(func() {
		DB.Exec("DELETE FROM channel_group_stability_policies")
	})
}

func TestChannelGroupStabilityPolicyDefaultsAndSchedule(t *testing.T) {
	clearChannelGroupStabilityPolicies(t)

	policy, err := GetOrCreateChannelGroupStabilityPolicy("claude")
	require.NoError(t, err)
	require.False(t, policy.Enabled)
	require.Equal(t, DefaultChannelGroupStabilityIntervalMinutes, policy.IntervalMinutes)
	require.Equal(t, DefaultChannelGroupStabilityHealthySeconds, policy.HealthyThresholdSeconds)
	require.Equal(t, DefaultChannelGroupStabilityTimeoutSeconds, policy.ProbeTimeoutSeconds)

	now := int64(123456789)
	saved, err := SaveChannelGroupStabilityConfig(ChannelGroupStabilityConfig{
		Group:                   "claude",
		Enabled:                 true,
		IntervalMinutes:         10,
		HealthyThresholdSeconds: 8,
		ProbeTimeoutSeconds:     20,
	}, now)
	require.NoError(t, err)
	require.True(t, saved.Enabled)
	require.Equal(t, now, saved.NextCheckAt)

	due, err := ListDueChannelGroupStabilityPolicies(now)
	require.NoError(t, err)
	require.Len(t, due, 1)
	require.Equal(t, "claude", due[0].Group)
	require.Equal(t, now+600000, ChannelGroupStabilityNextCheckAt(*saved, now))
}

func TestApplyChannelGroupStabilityPrioritiesRejectsStaleConfig(t *testing.T) {
	clearChannelGroupRoutingTables(t)
	clearChannelGroupStabilityPolicies(t)
	channel := insertRoutingTestChannel(t, 7001, 1, 0, "claude")

	policy, err := SaveChannelGroupStabilityConfig(ChannelGroupStabilityConfig{
		Group:                   "claude",
		Enabled:                 true,
		IntervalMinutes:         10,
		HealthyThresholdSeconds: 8,
		ProbeTimeoutSeconds:     20,
	}, 1)
	require.NoError(t, err)

	_, err = SaveChannelGroupStabilityConfig(ChannelGroupStabilityConfig{
		Group:                   "claude",
		Enabled:                 true,
		IntervalMinutes:         15,
		HealthyThresholdSeconds: 8,
		ProbeTimeoutSeconds:     20,
	}, 2)
	require.NoError(t, err)

	priority := int64(2)
	_, err = ApplyChannelGroupStabilityPriorities(*policy, true, []ChannelGroupRoutingPatch{{
		ChannelId: channel.Id,
		Priority:  &priority,
	}})
	require.ErrorIs(t, err, ErrChannelGroupStabilityPolicyStale)
}
