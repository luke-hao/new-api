package controller

import (
	"context"
	"errors"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestChannelRecoverySchedule(t *testing.T) {
	policy := operation_setting.DefaultChannelRecoveryPolicy()
	policy.Enabled = true
	channel := &model.Channel{Status: common.ChannelStatusAutoDisabled, OtherInfo: `{"status_reason":"status code: 403, Insufficient account balance","status_time":100}`}
	state, due := channelRecoveryDue(channel, policy, 1899)
	require.Equal(t, "balance", state.Reason)
	require.False(t, due)
	_, due = channelRecoveryDue(channel, policy, 1900)
	require.True(t, due)
	channel.Status = common.ChannelStatusManuallyDisabled
	_, due = channelRecoveryDue(channel, policy, 9999)
	require.False(t, due)
	channel.Status = common.ChannelStatusAutoDisabled
	policy.Enabled = false
	_, due = channelRecoveryDue(channel, policy, 9999)
	require.False(t, due)
}
func TestChannelRecoveryProbeUsesOnlyAutoDisabledKey(t *testing.T) {
	channel := &model.Channel{Key: "manual\nauto", ChannelInfo: model.ChannelInfo{IsMultiKey: true, MultiKeyStatusList: map[int]int{0: 2, 1: 3}}}
	probe, ok := recoveryProbeChannel(channel, 0)
	require.True(t, ok)
	require.Equal(t, "auto", probe.Key)
	require.False(t, probe.ChannelInfo.IsMultiKey)
	require.Equal(t, 3, channel.ChannelInfo.MultiKeyStatusList[1])
}
func TestChannelRecoveryLocalFailureAndConsecutiveSuccess(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Ability{}))
	oldProbe := channelRecoveryProbe
	t.Cleanup(func() { channelRecoveryProbe = oldProbe })
	common.OptionMapRWMutex.Lock()
	previous := common.OptionMap
	common.OptionMap = map[string]string{}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() { common.OptionMapRWMutex.Lock(); common.OptionMap = previous; common.OptionMapRWMutex.Unlock() })
	policy := operation_setting.DefaultChannelRecoveryPolicy()
	policy.Enabled = true
	rule := policy.Rules["balance"]
	rule.SuccessesRequired = 2
	policy.Rules["balance"] = rule
	raw, _ := common.Marshal(policy)
	common.OptionMapRWMutex.Lock()
	common.OptionMap[operation_setting.ChannelRecoveryPolicyOption] = string(raw)
	common.OptionMapRWMutex.Unlock()
	channel := &model.Channel{Id: 931, Key: "fixture", Status: 3, Models: "test", Group: "default", OtherInfo: `{"status_reason":"Insufficient account balance","status_time":1}`}
	require.NoError(t, db.Create(channel).Error)
	channelRecoveryProbe = func(context.Context, *model.Channel, int, string, string, bool) testResult {
		return testResult{localErr: errors.New("unsupported test")}
	}
	runChannelRecovery(channel, policy, 1)
	current, err := model.GetChannelById(931, true)
	require.NoError(t, err)
	require.Equal(t, 3, current.Status)
	require.Zero(t, model.GetChannelRecoveryState(current).Successes)
	channelRecoveryProbe = func(context.Context, *model.Channel, int, string, string, bool) testResult { return testResult{} }
	for i := 1; i <= 2; i++ {
		state := model.GetChannelRecoveryState(current)
		state.LastTestAt = time.Now().Unix() - 4000
		info := current.GetOtherInfo()
		info["auto_recovery"] = state
		current.SetOtherInfo(info)
		require.NoError(t, db.Model(current).Update("other_info", current.OtherInfo).Error)
		runChannelRecovery(current, policy, 1)
		current, err = model.GetChannelById(931, true)
		require.NoError(t, err)
		require.Equal(t, i, model.GetChannelRecoveryState(current).Successes)
		if i == 1 {
			require.Equal(t, 3, current.Status)
		} else {
			require.Equal(t, 1, current.Status)
		}
	}
}

func TestChannelRecoveryDiscardsConfigurationChange(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Ability{}))
	oldProbe := channelRecoveryProbe
	common.OptionMapRWMutex.Lock()
	previous := common.OptionMap
	common.OptionMap = map[string]string{}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		channelRecoveryProbe = oldProbe
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previous
		common.OptionMapRWMutex.Unlock()
	})
	policy := operation_setting.DefaultChannelRecoveryPolicy()
	policy.Enabled = true
	raw, _ := common.Marshal(policy)
	common.OptionMapRWMutex.Lock()
	common.OptionMap[operation_setting.ChannelRecoveryPolicyOption] = string(raw)
	common.OptionMapRWMutex.Unlock()
	channel := &model.Channel{Id: 932, Key: "fixture", Status: 3, Models: "test", Group: "default", OtherInfo: `{"status_reason":"Insufficient account balance","status_time":1}`}
	require.NoError(t, db.Create(channel).Error)
	channelRecoveryProbe = func(context.Context, *model.Channel, int, string, string, bool) testResult {
		disabled := policy
		disabled.Enabled = false
		raw, _ := common.Marshal(disabled)
		common.OptionMapRWMutex.Lock()
		common.OptionMap[operation_setting.ChannelRecoveryPolicyOption] = string(raw)
		common.OptionMapRWMutex.Unlock()
		return testResult{}
	}
	runChannelRecovery(channel, policy, 1)
	current, err := model.GetChannelById(932, true)
	require.NoError(t, err)
	require.Equal(t, common.ChannelStatusAutoDisabled, current.Status)
	require.Zero(t, model.GetChannelRecoveryState(current).Successes)
}
