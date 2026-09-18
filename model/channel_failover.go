package model

import (
	"sort"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

// GetChannelExcluding selects the highest remaining priority, preserving group
// and model permissions. It never falls back to an excluded channel.
func GetChannelExcluding(group, modelName string, excluded map[int]bool) (*Channel, error) {
	var candidates []ChannelCandidate
	if common.MemoryCacheEnabled {
		channelSyncLock.RLock()
		defer channelSyncLock.RUnlock()
		candidates = group2model2channels[group][modelName]
		if len(candidates) == 0 {
			candidates = group2model2channels[group][ratio_setting.FormatMatchingModelName(modelName)]
		}
	} else {
		var abilities []Ability
		query := func(name string) error {
			return DB.Where(commonGroupCol+" = ? AND model = ? AND enabled = ?", group, name, true).Find(&abilities).Error
		}
		if err := query(modelName); err != nil {
			return nil, err
		}
		if len(abilities) == 0 {
			if err := query(ratio_setting.FormatMatchingModelName(modelName)); err != nil {
				return nil, err
			}
		}
		for _, a := range abilities {
			priority := int64(0)
			if a.Priority != nil {
				priority = *a.Priority
			}
			candidates = append(candidates, ChannelCandidate{ChannelId: a.ChannelId, Priority: priority, Weight: int(a.Weight)})
		}
	}
	remaining := make([]ChannelCandidate, 0, len(candidates))
	for _, c := range candidates {
		if !excluded[c.ChannelId] {
			remaining = append(remaining, c)
		}
	}
	if len(remaining) == 0 {
		return nil, nil
	}
	sort.Slice(remaining, func(i, j int) bool { return remaining[i].Priority > remaining[j].Priority })
	top := remaining[0].Priority
	total := 0
	for _, c := range remaining {
		if c.Priority != top {
			break
		}
		total += c.Weight
	}
	zeroWeights := total == 0
	if zeroWeights {
		for _, c := range remaining {
			if c.Priority == top {
				total++
			}
		}
	}
	pick := common.GetRandomInt(total)
	for _, c := range remaining {
		if c.Priority != top {
			break
		}
		weight := c.Weight
		if zeroWeights {
			weight = 1
		}
		pick -= weight
		if pick < 0 {
			if common.MemoryCacheEnabled {
				return channelsIDM[c.ChannelId], nil
			}
			return GetChannelById(c.ChannelId, true)
		}
	}
	return nil, nil
}
