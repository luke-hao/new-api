package model

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Model routing is persistent source data; abilities is its rebuildable projection.
type ChannelModelRouting struct {
	ChannelId        int    `json:"channel_id" gorm:"primaryKey;autoIncrement:false"`
	Group            string `json:"group" gorm:"type:varchar(64);primaryKey;autoIncrement:false"`
	Model            string `json:"model" gorm:"type:varchar(255);primaryKey;autoIncrement:false"`
	PriorityOverride *int64 `json:"priority_override" gorm:"bigint"`
	PriorityLocked   bool   `json:"priority_locked" gorm:"not null;default:false"`
	LastCheckAt      int64  `json:"last_check_at"`
	LatencyMs        int64  `json:"latency_ms"`
	Result           string `json:"result" gorm:"type:varchar(32)"`
	Message          string `json:"message" gorm:"type:text"`
}

type ChannelModelStabilityPolicy struct {
	Group                   string `json:"group" gorm:"type:varchar(64);primaryKey;autoIncrement:false"`
	Model                   string `json:"model" gorm:"type:varchar(255);primaryKey;autoIncrement:false"`
	Paused                  bool   `json:"paused" gorm:"not null;default:false"`
	IntervalMinutes         *int   `json:"interval_minutes_override"`
	HealthyThresholdSeconds *int   `json:"healthy_threshold_seconds_override"`
	ProbeTimeoutSeconds     *int   `json:"probe_timeout_seconds_override"`
	ConfigVersion           int64  `json:"-" gorm:"not null;default:1"`
	Initialized             bool   `json:"initialized" gorm:"not null;default:false"`
	LastCheckAt             int64  `json:"last_check_at"`
	NextCheckAt             int64  `json:"next_check_at" gorm:"index"`
	LastResult              string `json:"last_result" gorm:"type:varchar(32)"`
	LastMessage             string `json:"last_message" gorm:"type:text"`
	LastPrimaryChannelId    int    `json:"last_primary_channel_id"`
	LastPrimaryLatencyMs    int64  `json:"last_primary_latency_ms"`
	LastReorderedAt         int64  `json:"last_reordered_at"`
}

// Registered by the scheduler. Empty model means all models in the group.
var CancelChannelModelStability func(group, model string)

func cancelModelStability(group, model string) {
	if CancelChannelModelStability != nil {
		CancelChannelModelStability(group, model)
	}
}

func channelHasModel(channel *Channel, name string) bool {
	for _, value := range channel.GetModels() {
		if strings.TrimSpace(value) == name {
			return true
		}
	}
	return false
}

func ListChannelRoutingModels(group string) ([]string, error) {
	var channels []*Channel
	if err := ApplyChannelGroupFilter(DB.Model(&Channel{}), group).Select("models").Find(&channels).Error; err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, c := range channels {
		for _, name := range c.GetModels() {
			name = strings.TrimSpace(name)
			if name != "" {
				seen[name] = true
			}
		}
	}
	result := make([]string, 0, len(seen))
	for name := range seen {
		result = append(result, name)
	}
	sort.Strings(result)
	return result, nil
}

func GetChannelModelPolicy(group, name string) (*ChannelModelStabilityPolicy, error) {
	p := ChannelModelStabilityPolicy{Group: group, Model: name, ConfigVersion: 1}
	err := DB.Where(commonGroupCol+" = ? AND model = ?", group, name).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &p, nil
	}
	return &p, err
}

func ResolveChannelModelPolicy(parent ChannelGroupStabilityPolicy, p ChannelModelStabilityPolicy) ChannelGroupStabilityPolicy {
	result := parent
	result.Model = p.Model
	result.ParentConfigVersion = parent.ConfigVersion
	result.ConfigVersion = p.ConfigVersion
	result.Enabled = parent.Enabled && !p.Paused
	result.Initialized = p.Initialized
	if p.IntervalMinutes != nil {
		result.IntervalMinutes = *p.IntervalMinutes
	}
	if p.HealthyThresholdSeconds != nil {
		result.HealthyThresholdSeconds = *p.HealthyThresholdSeconds
	}
	if p.ProbeTimeoutSeconds != nil {
		result.ProbeTimeoutSeconds = *p.ProbeTimeoutSeconds
	}
	result.LastCheckAt, result.NextCheckAt = p.LastCheckAt, p.NextCheckAt
	if !result.Enabled {
		result.NextCheckAt = 0
	}
	result.LastResult, result.LastMessage = p.LastResult, p.LastMessage
	if result.LastResult == "" {
		result.LastResult = ChannelGroupStabilityResultNever
	}
	result.LastPrimaryChannelId, result.LastPrimaryLatencyMs = p.LastPrimaryChannelId, p.LastPrimaryLatencyMs
	result.LastReorderedAt = p.LastReorderedAt
	return result
}

func validModelStabilitySettings(p ChannelGroupStabilityPolicy) error {
	if p.IntervalMinutes < 1 || p.IntervalMinutes > 1440 || p.HealthyThresholdSeconds < 1 || p.HealthyThresholdSeconds > 300 || p.ProbeTimeoutSeconds < 2 || p.ProbeTimeoutSeconds > 600 || p.ProbeTimeoutSeconds <= p.HealthyThresholdSeconds {
		return errors.New("检测间隔须为 1–1440 分钟，健康阈值 1–300 秒，超时 2–600 秒且大于健康阈值")
	}
	return nil
}

func lockChannelModelPolicyTx(tx *gorm.DB, group, name string) (*ChannelModelStabilityPolicy, error) {
	p := ChannelModelStabilityPolicy{Group: group, Model: name, ConfigVersion: 1}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&p).Error; err != nil {
		return nil, err
	}
	if err := tx.Model(&p).Where(commonGroupCol+" = ? AND model = ?", group, name).UpdateColumn("config_version", gorm.Expr("config_version")).Error; err != nil {
		return nil, err
	}
	err := tx.Where(commonGroupCol+" = ? AND model = ?", group, name).First(&p).Error
	return &p, err
}

func SaveChannelModelPolicy(input ChannelModelStabilityPolicy) error {
	names, err := ListChannelRoutingModels(input.Group)
	if err != nil {
		return err
	}
	found := false
	for _, name := range names {
		if name == input.Model {
			found = true
		}
	}
	if !found {
		return errors.New("当前分组没有此模型")
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		parent, err := lockChannelGroupStabilityPolicyTx(tx, input.Group)
		if err != nil {
			return err
		}
		p, err := lockChannelModelPolicyTx(tx, input.Group, input.Model)
		if err != nil {
			return err
		}
		p.Paused = input.Paused
		p.IntervalMinutes = input.IntervalMinutes
		p.HealthyThresholdSeconds = input.HealthyThresholdSeconds
		p.ProbeTimeoutSeconds = input.ProbeTimeoutSeconds
		effective := ResolveChannelModelPolicy(*parent, *p)
		if err := validModelStabilitySettings(effective); err != nil {
			return err
		}
		p.ConfigVersion++
		p.NextCheckAt = 0
		return tx.Save(p).Error
	})
	if err == nil {
		cancelModelStability(input.Group, input.Model)
	}
	return err
}

func ValidateInheritedModelSettingsTx(tx *gorm.DB, parent ChannelGroupStabilityPolicy) error {
	var children []ChannelModelStabilityPolicy
	if err := tx.Where(commonGroupCol+" = ?", parent.Group).Find(&children).Error; err != nil {
		return err
	}
	for _, child := range children {
		if err := validModelStabilitySettings(ResolveChannelModelPolicy(parent, child)); err != nil {
			return fmt.Errorf("%s: %w", child.Model, err)
		}
	}
	return nil
}

func ResolveModelRouting(channel *Channel, name string, groupRouting ChannelGroupRouting, routing ChannelModelRouting) (int64, bool, string) {
	priority := channel.GetPriority()
	source := "channel"
	if groupRouting.PriorityOverride != nil {
		priority = *groupRouting.PriorityOverride
		source = "group"
	}
	if routing.PriorityOverride != nil {
		priority = *routing.PriorityOverride
		source = "model"
	}
	// A group lock explicitly fixes the priority for every model.
	if groupRouting.PriorityLocked {
		if groupRouting.PriorityOverride != nil {
			priority = *groupRouting.PriorityOverride
		}
		source = "group"
	}
	return priority, groupRouting.PriorityLocked || routing.PriorityLocked, source
}

func ReprojectChannelModelPrioritiesTx(tx *gorm.DB, channelID int) error {
	var c Channel
	if err := tx.First(&c, channelID).Error; err != nil {
		return err
	}
	var rows []ChannelModelRouting
	if err := tx.Where("channel_id = ?", channelID).Find(&rows).Error; err != nil {
		return err
	}
	groups, err := loadChannelGroupRoutingMap(tx, channelID)
	if err != nil {
		return err
	}
	for _, r := range rows {
		if !channelHasGroup(&c, r.Group) || !channelHasModel(&c, r.Model) {
			if err := tx.Where("channel_id = ? AND "+commonGroupCol+" = ? AND model = ?", channelID, r.Group, r.Model).Delete(&ChannelModelRouting{}).Error; err != nil {
				return err
			}
			continue
		}
		priority, _, _ := ResolveModelRouting(&c, r.Model, groups[r.Group], r)
		if err := tx.Model(&Ability{}).Where("channel_id = ? AND "+commonGroupCol+" = ? AND model = ?", channelID, r.Group, r.Model).Update("priority", priority).Error; err != nil {
			return err
		}
	}
	return nil
}

func PopulateEffectiveModelRoutings(channels []*Channel, group, name string) error {
	if err := PopulateEffectiveChannelRoutings(channels, group); err != nil {
		return err
	}
	if name == "" || len(channels) == 0 {
		return nil
	}
	ids := make([]int, 0, len(channels))
	for _, c := range channels {
		ids = append(ids, c.Id)
	}
	var rows []ChannelModelRouting
	var groups []ChannelGroupRouting
	if err := DB.Where("channel_id IN ? AND "+commonGroupCol+" = ? AND model = ?", ids, group, name).Find(&rows).Error; err != nil {
		return err
	}
	if err := DB.Where("channel_id IN ? AND "+commonGroupCol+" = ?", ids, group).Find(&groups).Error; err != nil {
		return err
	}
	byID := map[int]ChannelModelRouting{}
	groupByID := map[int]ChannelGroupRouting{}
	for _, r := range rows {
		byID[r.ChannelId] = r
	}
	for _, r := range groups {
		groupByID[r.ChannelId] = r
	}
	for _, c := range channels {
		r, g := byID[c.Id], groupByID[c.Id]
		priority, locked, source := ResolveModelRouting(c, name, g, r)
		c.EffectivePriority = &priority
		c.PriorityLocked = locked
		c.PriorityOverridden = r.PriorityOverride != nil
		c.PrioritySource = source
		c.GroupPriorityLocked = g.PriorityLocked
		c.RoutingModel = name
		c.ModelTestResult = r.Result
		c.ModelTestMessage = r.Message
		c.ModelTestTime = r.LastCheckAt
		c.ModelResponseTime = r.LatencyMs
	}
	return nil
}

func ApplyRoutingModelFilter(query *gorm.DB, group, name string) *gorm.DB {
	if name == "" {
		return query
	}
	return query.Where("EXISTS (SELECT 1 FROM abilities AS arm WHERE arm.channel_id = channels.id AND arm."+commonGroupCol+" = ? AND arm.model = ?)", group, name)
}

func (options ChannelSortOptions) ApplyForModel(query *gorm.DB, group, name string) *gorm.DB {
	if name == "" {
		return options.ApplyForGroup(query, group)
	}
	if options.SortBy == "response_time" || options.SortBy == "test_time" {
		column := "latency_ms"
		if options.SortBy == "test_time" {
			column = "last_check_at"
		}
		direction := "DESC"
		if options.SortOrder == "asc" {
			direction = "ASC"
		}
		return query.Joins("LEFT JOIN channel_model_routings AS cmr_sort ON cmr_sort.channel_id = channels.id AND cmr_sort."+commonGroupCol+" = ? AND cmr_sort.model = ?", group, name).
			Order("COALESCE(cmr_sort." + column + ",0) " + direction).Order("channels.id ASC")
	}
	if options.SortBy == "priority" || (options.SortBy == "" && !options.IDSort) {
		direction := "DESC"
		if options.SortBy == "priority" && options.SortOrder == "asc" {
			direction = "ASC"
		}
		return query.Joins("JOIN abilities AS arm_sort ON arm_sort.channel_id = channels.id AND arm_sort."+commonGroupCol+" = ? AND arm_sort.model = ?", group, name).Order("arm_sort.priority " + direction).Order("channels.id ASC")
	}
	return options.Apply(query)
}

func updateChannelModelRoutingTx(tx *gorm.DB, group, name string, patches []ChannelGroupRoutingPatch, mode string) (ChannelGroupRoutingUpdateResult, error) {
	result := ChannelGroupRoutingUpdateResult{}
	if len(patches) == 0 {
		return result, errors.New("updates cannot be empty")
	}
	seen := map[int]bool{}
	for _, patch := range patches {
		if seen[patch.ChannelId] || patch.ChannelId <= 0 {
			return result, errors.New("invalid or duplicate channel")
		}
		seen[patch.ChannelId] = true
		if patch.Weight != nil || patch.InheritWeight {
			return result, errors.New("权重使用分组设置")
		}
		if patch.Priority != nil && patch.InheritPriority {
			return result, errors.New("conflicting priority")
		}
		if patch.Priority == nil && !patch.InheritPriority && patch.PriorityLocked == nil {
			return result, errors.New("no routing changes")
		}
		if mode == ChannelGroupRoutingRerank && (patch.Priority == nil || patch.InheritPriority || patch.PriorityLocked != nil) {
			return result, errors.New("rerank only accepts priority")
		}
		var c Channel
		if err := tx.First(&c, patch.ChannelId).Error; err != nil {
			return result, err
		}
		if !channelHasGroup(&c, group) || !channelHasModel(&c, name) {
			return result, errors.New("channel is not a member of this group/model")
		}
		groups, err := loadChannelGroupRoutingMap(tx, c.Id)
		if err != nil {
			return result, err
		}
		r := ChannelModelRouting{ChannelId: c.Id, Group: group, Model: name}
		err = tx.Where("channel_id = ? AND "+commonGroupCol+" = ? AND model = ?", c.Id, group, name).First(&r).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return result, err
		}
		if groups[group].PriorityLocked {
			if mode == ChannelGroupRoutingRerank {
				result.SkippedLocked++
				continue
			}
			return result, errors.New("请先解除该渠道的分组固定优先级")
		}
		if mode == ChannelGroupRoutingRerank && r.PriorityLocked {
			result.SkippedLocked++
			continue
		}
		if patch.PriorityLocked != nil {
			result.LockChanged = result.LockChanged || r.PriorityLocked != *patch.PriorityLocked
			r.PriorityLocked = *patch.PriorityLocked
		}
		if patch.InheritPriority {
			if r.PriorityLocked {
				return result, errors.New("请先解除固定优先级")
			}
			r.PriorityOverride = nil
		}
		if patch.Priority != nil {
			r.PriorityOverride = patch.Priority
		}
		if r.PriorityLocked && r.PriorityOverride == nil {
			priority, _, _ := ResolveModelRouting(&c, name, groups[group], r)
			r.PriorityOverride = &priority
		}
		if err := tx.Save(&r).Error; err != nil {
			return result, err
		}
		priority, _, _ := ResolveModelRouting(&c, name, groups[group], r)
		if err := tx.Model(&Ability{}).Where("channel_id = ? AND "+commonGroupCol+" = ? AND model = ?", c.Id, group, name).Update("priority", priority).Error; err != nil {
			return result, err
		}
		result.Updated++
	}
	return result, nil
}

func UpdateChannelModelRoutings(group, name string, patches []ChannelGroupRoutingPatch, mode string) (ChannelGroupRoutingUpdateResult, error) {
	result := ChannelGroupRoutingUpdateResult{}
	if group == "" || name == "" || len(name) > 255 {
		return result, errors.New("invalid group/model")
	}
	if mode == "" {
		mode = ChannelGroupRoutingManual
	}
	if mode != ChannelGroupRoutingManual && mode != ChannelGroupRoutingRerank {
		return result, errors.New("invalid mode")
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		parent, err := lockChannelGroupStabilityPolicyTx(tx, group)
		if err != nil {
			return err
		}
		p, err := lockChannelModelPolicyTx(tx, group, name)
		if err != nil {
			return err
		}
		result, err = updateChannelModelRoutingTx(tx, group, name, patches, mode)
		if err != nil {
			return err
		}
		p.ConfigVersion++
		p.NextCheckAt = ChannelGroupStabilityNextCheckAt(ResolveChannelModelPolicy(*parent, *p), time.Now().UnixMilli())
		return tx.Save(p).Error
	})
	if err == nil {
		cancelModelStability(group, name)
	}
	return result, err
}

// Channel membership/config changes invalidate in-flight model runs transactionally.
func InvalidateChannelModelRunsTx(tx *gorm.DB, c *Channel) error {
	groups := channelGroups(c.Group)
	var old []string
	if err := tx.Model(&Ability{}).Where("channel_id = ?", c.Id).Distinct(commonGroupCol).Pluck(commonGroupCol, &old).Error; err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, g := range append(groups, old...) {
		seen[g] = true
	}
	groups = groups[:0]
	for g := range seen {
		groups = append(groups, g)
	}
	sort.Strings(groups)
	for _, g := range groups {
		p, err := lockChannelGroupStabilityPolicyTx(tx, g)
		if err != nil {
			return err
		}
		if err := tx.Model(p).Where(commonGroupCol+" = ?", g).UpdateColumn("config_version", gorm.Expr("config_version + 1")).Error; err != nil {
			return err
		}
		cancelModelStability(g, "")
	}
	return nil
}

func RestoreChannelModelRoutingProjection() error {
	var ids []int
	if err := DB.Model(&ChannelModelRouting{}).Distinct("channel_id").Pluck("channel_id", &ids).Error; err != nil {
		return err
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		for _, id := range ids {
			var count int64
			if err := tx.Model(&Channel{}).Where("id = ?", id).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				if err := tx.Where("channel_id = ?", id).Delete(&ChannelModelRouting{}).Error; err != nil {
					return err
				}
				continue
			}
			if err := ReprojectChannelModelPrioritiesTx(tx, id); err != nil {
				return err
			}
		}
		return nil
	})
}
