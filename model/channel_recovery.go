package model

import (
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ChannelRecoveryState struct {
	KeyIndex   int    `json:"key_index"`
	Reason     string `json:"reason"`
	LastTestAt int64  `json:"last_test_at"`
	NextTestAt int64  `json:"next_test_at"`
	Attempts   int    `json:"attempts"`
	Successes  int    `json:"successes"`
	LastError  string `json:"last_error,omitempty"`
}

func GetChannelRecoveryState(channel *Channel) ChannelRecoveryState {
	var state ChannelRecoveryState
	raw, _ := common.Marshal(channel.GetOtherInfo()["auto_recovery"])
	_ = common.Unmarshal(raw, &state)
	return state
}
func recoverySnapshot(channel *Channel) string {
	copy := *channel
	copy.TestTime, copy.ResponseTime, copy.UsedQuota = 0, 0, 0
	copy.Balance, copy.BalanceUpdatedTime = 0, 0
	copy.Keys = nil
	data, _ := common.Marshal(copy)
	return string(data)
}

// A database row lock plus snapshot comparison prevents late probes from
// undoing manual disables, credential edits, deletions, or newer disable events.
func SaveChannelRecovery(snapshot *Channel, state ChannelRecoveryState, enableKey *string) (*Channel, bool, error) {
	channelStatusLock.Lock()
	defer channelStatusLock.Unlock()
	metadataLock := getChannelOtherInfoLock(snapshot.Id)
	metadataLock.Lock()
	defer metadataLock.Unlock()
	var current Channel
	applied := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, snapshot.Id).Error; err != nil {
			return err
		}
		if current.Status != common.ChannelStatusAutoDisabled || recoverySnapshot(&current) != recoverySnapshot(snapshot) {
			return nil
		}
		info := current.GetOtherInfo()
		info["auto_recovery"] = state
		if enableKey != nil {
			if current.ChannelInfo.IsMultiKey {
				found := false
				for i, key := range current.GetKeys() {
					if key == *enableKey && current.ChannelInfo.MultiKeyStatusList[i] == common.ChannelStatusAutoDisabled {
						delete(current.ChannelInfo.MultiKeyStatusList, i)
						delete(current.ChannelInfo.MultiKeyDisabledReason, i)
						delete(current.ChannelInfo.MultiKeyDisabledTime, i)
						found = true
						break
					}
				}
				if !found {
					return nil
				}
			}
			current.Status = common.ChannelStatusEnabled
			info["status_reason"] = ""
			info["status_time"] = common.GetTimestamp()
			delete(info, "auto_disable_reason")
		}
		current.SetOtherInfo(info)
		result := tx.Model(&Channel{}).Where("id = ? AND status = ?", current.Id, common.ChannelStatusAutoDisabled).
			Updates(map[string]interface{}{"status": current.Status, "other_info": current.OtherInfo, "channel_info": current.ChannelInfo})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return nil
		}
		if enableKey != nil {
			if err := tx.Model(&Ability{}).Where("channel_id = ?", current.Id).Update("enabled", true).Error; err != nil {
				return err
			}
		}
		applied = true
		return nil
	})
	if err == nil && applied && enableKey != nil {
		InitChannelCache()
	}
	return &current, applied, err
}
