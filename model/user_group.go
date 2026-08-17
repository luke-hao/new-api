package model

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

type UserGroupUsage struct {
	Name      string `json:"name" gorm:"column:name"`
	UserCount int64  `json:"user_count" gorm:"column:user_count"`
}

func GetUserGroupUsageCounts() (map[string]int64, error) {
	rows := make([]UserGroupUsage, 0)
	err := DB.Unscoped().Model(&User{}).
		Select(commonGroupCol+" AS name, COUNT(*) AS user_count").
		Where(commonGroupCol+" <> ?", "").
		Group("group").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	counts := make(map[string]int64, len(rows))
	for _, row := range rows {
		counts[row.Name] = row.UserCount
	}
	return counts, nil
}

func bootstrapUserGroupsOption() error {
	if DB == nil {
		return fmt.Errorf("database is not initialized")
	}

	var optionCount int64
	if err := DB.Model(&Option{}).
		Where(commonKeyCol+" = ?", "UserGroups").
		Count(&optionCount).Error; err != nil {
		return err
	}
	if optionCount > 0 {
		return nil
	}

	groups := setting.GetUserGroupsCopy()
	usableDescriptions := setting.GetUserUsableGroupsCopy()
	addGroup := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		if _, exists := groups[name]; exists {
			return
		}
		if description, ok := usableDescriptions[name]; ok && strings.TrimSpace(description) != "" {
			groups[name] = description
			return
		}
		groups[name] = name
	}

	usageCounts, err := GetUserGroupUsageCounts()
	if err != nil {
		return err
	}
	for name := range usageCounts {
		addGroup(name)
	}

	topupRatios := make(map[string]float64)
	if err := common.UnmarshalJsonStr(common.TopupGroupRatio2JSONString(), &topupRatios); err != nil {
		return err
	}
	for name := range topupRatios {
		addGroup(name)
	}

	for name := range ratio_setting.GetGroupGroupRatioCopy() {
		addGroup(name)
	}
	for name := range ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.ReadAll() {
		addGroup(name)
	}

	if err := setting.ValidateUserGroups(groups); err != nil {
		return err
	}
	jsonBytes, err := common.Marshal(groups)
	if err != nil {
		return err
	}
	return UpdateOption("UserGroups", string(jsonBytes))
}
