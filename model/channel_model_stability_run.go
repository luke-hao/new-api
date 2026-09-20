package model

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

func modelStabilityChannelsTx(tx *gorm.DB, group, name string) ([]*Channel, string, error) {
	var channels []*Channel
	query := ApplyRoutingModelFilter(ApplyChannelGroupFilter(tx.Model(&Channel{}), group), group, name)
	if err := query.Where("channels.status = ?", common.ChannelStatusEnabled).Order("channels.id ASC").Find(&channels).Error; err != nil {
		return nil, "", err
	}
	// Only configuration participates: routine balance/test counters must not invalidate probes.
	type config struct {
		ID, Status, Type         int
		Models, Group, Key       string
		Mapping, BaseURL         *string
		Settings                 string
		Setting, Params, Headers *string
	}
	snapshot := make([]config, 0, len(channels))
	for _, c := range channels {
		snapshot = append(snapshot, config{c.Id, c.Status, c.Type, c.Models, c.Group, c.Key, c.ModelMapping, c.BaseURL, c.OtherSettings, c.Setting, c.ParamOverride, c.HeaderOverride})
	}
	data, err := common.Marshal(snapshot)
	if err != nil {
		return nil, "", err
	}
	return channels, fmt.Sprintf("%x", sha256.Sum256(data)), nil
}

func GetChannelModelStabilityCandidates(group, name string) ([]*Channel, string, error) {
	channels, snapshot, err := modelStabilityChannelsTx(DB, group, name)
	if err != nil {
		return nil, "", err
	}
	if err := PopulateEffectiveModelRoutings(channels, group, name); err != nil {
		return nil, "", err
	}
	return channels, snapshot, nil
}

// Ranking, probe observations and schedule commit atomically under both version fences.
func CommitChannelModelStabilityRun(policy ChannelGroupStabilityPolicy, automatic bool, snapshot string, observations []ChannelModelRouting, patches []ChannelGroupRoutingPatch, update ChannelGroupStabilityRunUpdate, initialized bool) (bool, error) {
	err := DB.Transaction(func(tx *gorm.DB) error {
		parent, err := lockChannelGroupStabilityPolicyTx(tx, policy.Group)
		if err != nil {
			return err
		}
		child, err := lockChannelModelPolicyTx(tx, policy.Group, policy.Model)
		if err != nil {
			return err
		}
		if parent.ConfigVersion != policy.ParentConfigVersion || child.ConfigVersion != policy.ConfigVersion || (automatic && (!parent.Enabled || child.Paused)) {
			return ErrChannelGroupStabilityPolicyStale
		}
		if snapshot != "" {
			_, current, err := modelStabilityChannelsTx(tx, policy.Group, policy.Model)
			if err != nil {
				return err
			}
			if current != snapshot {
				return ErrChannelGroupStabilityPolicyStale
			}
		}
		if len(patches) > 0 {
			if _, err := updateChannelModelRoutingTx(tx, policy.Group, policy.Model, patches, ChannelGroupRoutingRerank); err != nil {
				return err
			}
		}
		for _, observation := range observations {
			r := ChannelModelRouting{Group: policy.Group, Model: policy.Model, ChannelId: observation.ChannelId}
			err := tx.Where("channel_id = ? AND "+commonGroupCol+" = ? AND model = ?", r.ChannelId, r.Group, r.Model).First(&r).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			r.LastCheckAt, r.LatencyMs, r.Result, r.Message = observation.LastCheckAt, observation.LatencyMs, observation.Result, observation.Message
			if err := tx.Save(&r).Error; err != nil {
				return err
			}
		}
		child.Initialized = child.Initialized || initialized
		child.LastCheckAt, child.NextCheckAt = update.LastCheckAt, update.NextCheckAt
		child.LastResult, child.LastMessage = update.LastResult, update.LastMessage
		child.LastPrimaryChannelId, child.LastPrimaryLatencyMs = update.LastPrimaryChannelId, update.LastPrimaryLatencyMs
		child.LastReorderedAt = update.LastReorderedAt
		return tx.Save(child).Error
	})
	return err == nil, err
}
