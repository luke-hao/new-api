package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func stabilityTestChannel(id int, priority int64) *model.Channel {
	priorityCopy := priority
	return &model.Channel{Id: id, Priority: &priorityCopy, EffectivePriority: &priorityCopy}
}

func stabilityTestResult(channel *model.Channel, success bool, latencyMs int64) channelStabilityProbeResult {
	return channelStabilityProbeResult{channel: channel, success: success, latencyMs: latencyMs}
}

func priorityPatchesByChannel(patches []model.ChannelGroupRoutingPatch) map[int]int64 {
	result := make(map[int]int64, len(patches))
	for _, patch := range patches {
		if patch.Priority != nil {
			result[patch.ChannelId] = *patch.Priority
		}
	}
	return result
}

func TestValidateChannelGroupStabilityConfig(t *testing.T) {
	valid := channelGroupStabilityConfigRequest{
		Group:                   "claude",
		Enabled:                 true,
		IntervalMinutes:         10,
		HealthyThresholdSeconds: 8,
		ProbeTimeoutSeconds:     20,
	}
	require.NoError(t, validateChannelGroupStabilityConfig(valid))

	invalidTimeout := valid
	invalidTimeout.ProbeTimeoutSeconds = invalidTimeout.HealthyThresholdSeconds
	require.Error(t, validateChannelGroupStabilityConfig(invalidTimeout))

	invalidInterval := valid
	invalidInterval.IntervalMinutes = 0
	require.Error(t, validateChannelGroupStabilityConfig(invalidInterval))
}

func TestSelectChannelGroupStabilityPrimaryDetectsTie(t *testing.T) {
	first := stabilityTestChannel(1, 7)
	second := stabilityTestChannel(2, 7)
	primary, tied := selectChannelGroupStabilityPrimary([]*model.Channel{first, second})
	require.Nil(t, primary)
	require.True(t, tied)

	secondPriority := int64(6)
	second.EffectivePriority = &secondPriority
	primary, tied = selectChannelGroupStabilityPrimary([]*model.Channel{first, second})
	require.Equal(t, first, primary)
	require.False(t, tied)
}

func TestBuildChannelGroupStabilityPrioritiesKeepsPrimaryWithinHysteresis(t *testing.T) {
	current := stabilityTestChannel(1, 2)
	candidate := stabilityTestChannel(2, 1)
	results := map[int]channelStabilityProbeResult{
		current.Id:   stabilityTestResult(current, true, 10000),
		candidate.Id: stabilityTestResult(candidate, true, 9000),
	}

	ordered, patches := buildChannelGroupStabilityPriorities([]*model.Channel{current, candidate}, results, current.Id)
	require.Equal(t, current.Id, ordered[0].channel.Id)
	require.Empty(t, patches)
}

func TestBuildChannelGroupStabilityPrioritiesSwitchesWhenCandidateIsFifteenPercentFaster(t *testing.T) {
	current := stabilityTestChannel(1, 2)
	candidate := stabilityTestChannel(2, 1)
	results := map[int]channelStabilityProbeResult{
		current.Id:   stabilityTestResult(current, true, 10000),
		candidate.Id: stabilityTestResult(candidate, true, 8000),
	}

	ordered, patches := buildChannelGroupStabilityPriorities([]*model.Channel{current, candidate}, results, current.Id)
	require.Equal(t, candidate.Id, ordered[0].channel.Id)
	require.Equal(t, map[int]int64{1: 1, 2: 2}, priorityPatchesByChannel(patches))
}

func TestBuildChannelGroupStabilityPrioritiesIgnoresHysteresisWhenPrimaryFails(t *testing.T) {
	current := stabilityTestChannel(1, 2)
	candidate := stabilityTestChannel(2, 1)
	results := map[int]channelStabilityProbeResult{
		current.Id:   stabilityTestResult(current, false, 20000),
		candidate.Id: stabilityTestResult(candidate, true, 9500),
	}

	ordered, patches := buildChannelGroupStabilityPriorities([]*model.Channel{current, candidate}, results, current.Id)
	require.Equal(t, candidate.Id, ordered[0].channel.Id)
	require.Equal(t, map[int]int64{1: 0}, priorityPatchesByChannel(patches))
}

func TestBuildChannelGroupStabilityPrioritiesPreservesPrioritiesWhenAllFail(t *testing.T) {
	first := stabilityTestChannel(1, 2)
	second := stabilityTestChannel(2, 1)
	results := map[int]channelStabilityProbeResult{
		first.Id:  stabilityTestResult(first, false, 20000),
		second.Id: stabilityTestResult(second, false, 20000),
	}

	ordered, patches := buildChannelGroupStabilityPriorities([]*model.Channel{first, second}, results, first.Id)
	require.Nil(t, ordered)
	require.Nil(t, patches)
}
