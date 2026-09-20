package operation_setting

import (
	"fmt"
	"github.com/QuantumNous/new-api/common"
)

const ChannelRecoveryPolicyOption = "ChannelRecoveryPolicy"

var ChannelRecoveryReasons = []string{"balance", "rate_limit", "timeout", "authentication", "other"}

type ChannelRecoveryRule struct {
	Enabled           bool `json:"enabled"`
	IntervalMinutes   int  `json:"interval_minutes"`
	SuccessesRequired int  `json:"successes_required"`
}
type ChannelRecoveryPolicy struct {
	Enabled bool                           `json:"enabled"`
	Rules   map[string]ChannelRecoveryRule `json:"rules"`
}

func DefaultChannelRecoveryPolicy() ChannelRecoveryPolicy {
	return ChannelRecoveryPolicy{Rules: map[string]ChannelRecoveryRule{
		"balance": {true, 30, 1}, "rate_limit": {true, 2, 2}, "timeout": {true, 5, 2},
		"authentication": {false, 60, 1}, "other": {false, 10, 2},
	}}
}
func ParseChannelRecoveryPolicy(raw string) (ChannelRecoveryPolicy, error) {
	var policy ChannelRecoveryPolicy
	if err := common.UnmarshalJsonStr(raw, &policy); err != nil {
		return policy, fmt.Errorf("恢复规则格式错误: %w", err)
	}
	if len(policy.Rules) != len(ChannelRecoveryReasons) {
		return policy, fmt.Errorf("恢复规则必须包含全部五类禁用原因")
	}
	for _, reason := range ChannelRecoveryReasons {
		rule, ok := policy.Rules[reason]
		if !ok || rule.IntervalMinutes < 1 || rule.IntervalMinutes > 10080 || rule.SuccessesRequired < 1 || rule.SuccessesRequired > 10 {
			return policy, fmt.Errorf("%s: 重试间隔须为 1–10080 分钟，连续成功次数须为 1–10", reason)
		}
	}
	return policy, nil
}
func GetChannelRecoveryPolicy() ChannelRecoveryPolicy {
	common.OptionMapRWMutex.RLock()
	raw := common.OptionMap[ChannelRecoveryPolicyOption]
	common.OptionMapRWMutex.RUnlock()
	policy, err := ParseChannelRecoveryPolicy(raw)
	if err != nil {
		return DefaultChannelRecoveryPolicy()
	}
	return policy
}
