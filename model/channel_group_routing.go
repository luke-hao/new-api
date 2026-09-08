package model

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
)

// ChannelGroupRouting stores optional routing overrides for one channel in one group.
// A nil override inherits the corresponding channel-level value.
type ChannelGroupRouting struct {
	ChannelId        int    `json:"channel_id" gorm:"primaryKey;autoIncrement:false;index"`
	Group            string `json:"group" gorm:"type:varchar(64);primaryKey;autoIncrement:false;index"`
	PriorityOverride *int64 `json:"priority_override" gorm:"bigint"`
	WeightOverride   *uint  `json:"weight_override"`
	PriorityLocked   bool   `json:"priority_locked" gorm:"not null;default:false"`
}

type ChannelGroupRoutingPatch struct {
	ChannelId       int
	Priority        *int64
	Weight          *uint
	InheritPriority bool
	InheritWeight   bool
	PriorityLocked  *bool
}

const (
	ChannelGroupRoutingManual = "manual"
	ChannelGroupRoutingRerank = "rerank"
)

type ChannelGroupRoutingUpdateResult struct {
	Updated       int  `json:"updated"`
	SkippedLocked int  `json:"skipped_locked"`
	LockChanged   bool `json:"-"`
}

func channelGroups(groupList string) []string {
	parts := strings.Split(groupList, ",")
	groups := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		group := strings.TrimSpace(part)
		if group == "" {
			continue
		}
		if _, ok := seen[group]; ok {
			continue
		}
		seen[group] = struct{}{}
		groups = append(groups, group)
	}
	return groups
}

func channelHasGroup(channel *Channel, group string) bool {
	for _, candidate := range channelGroups(channel.Group) {
		if candidate == group {
			return true
		}
	}
	return false
}

func loadChannelGroupRoutingMap(tx *gorm.DB, channelId int) (map[string]ChannelGroupRouting, error) {
	var routings []ChannelGroupRouting
	if err := tx.Where("channel_id = ?", channelId).Find(&routings).Error; err != nil {
		return nil, err
	}
	result := make(map[string]ChannelGroupRouting, len(routings))
	for _, routing := range routings {
		result[routing.Group] = routing
	}
	return result, nil
}

func resolveChannelGroupRouting(channel *Channel, group string, routings map[string]ChannelGroupRouting) (int64, uint, bool, bool) {
	priority := channel.GetPriority()
	weight := uint(channel.GetWeight())
	priorityOverridden := false
	weightOverridden := false
	if routing, ok := routings[group]; ok {
		if routing.PriorityOverride != nil {
			priority = *routing.PriorityOverride
			priorityOverridden = true
		}
		if routing.WeightOverride != nil {
			weight = *routing.WeightOverride
			weightOverridden = true
		}
	}
	return priority, weight, priorityOverridden, weightOverridden
}

func CleanupChannelGroupRoutings(tx *gorm.DB, channel *Channel) error {
	groups := channelGroups(channel.Group)
	query := tx.Where("channel_id = ?", channel.Id)
	if len(groups) == 0 {
		return query.Delete(&ChannelGroupRouting{}).Error
	}
	return query.Where(commonGroupCol+" NOT IN ?", groups).Delete(&ChannelGroupRouting{}).Error
}

func DeleteChannelGroupRoutings(tx *gorm.DB, channelIds []int) error {
	if len(channelIds) == 0 {
		return nil
	}
	return tx.Where("channel_id IN ?", channelIds).Delete(&ChannelGroupRouting{}).Error
}

func CopyChannelGroupRoutings(sourceChannelId int, targetChannelId int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var routings []ChannelGroupRouting
		if err := tx.Where("channel_id = ?", sourceChannelId).Find(&routings).Error; err != nil {
			return err
		}
		for i := range routings {
			routings[i].ChannelId = targetChannelId
		}
		if len(routings) == 0 {
			return nil
		}
		return tx.Create(&routings).Error
	})
}

func PopulateEffectiveChannelRoutings(channels []*Channel, group string) error {
	group = strings.TrimSpace(group)
	if group == "" || len(channels) == 0 {
		return nil
	}
	ids := make([]int, 0, len(channels))
	for _, channel := range channels {
		ids = append(ids, channel.Id)
	}
	var routings []ChannelGroupRouting
	if err := DB.Where("channel_id IN ? AND "+commonGroupCol+" = ?", ids, group).Find(&routings).Error; err != nil {
		return err
	}
	routingByChannel := make(map[int]ChannelGroupRouting, len(routings))
	for _, routing := range routings {
		routingByChannel[routing.ChannelId] = routing
	}
	for _, channel := range channels {
		routingMap := map[string]ChannelGroupRouting{}
		if routing, ok := routingByChannel[channel.Id]; ok {
			routingMap[group] = routing
		}
		priority, weight, priorityOverridden, weightOverridden := resolveChannelGroupRouting(channel, group, routingMap)
		channel.EffectivePriority = &priority
		channel.EffectiveWeight = &weight
		channel.PriorityLocked = routingByChannel[channel.Id].PriorityLocked
		channel.PriorityOverridden = priorityOverridden
		channel.WeightOverridden = weightOverridden
	}
	return nil
}

func updateChannelGroupRoutingsTx(tx *gorm.DB, group string, patches []ChannelGroupRoutingPatch, mode string) (ChannelGroupRoutingUpdateResult, error) {
	result := ChannelGroupRoutingUpdateResult{}
	if len(patches) == 0 {
		return result, errors.New("updates cannot be empty")
	}
	channelIds := make([]int, 0, len(patches))
	seen := make(map[int]struct{}, len(patches))
	for _, patch := range patches {
		if patch.ChannelId <= 0 {
			return result, errors.New("invalid channel id")
		}
		if _, ok := seen[patch.ChannelId]; ok {
			return result, fmt.Errorf("duplicate channel id: %d", patch.ChannelId)
		}
		seen[patch.ChannelId] = struct{}{}
		if patch.Priority != nil && patch.InheritPriority {
			return result, errors.New("cannot set and inherit priority at the same time")
		}
		if patch.Weight != nil && patch.InheritWeight {
			return result, errors.New("cannot set and inherit weight at the same time")
		}
		if patch.Priority == nil && patch.Weight == nil && !patch.InheritPriority && !patch.InheritWeight && patch.PriorityLocked == nil {
			return result, errors.New("no routing changes")
		}
		if mode == ChannelGroupRoutingRerank && (patch.Priority == nil || patch.Weight != nil || patch.InheritPriority || patch.InheritWeight || patch.PriorityLocked != nil) {
			return result, errors.New("rerank only accepts priority updates")
		}
		channelIds = append(channelIds, patch.ChannelId)
	}
	var channels []*Channel
	if err := tx.Where("id IN ?", channelIds).Find(&channels).Error; err != nil {
		return result, err
	}
	if len(channels) != len(channelIds) {
		return result, errors.New("one or more channels do not exist")
	}
	channelById := make(map[int]*Channel, len(channels))
	for _, channel := range channels {
		if !channelHasGroup(channel, group) {
			return result, fmt.Errorf("channel %d is not attached to group %s", channel.Id, group)
		}
		channelById[channel.Id] = channel
	}
	for _, patch := range patches {
		channel := channelById[patch.ChannelId]
		routing := ChannelGroupRouting{ChannelId: patch.ChannelId, Group: group}
		err := tx.Where("channel_id = ? AND "+commonGroupCol+" = ?", patch.ChannelId, group).First(&routing).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return result, err
		}
		if mode == ChannelGroupRoutingRerank && routing.PriorityLocked {
			result.SkippedLocked++
			continue
		}
		if patch.PriorityLocked != nil {
			result.LockChanged = result.LockChanged || routing.PriorityLocked != *patch.PriorityLocked
			routing.PriorityLocked = *patch.PriorityLocked
		}
		if patch.InheritPriority {
			if routing.PriorityLocked {
				return result, errors.New("unlock priority before restoring the channel default")
			}
			routing.PriorityOverride = nil
		} else if patch.Priority != nil {
			routing.PriorityOverride = patch.Priority
		}
		// A lock snapshots the effective group priority, rather than inheriting future global edits.
		if routing.PriorityLocked && routing.PriorityOverride == nil {
			priority := channel.GetPriority()
			routing.PriorityOverride = &priority
		}
		if patch.InheritWeight {
			routing.WeightOverride = nil
		} else if patch.Weight != nil {
			routing.WeightOverride = patch.Weight
		}
		if routing.PriorityOverride == nil && routing.WeightOverride == nil && !routing.PriorityLocked {
			if err := tx.Where("channel_id = ? AND "+commonGroupCol+" = ?", patch.ChannelId, group).Delete(&ChannelGroupRouting{}).Error; err != nil {
				return result, err
			}
		} else if err := tx.Save(&routing).Error; err != nil {
			return result, err
		}
		routingMap := map[string]ChannelGroupRouting{group: routing}
		priority, weight, _, _ := resolveChannelGroupRouting(channel, group, routingMap)
		if err := tx.Model(&Ability{}).Where("channel_id = ? AND "+commonGroupCol+" = ?", patch.ChannelId, group).Updates(map[string]interface{}{"priority": priority, "weight": weight}).Error; err != nil {
			return result, err
		}
		result.Updated++
	}
	return result, nil
}

func UpdateChannelGroupRoutingsWithMode(group string, patches []ChannelGroupRoutingPatch, mode string) (ChannelGroupRoutingUpdateResult, error) {
	result := ChannelGroupRoutingUpdateResult{}
	group = strings.TrimSpace(group)
	if group == "" || utf8.RuneCountInString(group) > 64 {
		return result, errors.New("invalid group")
	}
	if mode == "" {
		mode = ChannelGroupRoutingManual
	}
	if mode != ChannelGroupRoutingManual && mode != ChannelGroupRoutingRerank {
		return result, errors.New("invalid routing update mode")
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		policy, err := lockChannelGroupStabilityPolicyTx(tx, group)
		if err != nil {
			return err
		}
		result, err = updateChannelGroupRoutingsTx(tx, group, patches, mode)
		if err != nil {
			return err
		}
		if result.LockChanged {
			// The shared write lock and version fence reject in-flight results on every DB engine.
			nextCheckAt := ChannelGroupStabilityNextCheckAt(*policy, time.Now().UnixMilli())
			return tx.Model(&ChannelGroupStabilityPolicy{}).Where(commonGroupCol+" = ?", group).Updates(map[string]interface{}{
				"config_version": gorm.Expr("config_version + 1"), "next_check_at": nextCheckAt,
				"last_primary_channel_id": 0, "last_primary_latency_ms": 0,
				"last_result": ChannelGroupStabilityResultNever, "last_message": "分组固定优先级已更新，等待下一次检测",
			}).Error
		}
		return nil
	})
	if err != nil {
		return ChannelGroupRoutingUpdateResult{}, err
	}
	return result, nil
}

func UpdateChannelGroupRoutings(group string, patches []ChannelGroupRoutingPatch) (int, error) {
	result, err := UpdateChannelGroupRoutingsWithMode(group, patches, ChannelGroupRoutingManual)
	return result.Updated, err
}
