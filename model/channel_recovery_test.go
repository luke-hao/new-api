package model

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestChannelRecoveryRejectsStaleResults(t *testing.T) {
	clearChannelGroupRoutingTables(t)
	for i, mutation := range []string{"manual", "key", "models", "reason", "deleted"} {
		t.Run(mutation, func(t *testing.T) {
			channel := insertRoutingTestChannel(t, 910+i, 5, 0, "default")
			require.NoError(t, DB.Model(channel).Updates(map[string]interface{}{"status": common.ChannelStatusAutoDisabled, "other_info": `{"status_reason":"Insufficient account balance","status_time":1}`}).Error)
			snapshot, err := GetChannelById(channel.Id, true)
			require.NoError(t, err)
			switch mutation {
			case "manual":
				DB.Model(channel).Update("status", common.ChannelStatusManuallyDisabled)
			case "key":
				DB.Model(channel).Update("key", "replaced")
			case "models":
				DB.Model(channel).Update("models", "changed-model")
			case "reason":
				DB.Model(channel).Update("other_info", `{"status_time":2}`)
			case "deleted":
				DB.Delete(channel)
			}
			_, applied, err := SaveChannelRecovery(snapshot, ChannelRecoveryState{Reason: "balance"}, &snapshot.Key)
			if mutation == "deleted" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.False(t, applied)
		})
	}
}
func TestChannelRecoveryUpdatesAbilitiesAndCache(t *testing.T) {
	clearChannelGroupRoutingTables(t)
	channel := insertRoutingTestChannel(t, 920, 5, 0, "default")
	require.NoError(t, DB.Model(channel).Updates(map[string]interface{}{"status": common.ChannelStatusAutoDisabled, "other_info": `{"status_time":1}`}).Error)
	require.NoError(t, UpdateAbilityStatus(channel.Id, false))
	common.MemoryCacheEnabled = true
	InitChannelCache()
	snapshot, err := GetChannelById(channel.Id, true)
	require.NoError(t, err)
	claim, applied, err := SaveChannelRecovery(snapshot, ChannelRecoveryState{Reason: "balance", Attempts: 1}, nil)
	require.NoError(t, err)
	require.True(t, applied)
	_, applied, err = SaveChannelRecovery(snapshot, ChannelRecoveryState{}, &snapshot.Key)
	require.NoError(t, err)
	require.False(t, applied)
	_, applied, err = SaveChannelRecovery(claim, ChannelRecoveryState{Reason: "balance", Successes: 1}, &claim.Key)
	require.NoError(t, err)
	require.True(t, applied)
	ability := getRoutingTestAbility(t, channel.Id, "default")
	require.True(t, ability.Enabled)
	cached, err := CacheGetChannel(channel.Id)
	require.NoError(t, err)
	require.Equal(t, common.ChannelStatusEnabled, cached.Status)
}

func TestLateAutomaticDisablePreservesManualDisable(t *testing.T) {
	clearChannelGroupRoutingTables(t)
	channel := insertRoutingTestChannel(t, 940, 5, 0, "default")
	require.NoError(t, DB.Model(channel).Update("status", common.ChannelStatusManuallyDisabled).Error)
	for _, cached := range []bool{false, true} {
		common.MemoryCacheEnabled = cached
		InitChannelCache()
		require.False(t, UpdateChannelStatus(channel.Id, channel.Key, common.ChannelStatusAutoDisabled, "late balance failure", "balance"))
		current, err := GetChannelById(channel.Id, true)
		require.NoError(t, err)
		require.Equal(t, common.ChannelStatusManuallyDisabled, current.Status)
	}
}

func TestChannelRecoveryRestoresOnlySuccessfulAutoDisabledKey(t *testing.T) {
	clearChannelGroupRoutingTables(t)
	channel := insertRoutingTestChannel(t, 950, 5, 0, "default")
	channel.Key = "manual-key\nauto-key"
	channel.Status = common.ChannelStatusAutoDisabled
	channel.ChannelInfo = ChannelInfo{IsMultiKey: true, MultiKeySize: 2, MultiKeyStatusList: map[int]int{0: common.ChannelStatusManuallyDisabled, 1: common.ChannelStatusAutoDisabled}}
	require.NoError(t, DB.Model(channel).Select("key", "status", "channel_info").Updates(channel).Error)
	snapshot, err := GetChannelById(channel.Id, true)
	require.NoError(t, err)
	manual := "manual-key"
	_, applied, err := SaveChannelRecovery(snapshot, ChannelRecoveryState{}, &manual)
	require.NoError(t, err)
	require.False(t, applied)
	auto := "auto-key"
	current, applied, err := SaveChannelRecovery(snapshot, ChannelRecoveryState{Successes: 2}, &auto)
	require.NoError(t, err)
	require.True(t, applied)
	require.Equal(t, common.ChannelStatusEnabled, current.Status)
	require.Equal(t, common.ChannelStatusManuallyDisabled, current.ChannelInfo.MultiKeyStatusList[0])
	_, exists := current.ChannelInfo.MultiKeyStatusList[1]
	require.False(t, exists)
}
