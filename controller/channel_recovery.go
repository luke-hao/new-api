package controller

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

var channelRecoveryOnce sync.Once
var channelRecoveryProbe = testChannelWithContext

func channelRecoveryReason(channel *model.Channel) string {
	info := channel.GetOtherInfo()
	if reason, ok := info["auto_disable_reason"].(string); ok && reason != "" {
		return reason
	}
	reason := service.ChannelDisableReasonFromText(fmt.Sprint(info["status_reason"]))
	if reason == "other" && channel.ChannelInfo.IsMultiKey {
		for index := range channel.GetKeys() {
			if channel.ChannelInfo.MultiKeyStatusList[index] == common.ChannelStatusAutoDisabled {
				if candidate := service.ChannelDisableReasonFromText(channel.ChannelInfo.MultiKeyDisabledReason[index]); candidate != "other" {
					return candidate
				}
			}
		}
	}
	return reason
}
func channelRecoveryDue(channel *model.Channel, policy operation_setting.ChannelRecoveryPolicy, now int64) (model.ChannelRecoveryState, bool) {
	state := model.GetChannelRecoveryState(channel)
	if channel.Status != common.ChannelStatusAutoDisabled || !policy.Enabled {
		return state, false
	}
	reason := channelRecoveryReason(channel)
	if state.Reason != "" {
		reason = state.Reason
	}
	rule, ok := policy.Rules[reason]
	if !ok || !rule.Enabled {
		return state, false
	}
	state.Reason = reason
	base := state.LastTestAt
	if base == 0 {
		if stamp, ok := channel.GetOtherInfo()["status_time"].(float64); ok {
			base = int64(stamp)
		}
	}
	// Recompute from the configured interval so edits take effect immediately.
	due := base + int64(rule.IntervalMinutes)*60
	return state, now >= due
}
func recoveryProbeChannel(channel *model.Channel, attempt int) (*model.Channel, bool) {
	probe := *channel
	if !channel.ChannelInfo.IsMultiKey {
		return &probe, true
	}
	keys := channel.GetKeys()
	candidates := []string{}
	for i, key := range keys {
		if channel.ChannelInfo.MultiKeyStatusList[i] == common.ChannelStatusAutoDisabled {
			candidates = append(candidates, key)
		}
	}
	if len(candidates) == 0 {
		return nil, false
	}
	probe.Key = candidates[attempt%len(candidates)]
	probe.Keys = nil
	// Use exactly the selected auto-disabled key without mutating the live cache.
	probe.ChannelInfo = model.ChannelInfo{}
	return &probe, true
}

func runChannelRecovery(channel *model.Channel, policy operation_setting.ChannelRecoveryPolicy, testUserID int) {
	now := time.Now().Unix()
	state, due := channelRecoveryDue(channel, policy, now)
	if !due {
		return
	}
	probe, ok := recoveryProbeChannel(channel, state.KeyIndex)
	if !ok {
		return
	}
	rule := policy.Rules[state.Reason]
	state.LastTestAt = now
	state.NextTestAt = now + int64(rule.IntervalMinutes)*60
	state.Attempts++
	claimed, applied, err := model.SaveChannelRecovery(channel, state, nil)
	if err != nil || !applied {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	started := time.Now()
	result := channelRecoveryProbe(ctx, probe, testUserID, "", "", shouldUseStreamForAutomaticChannelTest(probe))
	// Keep policy updates serialized with the final write. A disabled rule
	// must not re-enable a channel between this check and the transaction.
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	currentPolicy, policyErr := operation_setting.ParseChannelRecoveryPolicy(common.OptionMap[operation_setting.ChannelRecoveryPolicyOption])
	if policyErr != nil || !reflect.DeepEqual(policy, currentPolicy) {
		return
	}
	success := result.localErr == nil && result.newAPIError == nil && ctx.Err() == nil
	// A timeout-classified channel must also meet the configured latency threshold.
	threshold := common.ChannelDisableThreshold
	if success && state.Reason == "timeout" && threshold > 0 && time.Since(started).Seconds() > threshold {
		success = false
		state.LastError = "响应时间仍超过自动禁用阈值"
	}
	if success {
		state.Successes++
		state.LastError = ""
	} else {
		state.Successes = 0
		state.KeyIndex++
		if result.localErr != nil {
			state.LastError = "恢复检测失败（本地测试器错误）"
		}
		if result.newAPIError != nil {
			state.Reason = service.ChannelDisableReason(result.newAPIError)
			state.LastError = "恢复检测失败：" + state.Reason
		}
		if ctx.Err() != nil {
			state.LastError = "恢复检测超时"
		}
		// Store only a category, never upstream response bodies or credentials.

	}
	state.LastTestAt = time.Now().Unix()
	var enableKey *string
	if success && state.Successes >= rule.SuccessesRequired {
		enableKey = &probe.Key
		state.NextTestAt = 0
	} else if next, ok := policy.Rules[state.Reason]; ok {
		state.NextTestAt = time.Now().Unix() + int64(next.IntervalMinutes)*60
	}
	_, applied, err = model.SaveChannelRecovery(claimed, state, enableKey)
	if err != nil {
		common.SysLog(fmt.Sprintf("channel recovery save failed: channel #%d: %v", channel.Id, err))
	}
	if applied && enableKey != nil {
		common.SysLog(fmt.Sprintf("channel #%d recovered after successful checks (%s)", channel.Id, state.Reason))
	}
}

func AutomaticallyRecoverChannels() {
	if !common.IsMasterNode {
		return
	}
	channelRecoveryOnce.Do(func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			policy := operation_setting.GetChannelRecoveryPolicy()
			if !policy.Enabled {
				continue
			}
			testUserID, err := resolveChannelTestUserID(nil)
			if err != nil {
				continue
			}
			var channels []*model.Channel
			if err := model.DB.Where("status = ?", common.ChannelStatusAutoDisabled).Order("id").Find(&channels).Error; err != nil {
				continue
			}
			var wg sync.WaitGroup
			slots := make(chan struct{}, 2)
			for _, channel := range channels {
				if _, due := channelRecoveryDue(channel, policy, time.Now().Unix()); !due {
					continue
				}
				slots <- struct{}{}
				wg.Add(1)
				go func(channel *model.Channel) {
					defer wg.Done()
					defer func() { <-slots }()
					// Configuration can change while waiting for a worker.
					if reflect.DeepEqual(policy, operation_setting.GetChannelRecoveryPolicy()) {
						runChannelRecovery(channel, policy, testUserID)
					}
				}(channel)
			}
			wg.Wait()
		}
	})
}
