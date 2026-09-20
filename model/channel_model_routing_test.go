package model

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func clearModelRouting(t *testing.T) {
	t.Helper()
	clearChannelGroupRoutingTables(t)
	clearChannelGroupStabilityPolicies(t)
	require.NoError(t, DB.Exec("DELETE FROM channel_model_routings").Error)
	require.NoError(t, DB.Exec("DELETE FROM channel_model_stability_policies").Error)
	t.Cleanup(func() {
		DB.Exec("DELETE FROM channel_model_routings")
		DB.Exec("DELETE FROM channel_model_stability_policies")
	})
}
func modelInt(v int) *int          { return &v }
func modelPriority(v int64) *int64 { return &v }

func TestModelRoutingIsolationRebuildAndCache(t *testing.T) {
	clearModelRouting(t)
	a := insertRoutingTestChannel(t, 9501, 7, 0, "model-a,model-b")
	b := insertRoutingTestChannel(t, 9502, 6, 0, "model-a,model-b")
	for _, c := range []*Channel{a, b} {
		c.Models = "sol,astra"
		require.NoError(t, c.Update())
	}
	_, err := UpdateChannelModelRoutings("model-a", "sol", []ChannelGroupRoutingPatch{{ChannelId: a.Id, Priority: modelPriority(2)}, {ChannelId: b.Id, Priority: modelPriority(1)}}, "")
	require.NoError(t, err)
	_, err = UpdateChannelModelRoutings("model-a", "astra", []ChannelGroupRoutingPatch{{ChannelId: a.Id, Priority: modelPriority(1)}, {ChannelId: b.Id, Priority: modelPriority(2)}}, "")
	require.NoError(t, err)
	for _, cache := range []bool{false, true} {
		common.MemoryCacheEnabled = cache
		InitChannelCache()
		selected, err := GetRandomSatisfiedChannel("model-a", "sol", 0)
		require.NoError(t, err)
		require.Equal(t, a.Id, selected.Id)
		selected, err = GetRandomSatisfiedChannel("model-a", "astra", 0)
		require.NoError(t, err)
		require.Equal(t, b.Id, selected.Id)
		selected, err = GetRandomSatisfiedChannel("model-b", "astra", 0)
		require.NoError(t, err)
		require.Equal(t, a.Id, selected.Id)
		selected, err = GetChannelExcluding("model-a", "astra", map[int]bool{b.Id: true})
		require.NoError(t, err)
		require.Equal(t, a.Id, selected.Id)
	}
	a.Priority = modelPriority(30)
	require.NoError(t, a.Update())
	_, _, err = FixAbility()
	require.NoError(t, err)
	for _, name := range []string{"sol", "astra"} {
		require.NoError(t, PopulateEffectiveModelRoutings([]*Channel{a, b}, "model-a", name))
		if name == "sol" {
			require.Equal(t, int64(2), *a.EffectivePriority)
		} else {
			require.Equal(t, int64(1), *a.EffectivePriority)
		}
	}
	_, err = UpdateChannelGroupRoutings("model-a", []ChannelGroupRoutingPatch{{ChannelId: a.Id, Priority: modelPriority(40)}})
	require.NoError(t, err)
	require.NoError(t, PopulateEffectiveModelRoutings([]*Channel{a}, "model-a", "astra"))
	require.Equal(t, int64(1), *a.EffectivePriority)
	a.Models = "sol"
	require.NoError(t, a.Update())
	var count int64
	require.NoError(t, DB.Model(&ChannelModelRouting{}).Where("channel_id = ? AND model = ?", a.Id, "astra").Count(&count).Error)
	require.Zero(t, count)
}

func TestModelStabilityInheritanceAndVersionFence(t *testing.T) {
	clearModelRouting(t)
	a := insertRoutingTestChannel(t, 9503, 7, 0, "model-a")
	a.Models = "sol"
	require.NoError(t, a.Update())
	parent, err := SaveChannelGroupStabilityConfig(ChannelGroupStabilityConfig{Group: "model-a", Enabled: true, IntervalMinutes: 5, HealthyThresholdSeconds: 8, ProbeTimeoutSeconds: 10}, time.Now().UnixMilli())
	require.NoError(t, err)
	child, err := GetChannelModelPolicy("model-a", "sol")
	require.NoError(t, err)
	effective := ResolveChannelModelPolicy(*parent, *child)
	require.Equal(t, 5, effective.IntervalMinutes)
	require.True(t, effective.Enabled)
	require.False(t, effective.Initialized)
	_, snapshot, err := GetChannelModelStabilityCandidates("model-a", "sol")
	require.NoError(t, err)
	require.NoError(t, SaveChannelModelPolicy(ChannelModelStabilityPolicy{Group: "model-a", Model: "sol", IntervalMinutes: modelInt(2), Paused: true}))
	_, err = CommitChannelModelStabilityRun(effective, true, snapshot, nil, []ChannelGroupRoutingPatch{{ChannelId: a.Id, Priority: modelPriority(1)}}, ChannelGroupStabilityRunUpdate{}, true)
	require.ErrorIs(t, err, ErrChannelGroupStabilityPolicyStale)
	child, err = GetChannelModelPolicy("model-a", "sol")
	require.NoError(t, err)
	effective = ResolveChannelModelPolicy(*parent, *child)
	require.Equal(t, 2, effective.IntervalMinutes)
	require.False(t, effective.Enabled)
	require.Error(t, SaveChannelModelPolicy(ChannelModelStabilityPolicy{Group: "model-a", Model: "sol", ProbeTimeoutSeconds: modelInt(7)}))
	require.NoError(t, SaveChannelModelPolicy(ChannelModelStabilityPolicy{Group: "model-a", Model: "sol", HealthyThresholdSeconds: modelInt(9)}))
	_, err = SaveChannelGroupStabilityConfig(ChannelGroupStabilityConfig{Group: "model-a", Enabled: true, IntervalMinutes: 5, HealthyThresholdSeconds: 4, ProbeTimeoutSeconds: 8}, time.Now().UnixMilli())
	require.Error(t, err)
}

func TestModelRoutingLocksSortAndCommit(t *testing.T) {
	clearModelRouting(t)
	a := insertRoutingTestChannel(t, 9504, 7, 0, "model-a")
	b := insertRoutingTestChannel(t, 9505, 6, 0, "model-a")
	for _, c := range []*Channel{a, b} {
		c.Models = "sol,astra"
		require.NoError(t, c.Update())
	}
	lock := true
	_, err := UpdateChannelModelRoutings("model-a", "sol", []ChannelGroupRoutingPatch{{ChannelId: a.Id, PriorityLocked: &lock}}, "")
	require.NoError(t, err)
	result, err := UpdateChannelModelRoutings("model-a", "sol", []ChannelGroupRoutingPatch{{ChannelId: a.Id, Priority: modelPriority(0)}, {ChannelId: b.Id, Priority: modelPriority(9)}}, "rerank")
	require.NoError(t, err)
	require.Equal(t, 1, result.SkippedLocked)
	require.NoError(t, PopulateEffectiveModelRoutings([]*Channel{a}, "model-a", "astra"))
	require.False(t, a.PriorityLocked)
	var sorted []*Channel
	require.NoError(t, NewChannelSortOptions("priority", "desc", false).ApplyForModel(DB.Model(&Channel{}), "model-a", "sol").Limit(1).Find(&sorted).Error)
	require.Equal(t, b.Id, sorted[0].Id)
	parent, err := GetOrCreateChannelGroupStabilityPolicy("model-a")
	require.NoError(t, err)
	child, err := GetChannelModelPolicy("model-a", "sol")
	require.NoError(t, err)
	p := ResolveChannelModelPolicy(*parent, *child)
	_, snapshot, err := GetChannelModelStabilityCandidates(p.Group, p.Model)
	require.NoError(t, err)
	obs := []ChannelModelRouting{{ChannelId: b.Id, LatencyMs: 900, Result: "success", LastCheckAt: 1}}
	_, err = CommitChannelModelStabilityRun(p, false, snapshot, obs, []ChannelGroupRoutingPatch{{ChannelId: b.Id, Priority: modelPriority(8)}}, ChannelGroupStabilityRunUpdate{LastResult: "reranked", LastCheckAt: 1, LastPrimaryChannelId: b.Id}, true)
	require.NoError(t, err)
	require.NoError(t, PopulateEffectiveModelRoutings([]*Channel{b}, p.Group, p.Model))
	require.Equal(t, int64(900), b.ModelResponseTime)
	child, err = GetChannelModelPolicy(p.Group, p.Model)
	require.NoError(t, err)
	require.True(t, child.Initialized)
	require.NoError(t, DB.Model(&Channel{}).Where("id = ?", b.Id).Update("models", "astra").Error)
	_, err = CommitChannelModelStabilityRun(p, false, snapshot, nil, nil, ChannelGroupStabilityRunUpdate{}, true)
	require.ErrorIs(t, err, ErrChannelGroupStabilityPolicyStale)
}
