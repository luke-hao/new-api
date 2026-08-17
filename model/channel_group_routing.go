package model

import (
	"errors"
	"fmt"
	"strings"
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
}

type ChannelGroupRoutingPatch struct {
	ChannelId       int
	Priority        *int64
	Weight          *uint
	InheritPriority bool
	InheritWeight   bool
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
		channel.PriorityOverridden = priorityOverridden
		channel.WeightOverridden = weightOverridden
	}
	return nil
}

func updateChannelGroupRoutingsTx(tx *gorm.DB, group string, patches []ChannelGroupRoutingPatch) (int, error) {
	group = strings.TrimSpace(group)
	if group == "" {
		return 0, errors.New("group cannot be empty")
	}
	if utf8.RuneCountInString(group) > 64 {
		return 0, errors.New("group is too long")
	}
	if len(patches) == 0 {
		return 0, errors.New("updates cannot be empty")
	}

	channelIds := make([]int, 0, len(patches))
	seen := make(map[int]struct{}, len(patches))
	for _, patch := range patches {
		if patch.ChannelId <= 0 {
			return 0, errors.New("invalid channel id")
		}
		if _, ok := seen[patch.ChannelId]; ok {
			return 0, fmt.Errorf("duplicate channel id: %d", patch.ChannelId)
		}
		seen[patch.ChannelId] = struct{}{}
		if patch.Priority != nil && patch.InheritPriority {
			return 0, fmt.Errorf("channel %d cannot set and inherit priority at the same time", patch.ChannelId)
		}
		if patch.Weight != nil && patch.InheritWeight {
			return 0, fmt.Errorf("channel %d cannot set and inherit weight at the same time", patch.ChannelId)
		}
		if patch.Priority == nil && patch.Weight == nil && !patch.InheritPriority && !patch.InheritWeight {
			return 0, fmt.Errorf("channel %d has no routing changes", patch.ChannelId)
		}
		channelIds = append(channelIds, patch.ChannelId)
	}

	var channels []*Channel
	if err := tx.Where("id IN ?", channelIds).Find(&channels).Error; err != nil {
		return 0, err
	}
	if len(channels) != len(channelIds) {
		return 0, errors.New("one or more channels do not exist")
	}
	channelById := make(map[int]*Channel, len(channels))
	for _, channel := range channels {
		if !channelHasGroup(channel, group) {
			return 0, fmt.Errorf("channel %d is not attached to group %s", channel.Id, group)
		}
		channelById[channel.Id] = channel
	}

	for _, patch := range patches {
		channel := channelById[patch.ChannelId]
		routing := ChannelGroupRouting{ChannelId: patch.ChannelId, Group: group}
		err := tx.Where("channel_id = ? AND "+commonGroupCol+" = ?", patch.ChannelId, group).First(&routing).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, err
		}
		if patch.InheritPriority {
			routing.PriorityOverride = nil
		} else if patch.Priority != nil {
			routing.PriorityOverride = patch.Priority
		}
		if patch.InheritWeight {
			routing.WeightOverride = nil
		} else if patch.Weight != nil {
			routing.WeightOverride = patch.Weight
		}

		if routing.PriorityOverride == nil && routing.WeightOverride == nil {
			if err := tx.Where("channel_id = ? AND "+commonGroupCol+" = ?", patch.ChannelId, group).Delete(&ChannelGroupRouting{}).Error; err != nil {
				return 0, err
			}
		} else if err := tx.Save(&routing).Error; err != nil {
			return 0, err
		}

		routingMap := map[string]ChannelGroupRouting{}
		if routing.PriorityOverride != nil || routing.WeightOverride != nil {
			routingMap[group] = routing
		}
		priority, weight, _, _ := resolveChannelGroupRouting(channel, group, routingMap)
		if err := tx.Model(&Ability{}).
			Where("channel_id = ? AND "+commonGroupCol+" = ?", patch.ChannelId, group).
			Updates(map[string]interface{}{"priority": priority, "weight": weight}).Error; err != nil {
			return 0, err
		}
	}
	return len(patches), nil
}

func UpdateChannelGroupRoutings(group string, patches []ChannelGroupRoutingPatch) (int, error) {
	updated := 0
	err := DB.Transaction(func(tx *gorm.DB) error {
		var err error
		updated, err = updateChannelGroupRoutingsTx(tx, group, patches)
		return err
	})
	return updated, err
}
