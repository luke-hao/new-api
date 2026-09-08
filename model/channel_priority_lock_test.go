package model

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
	"time"
)

func priorityLockBool(v bool) *bool  { return &v }
func priorityLockInt(v int64) *int64 { return &v }
func preparePriorityLockTest(t *testing.T) {
	clearChannelGroupRoutingTables(t)
	clearChannelGroupStabilityPolicies(t)
}

func TestPriorityLockSnapshotsEffectiveValueAndIsolatesGroups(t *testing.T) {
	preparePriorityLockTest(t)
	channel := insertRoutingTestChannel(t, 8101, 7, 0, "lock-a,lock-b")
	result, err := UpdateChannelGroupRoutingsWithMode("lock-a", []ChannelGroupRoutingPatch{{ChannelId: channel.Id, PriorityLocked: priorityLockBool(true)}}, "")
	require.NoError(t, err)
	require.True(t, result.LockChanged)
	channel.Priority = priorityLockInt(20)
	require.NoError(t, channel.Update())
	require.Equal(t, int64(7), *getRoutingTestAbility(t, channel.Id, "lock-a").Priority)
	require.Equal(t, int64(20), *getRoutingTestAbility(t, channel.Id, "lock-b").Priority)
	require.NoError(t, PopulateEffectiveChannelRoutings([]*Channel{channel}, "lock-a"))
	require.True(t, channel.PriorityLocked)
	require.Equal(t, int64(7), *channel.EffectivePriority)
	require.NoError(t, PopulateEffectiveChannelRoutings([]*Channel{channel}, "lock-b"))
	require.False(t, channel.PriorityLocked)
}
func TestPriorityLockManualEditingUnlockAndRestore(t *testing.T) {
	preparePriorityLockTest(t)
	channel := insertRoutingTestChannel(t, 8102, 7, 0, "lock-a")
	for _, priority := range []int64{0, -5, 12} {
		_, err := UpdateChannelGroupRoutings("lock-a", []ChannelGroupRoutingPatch{{ChannelId: channel.Id, Priority: &priority, PriorityLocked: priorityLockBool(true)}})
		require.NoError(t, err)
		require.Equal(t, priority, *getRoutingTestAbility(t, channel.Id, "lock-a").Priority)
	}
	_, err := UpdateChannelGroupRoutings("lock-a", []ChannelGroupRoutingPatch{{ChannelId: channel.Id, InheritPriority: true}})
	require.Error(t, err)
	_, err = UpdateChannelGroupRoutings("lock-a", []ChannelGroupRoutingPatch{{ChannelId: channel.Id, InheritWeight: true}})
	require.NoError(t, err)
	_, err = UpdateChannelGroupRoutings("lock-a", []ChannelGroupRoutingPatch{{ChannelId: channel.Id, PriorityLocked: priorityLockBool(false)}})
	require.NoError(t, err)
	require.Equal(t, int64(12), *getRoutingTestAbility(t, channel.Id, "lock-a").Priority)
	_, err = UpdateChannelGroupRoutings("lock-a", []ChannelGroupRoutingPatch{{ChannelId: channel.Id, InheritPriority: true}})
	require.NoError(t, err)
	require.Equal(t, int64(7), *getRoutingTestAbility(t, channel.Id, "lock-a").Priority)
	var count int64
	require.NoError(t, DB.Model(&ChannelGroupRouting{}).Where("channel_id = ?", channel.Id).Count(&count).Error)
	require.Zero(t, count)
}
func TestPriorityLockRerankSkipsLatestLocksAndKeepsOtherGroupEligible(t *testing.T) {
	preparePriorityLockTest(t)
	a := insertRoutingTestChannel(t, 8103, 7, 0, "lock-a,lock-b")
	b := insertRoutingTestChannel(t, 8104, 1, 0, "lock-a")
	_, err := UpdateChannelGroupRoutings("lock-a", []ChannelGroupRoutingPatch{{ChannelId: a.Id, PriorityLocked: priorityLockBool(true)}})
	require.NoError(t, err)
	patches := []ChannelGroupRoutingPatch{{ChannelId: a.Id, Priority: priorityLockInt(0)}, {ChannelId: b.Id, Priority: priorityLockInt(2)}}
	result, err := UpdateChannelGroupRoutingsWithMode("lock-a", patches, ChannelGroupRoutingRerank)
	require.NoError(t, err)
	require.Equal(t, 1, result.SkippedLocked)
	require.Equal(t, 1, result.Updated)
	require.Equal(t, int64(7), *getRoutingTestAbility(t, a.Id, "lock-a").Priority)
	require.Equal(t, int64(2), *getRoutingTestAbility(t, b.Id, "lock-a").Priority)
	result, err = UpdateChannelGroupRoutingsWithMode("lock-b", patches[:1], ChannelGroupRoutingRerank)
	require.NoError(t, err)
	require.Equal(t, 1, result.Updated)
	require.Zero(t, result.SkippedLocked)
}
func TestPriorityLockRejectsInflightDetectionAndPreservesSchedule(t *testing.T) {
	preparePriorityLockTest(t)
	a := insertRoutingTestChannel(t, 8105, 7, 0, "lock-a")
	policy, err := SaveChannelGroupStabilityConfig(ChannelGroupStabilityConfig{Group: "lock-a", Enabled: true, IntervalMinutes: 5, HealthyThresholdSeconds: 8, ProbeTimeoutSeconds: 10}, 1)
	require.NoError(t, err)
	before := time.Now().UnixMilli()
	_, err = UpdateChannelGroupRoutings("lock-a", []ChannelGroupRoutingPatch{{ChannelId: a.Id, PriorityLocked: priorityLockBool(true)}})
	require.NoError(t, err)
	for _, automatic := range []bool{false, true} {
		_, err = ApplyChannelGroupStabilityPriorities(*policy, automatic, []ChannelGroupRoutingPatch{{ChannelId: a.Id, Priority: priorityLockInt(0)}})
		require.ErrorIs(t, err, ErrChannelGroupStabilityPolicyStale)
	}
	current, err := GetOrCreateChannelGroupStabilityPolicy("lock-a")
	require.NoError(t, err)
	require.Greater(t, current.ConfigVersion, policy.ConfigVersion)
	require.GreaterOrEqual(t, current.NextCheckAt, before+5*60*1000)
	updated, err := ApplyChannelGroupStabilityPriorities(*current, true, []ChannelGroupRoutingPatch{{ChannelId: a.Id, Priority: priorityLockInt(0)}})
	require.NoError(t, err)
	require.Zero(t, updated)
	require.Equal(t, int64(7), *getRoutingTestAbility(t, a.Id, "lock-a").Priority)
}
func TestPriorityLockDoesNotRemoveChannelFromRouting(t *testing.T) {
	preparePriorityLockTest(t)
	a := insertRoutingTestChannel(t, 8106, 7, 0, "lock-a")
	insertRoutingTestChannel(t, 8107, 1, 0, "lock-a")
	_, err := UpdateChannelGroupRoutings("lock-a", []ChannelGroupRoutingPatch{{ChannelId: a.Id, PriorityLocked: priorityLockBool(true)}})
	require.NoError(t, err)
	for _, cache := range []bool{false, true} {
		common.MemoryCacheEnabled = cache
		if cache {
			InitChannelCache()
		}
		selected, err := GetRandomSatisfiedChannel("lock-a", "routing-test-model", 0)
		require.NoError(t, err)
		require.Equal(t, a.Id, selected.Id)
	}
}
func TestPriorityLockValidationAndAtomicFailure(t *testing.T) {
	preparePriorityLockTest(t)
	a := insertRoutingTestChannel(t, 8108, 7, 0, "lock-a")
	for _, mode := range []string{"invalid", ChannelGroupRoutingRerank} {
		_, err := UpdateChannelGroupRoutingsWithMode("lock-a", []ChannelGroupRoutingPatch{{ChannelId: a.Id, PriorityLocked: priorityLockBool(true)}}, mode)
		require.Error(t, err)
	}
	_, err := UpdateChannelGroupRoutings("lock-a", []ChannelGroupRoutingPatch{{ChannelId: a.Id, PriorityLocked: priorityLockBool(true), InheritPriority: true}})
	require.Error(t, err)
	require.NoError(t, PopulateEffectiveChannelRoutings([]*Channel{a}, "lock-a"))
	require.False(t, a.PriorityLocked)
}
func TestPriorityLockConcurrentRerankingNeverOverwritesFixedValue(t *testing.T) {
	preparePriorityLockTest(t)
	a := insertRoutingTestChannel(t, 8109, 1, 0, "lock-a")
	var wg sync.WaitGroup
	errs := make(chan error, 21)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := UpdateChannelGroupRoutingsWithMode("lock-a", []ChannelGroupRoutingPatch{{ChannelId: a.Id, Priority: priorityLockInt(2)}}, ChannelGroupRoutingRerank)
			errs <- err
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := UpdateChannelGroupRoutings("lock-a", []ChannelGroupRoutingPatch{{ChannelId: a.Id, Priority: priorityLockInt(7), PriorityLocked: priorityLockBool(true)}})
		errs <- err
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, int64(7), *getRoutingTestAbility(t, a.Id, "lock-a").Priority)
}
func TestPriorityLockCopyAndRemovedGroupCleanup(t *testing.T) {
	preparePriorityLockTest(t)
	a := insertRoutingTestChannel(t, 8110, 7, 0, "lock-a,lock-b")
	b := insertRoutingTestChannel(t, 8111, 1, 0, "lock-a,lock-b")
	_, err := UpdateChannelGroupRoutings("lock-a", []ChannelGroupRoutingPatch{{ChannelId: a.Id, PriorityLocked: priorityLockBool(true)}})
	require.NoError(t, err)
	require.NoError(t, CopyChannelGroupRoutings(a.Id, b.Id))
	require.NoError(t, PopulateEffectiveChannelRoutings([]*Channel{b}, "lock-a"))
	require.True(t, b.PriorityLocked)
	a.Group = "lock-b"
	require.NoError(t, a.Update())
	var count int64
	require.NoError(t, DB.Model(&ChannelGroupRouting{}).Where("channel_id = ?", a.Id).Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, DeleteChannelGroupRoutings(DB, []int{b.Id}))
	require.NoError(t, DB.Model(&ChannelGroupRouting{}).Where("channel_id = ?", b.Id).Count(&count).Error)
	require.Zero(t, count)
}
