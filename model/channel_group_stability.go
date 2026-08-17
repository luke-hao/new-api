package model

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

const (
	DefaultChannelGroupStabilityIntervalMinutes = 10
	DefaultChannelGroupStabilityHealthySeconds  = 8
	DefaultChannelGroupStabilityTimeoutSeconds  = 20
	ChannelGroupStabilityResultNever            = "never"
	ChannelGroupStabilityResultHealthy          = "healthy"
	ChannelGroupStabilityResultReranked         = "reranked"
	ChannelGroupStabilityResultUnchanged        = "unchanged"
	ChannelGroupStabilityResultAllFailed        = "all_failed"
	ChannelGroupStabilityResultNoChannels       = "no_channels"
	ChannelGroupStabilityResultCancelled        = "cancelled"
	ChannelGroupStabilityResultError            = "error"
)

var ErrChannelGroupStabilityPolicyStale = errors.New("channel group stability policy changed while the task was running")

type ChannelGroupStabilityPolicy struct {
	Group                   string `json:"group" gorm:"type:varchar(64);primaryKey"`
	Enabled                 bool   `json:"enabled" gorm:"not null;default:false"`
	IntervalMinutes         int    `json:"interval_minutes" gorm:"not null;default:10"`
	HealthyThresholdSeconds int    `json:"healthy_threshold_seconds" gorm:"not null;default:8"`
	ProbeTimeoutSeconds     int    `json:"probe_timeout_seconds" gorm:"not null;default:20"`
	ConfigVersion           int64  `json:"-" gorm:"not null;default:1"`
	LastCheckAt             int64  `json:"last_check_at" gorm:"not null;default:0"`
	NextCheckAt             int64  `json:"next_check_at" gorm:"not null;default:0;index"`
	LastResult              string `json:"last_result" gorm:"type:varchar(32);not null;default:'never'"`
	LastMessage             string `json:"last_message" gorm:"type:text"`
	LastPrimaryChannelId    int    `json:"last_primary_channel_id" gorm:"not null;default:0"`
	LastPrimaryLatencyMs    int64  `json:"last_primary_latency_ms" gorm:"not null;default:0"`
	LastReorderedAt         int64  `json:"last_reordered_at" gorm:"not null;default:0"`
	CreatedAt               int64  `json:"-" gorm:"autoCreateTime:milli"`
	UpdatedAt               int64  `json:"-" gorm:"autoUpdateTime:milli"`
}

type ChannelGroupStabilityConfig struct {
	Group                   string
	Enabled                 bool
	IntervalMinutes         int
	HealthyThresholdSeconds int
	ProbeTimeoutSeconds     int
}

type ChannelGroupStabilityRunUpdate struct {
	LastCheckAt          int64
	NextCheckAt          int64
	LastResult           string
	LastMessage          string
	LastPrimaryChannelId int
	LastPrimaryLatencyMs int64
	LastReorderedAt      int64
}

func defaultChannelGroupStabilityPolicy(group string) ChannelGroupStabilityPolicy {
	return ChannelGroupStabilityPolicy{
		Group:                   strings.TrimSpace(group),
		IntervalMinutes:         DefaultChannelGroupStabilityIntervalMinutes,
		HealthyThresholdSeconds: DefaultChannelGroupStabilityHealthySeconds,
		ProbeTimeoutSeconds:     DefaultChannelGroupStabilityTimeoutSeconds,
		ConfigVersion:           1,
		LastResult:              ChannelGroupStabilityResultNever,
	}
}

func GetOrCreateChannelGroupStabilityPolicy(group string) (*ChannelGroupStabilityPolicy, error) {
	group = strings.TrimSpace(group)
	if group == "" {
		return nil, errors.New("group cannot be empty")
	}
	policy := defaultChannelGroupStabilityPolicy(group)
	if err := DB.Where(commonGroupCol+" = ?", group).FirstOrCreate(&policy).Error; err != nil {
		return nil, err
	}
	return &policy, nil
}

func SaveChannelGroupStabilityConfig(config ChannelGroupStabilityConfig, now int64) (*ChannelGroupStabilityPolicy, error) {
	config.Group = strings.TrimSpace(config.Group)
	if config.Group == "" {
		return nil, errors.New("group cannot be empty")
	}
	if utf8.RuneCountInString(config.Group) > 64 {
		return nil, errors.New("group is too long")
	}

	policy := defaultChannelGroupStabilityPolicy(config.Group)
	err := DB.Transaction(func(tx *gorm.DB) error {
		err := tx.Where(commonGroupCol+" = ?", config.Group).First(&policy).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			policy.Enabled = config.Enabled
			policy.IntervalMinutes = config.IntervalMinutes
			policy.HealthyThresholdSeconds = config.HealthyThresholdSeconds
			policy.ProbeTimeoutSeconds = config.ProbeTimeoutSeconds
			if config.Enabled {
				policy.NextCheckAt = now
			}
			return tx.Create(&policy).Error
		}
		if err != nil {
			return err
		}

		policy.Enabled = config.Enabled
		policy.IntervalMinutes = config.IntervalMinutes
		policy.HealthyThresholdSeconds = config.HealthyThresholdSeconds
		policy.ProbeTimeoutSeconds = config.ProbeTimeoutSeconds
		policy.ConfigVersion++
		if config.Enabled {
			policy.NextCheckAt = now
		} else {
			policy.NextCheckAt = 0
		}
		return tx.Save(&policy).Error
	})
	if err != nil {
		return nil, err
	}
	return &policy, nil
}

func ListDueChannelGroupStabilityPolicies(now int64) ([]ChannelGroupStabilityPolicy, error) {
	policies := make([]ChannelGroupStabilityPolicy, 0)
	err := DB.Where("enabled = ? AND (next_check_at = 0 OR next_check_at <= ?)", true, now).
		Order("next_check_at ASC").
		Find(&policies).Error
	return policies, err
}

func UpdateChannelGroupStabilityRun(policy ChannelGroupStabilityPolicy, automatic bool, update ChannelGroupStabilityRunUpdate) (bool, error) {
	values := map[string]interface{}{
		"last_check_at":           update.LastCheckAt,
		"next_check_at":           update.NextCheckAt,
		"last_result":             update.LastResult,
		"last_message":            update.LastMessage,
		"last_primary_channel_id": update.LastPrimaryChannelId,
		"last_primary_latency_ms": update.LastPrimaryLatencyMs,
		"last_reordered_at":       update.LastReorderedAt,
	}
	query := DB.Model(&ChannelGroupStabilityPolicy{}).
		Where(commonGroupCol+" = ? AND config_version = ?", policy.Group, policy.ConfigVersion)
	if automatic {
		query = query.Where("enabled = ?", true)
	}
	result := query.Updates(values)
	return result.RowsAffected == 1, result.Error
}

func ApplyChannelGroupStabilityPriorities(policy ChannelGroupStabilityPolicy, automatic bool, patches []ChannelGroupRoutingPatch) (int, error) {
	updated := 0
	err := DB.Transaction(func(tx *gorm.DB) error {
		query := tx.Where(commonGroupCol+" = ? AND config_version = ?", policy.Group, policy.ConfigVersion)
		if automatic {
			query = query.Where("enabled = ?", true)
		}
		var current ChannelGroupStabilityPolicy
		if err := query.First(&current).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrChannelGroupStabilityPolicyStale
			}
			return err
		}
		var err error
		updated, err = updateChannelGroupRoutingsTx(tx, policy.Group, patches)
		return err
	})
	return updated, err
}

func GetEnabledChannelsForGroupStability(group string) ([]*Channel, error) {
	group = strings.TrimSpace(group)
	if group == "" {
		return nil, errors.New("group cannot be empty")
	}
	channels := make([]*Channel, 0)
	query := ApplyChannelGroupFilter(DB.Where("status = ?", common.ChannelStatusEnabled), group)
	if err := query.Order("id ASC").Find(&channels).Error; err != nil {
		return nil, err
	}
	if err := PopulateEffectiveChannelRoutings(channels, group); err != nil {
		return nil, err
	}
	return channels, nil
}

func ChannelGroupStabilityNextCheckAt(policy ChannelGroupStabilityPolicy, completedAt int64) int64 {
	if !policy.Enabled {
		return 0
	}
	return completedAt + int64(time.Duration(policy.IntervalMinutes)*time.Minute/time.Millisecond)
}
