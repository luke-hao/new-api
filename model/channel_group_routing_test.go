package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func clearChannelGroupRoutingTables(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.Exec("DELETE FROM channel_group_routings").Error)
	require.NoError(t, DB.Exec("DELETE FROM abilities").Error)
	require.NoError(t, DB.Exec("DELETE FROM channels").Error)
	t.Cleanup(func() {
		common.MemoryCacheEnabled = false
		DB.Exec("DELETE FROM channel_group_routings")
		DB.Exec("DELETE FROM abilities")
		DB.Exec("DELETE FROM channels")
	})
}

func insertRoutingTestChannel(t *testing.T, id int, priority int64, weight uint, groups string) *Channel {
	t.Helper()
	channel := &Channel{
		Id:       id,
		Type:     1,
		Key:      fmt.Sprintf("key-%d", id),
		Status:   common.ChannelStatusEnabled,
		Name:     fmt.Sprintf("channel-%d", id),
		Models:   "routing-test-model",
		Group:    groups,
		Priority: &priority,
		Weight:   &weight,
	}
	require.NoError(t, channel.Insert())
	return channel
}

func getRoutingTestAbility(t *testing.T, channelId int, group string) Ability {
	t.Helper()
	var ability Ability
	require.NoError(t, DB.Where("channel_id = ? AND "+commonGroupCol+" = ?", channelId, group).First(&ability).Error)
	return ability
}

func TestChannelGroupRoutingOverrideAndInheritance(t *testing.T) {
	clearChannelGroupRoutingTables(t)
	channel := insertRoutingTestChannel(t, 101, 1, 10, "group-a,group-b")

	overridePriority := int64(9)
	overrideWeight := uint(20)
	_, err := UpdateChannelGroupRoutings("group-a", []ChannelGroupRoutingPatch{{
		ChannelId: channel.Id,
		Priority:  &overridePriority,
		Weight:    &overrideWeight,
	}})
	require.NoError(t, err)

	groupA := getRoutingTestAbility(t, channel.Id, "group-a")
	groupB := getRoutingTestAbility(t, channel.Id, "group-b")
	require.Equal(t, int64(9), *groupA.Priority)
	require.Equal(t, uint(20), groupA.Weight)
	require.Equal(t, int64(1), *groupB.Priority)
	require.Equal(t, uint(10), groupB.Weight)

	newPriority := int64(3)
	newWeight := uint(30)
	channel.Priority = &newPriority
	channel.Weight = &newWeight
	require.NoError(t, channel.Update())

	groupA = getRoutingTestAbility(t, channel.Id, "group-a")
	groupB = getRoutingTestAbility(t, channel.Id, "group-b")
	require.Equal(t, int64(9), *groupA.Priority)
	require.Equal(t, uint(20), groupA.Weight)
	require.Equal(t, int64(3), *groupB.Priority)
	require.Equal(t, uint(30), groupB.Weight)

	_, err = UpdateChannelGroupRoutings("group-a", []ChannelGroupRoutingPatch{{
		ChannelId:       channel.Id,
		InheritPriority: true,
	}})
	require.NoError(t, err)
	groupA = getRoutingTestAbility(t, channel.Id, "group-a")
	require.Equal(t, int64(3), *groupA.Priority)
	require.Equal(t, uint(20), groupA.Weight)

	_, err = UpdateChannelGroupRoutings("group-a", []ChannelGroupRoutingPatch{{
		ChannelId:     channel.Id,
		InheritWeight: true,
	}})
	require.NoError(t, err)
	groupA = getRoutingTestAbility(t, channel.Id, "group-a")
	require.Equal(t, int64(3), *groupA.Priority)
	require.Equal(t, uint(30), groupA.Weight)
	var count int64
	require.NoError(t, DB.Model(&ChannelGroupRouting{}).Where("channel_id = ?", channel.Id).Count(&count).Error)
	require.Zero(t, count)
}

func TestChannelGroupRoutingRemovedGroupCleanup(t *testing.T) {
	clearChannelGroupRoutingTables(t)
	channel := insertRoutingTestChannel(t, 102, 1, 10, "group-a,group-b")
	override := int64(8)
	_, err := UpdateChannelGroupRoutings("group-a", []ChannelGroupRoutingPatch{{ChannelId: channel.Id, Priority: &override}})
	require.NoError(t, err)

	channel.Group = "group-b"
	require.NoError(t, channel.Update())
	var count int64
	require.NoError(t, DB.Model(&ChannelGroupRouting{}).Where("channel_id = ?", channel.Id).Count(&count).Error)
	require.Zero(t, count)
}

func TestChannelGroupRoutingDatabaseAndCacheSelection(t *testing.T) {
	clearChannelGroupRoutingTables(t)
	channel1 := insertRoutingTestChannel(t, 103, 10, 10, "group-a,group-b")
	channel2 := insertRoutingTestChannel(t, 104, 5, 10, "group-a,group-b")

	channel1GroupAPriority := int64(1)
	channel2GroupAPriority := int64(20)
	_, err := UpdateChannelGroupRoutings("group-a", []ChannelGroupRoutingPatch{
		{ChannelId: channel1.Id, Priority: &channel1GroupAPriority},
		{ChannelId: channel2.Id, Priority: &channel2GroupAPriority},
	})
	require.NoError(t, err)

	common.MemoryCacheEnabled = false
	selected, err := GetRandomSatisfiedChannel("group-a", "routing-test-model", 0)
	require.NoError(t, err)
	require.Equal(t, channel2.Id, selected.Id)
	selected, err = GetRandomSatisfiedChannel("group-b", "routing-test-model", 0)
	require.NoError(t, err)
	require.Equal(t, channel1.Id, selected.Id)

	common.MemoryCacheEnabled = true
	InitChannelCache()
	selected, err = GetRandomSatisfiedChannel("group-a", "routing-test-model", 0)
	require.NoError(t, err)
	require.Equal(t, channel2.Id, selected.Id)
	selected, err = GetRandomSatisfiedChannel("group-b", "routing-test-model", 0)
	require.NoError(t, err)
	require.Equal(t, channel1.Id, selected.Id)
}

func TestChannelGroupRoutingEffectiveProjectionAndSort(t *testing.T) {
	clearChannelGroupRoutingTables(t)
	channel1 := insertRoutingTestChannel(t, 105, 10, 10, "group-a")
	channel2 := insertRoutingTestChannel(t, 106, 5, 10, "group-a")
	override := int64(20)
	_, err := UpdateChannelGroupRoutings("group-a", []ChannelGroupRoutingPatch{{
		ChannelId: channel2.Id,
		Priority:  &override,
	}})
	require.NoError(t, err)

	var channels []*Channel
	options := NewChannelSortOptions("priority", "desc", false)
	query := ApplyChannelGroupFilter(DB.Model(&Channel{}), "group-a")
	require.NoError(t, options.ApplyForGroup(query, "group-a").Find(&channels).Error)
	require.Len(t, channels, 2)
	require.Equal(t, channel2.Id, channels[0].Id)
	require.Equal(t, channel1.Id, channels[1].Id)

	require.NoError(t, PopulateEffectiveChannelRoutings(channels, "group-a"))
	require.Equal(t, int64(20), *channels[0].EffectivePriority)
	require.True(t, channels[0].PriorityOverridden)
	require.Equal(t, int64(10), *channels[1].EffectivePriority)
	require.False(t, channels[1].PriorityOverridden)
}
