package service

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

const GroupSpecialUsableGroupOptionKey = "group_ratio_setting.group_special_usable_group"

const native4KImageGroup = "生图分组-image2-4k(原生)"

type GroupSettingsValues struct {
	UserGroups              string `json:"UserGroups"`
	GroupRatio              string `json:"GroupRatio"`
	TopupGroupRatio         string `json:"TopupGroupRatio"`
	UserUsableGroups        string `json:"UserUsableGroups"`
	GroupGroupRatio         string `json:"GroupGroupRatio"`
	ImageSizeGroupPrices    string `json:"ImageSizeGroupPrices"`
	AutoGroups              string `json:"AutoGroups"`
	DefaultUseAutoGroup     bool   `json:"DefaultUseAutoGroup"`
	GroupSpecialUsableGroup string `json:"GroupSpecialUsableGroup"`
}

type parsedGroupSettings struct {
	userGroups       map[string]string
	billingGroups    map[string]float64
	topupRatios      map[string]float64
	selectableGroups map[string]string
	groupRatios      map[string]map[string]float64
	imageSizePrices  ratio_setting.ImageSizeGroupPrices
	autoGroups       []string
	specialGroups    map[string]map[string]string
}

func CurrentGroupSettingsValues() GroupSettingsValues {
	return GroupSettingsValues{
		UserGroups:              setting.UserGroups2JSONString(),
		GroupRatio:              ratio_setting.GroupRatio2JSONString(),
		TopupGroupRatio:         common.TopupGroupRatio2JSONString(),
		UserUsableGroups:        setting.UserUsableGroups2JSONString(),
		GroupGroupRatio:         ratio_setting.GroupGroupRatio2JSONString(),
		ImageSizeGroupPrices:    ratio_setting.ImageSizeGroupPrices2JSONString(),
		AutoGroups:              setting.AutoGroups2JsonString(),
		DefaultUseAutoGroup:     setting.DefaultUseAutoGroup,
		GroupSpecialUsableGroup: ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.MarshalJSONString(),
	}
}

func validateGroupIdentifier(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("group name cannot be empty")
	}
	if name != strings.TrimSpace(name) {
		return fmt.Errorf("group name cannot start or end with whitespace: %q", name)
	}
	if utf8.RuneCountInString(name) > setting.MaxUserGroupNameLength {
		return fmt.Errorf("group name is longer than %d characters: %s", setting.MaxUserGroupNameLength, name)
	}
	return nil
}

func parseGroupSettings(values GroupSettingsValues) (*parsedGroupSettings, error) {
	userGroups, err := setting.ParseUserGroupsJSONString(values.UserGroups)
	if err != nil {
		return nil, fmt.Errorf("invalid user groups: %w", err)
	}

	parsed := &parsedGroupSettings{userGroups: userGroups}
	if err := common.UnmarshalJsonStr(values.GroupRatio, &parsed.billingGroups); err != nil {
		return nil, fmt.Errorf("invalid billing groups: %w", err)
	}
	if err := common.UnmarshalJsonStr(values.TopupGroupRatio, &parsed.topupRatios); err != nil {
		return nil, fmt.Errorf("invalid top-up group ratios: %w", err)
	}
	if err := common.UnmarshalJsonStr(values.UserUsableGroups, &parsed.selectableGroups); err != nil {
		return nil, fmt.Errorf("invalid user-selectable billing groups: %w", err)
	}
	if err := common.UnmarshalJsonStr(values.GroupGroupRatio, &parsed.groupRatios); err != nil {
		return nil, fmt.Errorf("invalid inter-group ratios: %w", err)
	}
	parsed.imageSizePrices, err = ratio_setting.ParseImageSizeGroupPricesJSONString(values.ImageSizeGroupPrices)
	if err != nil {
		return nil, fmt.Errorf("invalid image size group prices: %w", err)
	}
	if err := common.UnmarshalJsonStr(values.AutoGroups, &parsed.autoGroups); err != nil {
		return nil, fmt.Errorf("invalid auto groups: %w", err)
	}
	if err := common.UnmarshalJsonStr(values.GroupSpecialUsableGroup, &parsed.specialGroups); err != nil {
		return nil, fmt.Errorf("invalid special usable group rules: %w", err)
	}
	return parsed, nil
}

func validateParsedGroupSettings(parsed *parsedGroupSettings) error {
	for name, ratio := range parsed.billingGroups {
		if err := validateGroupIdentifier(name); err != nil {
			return fmt.Errorf("invalid billing group: %w", err)
		}
		if ratio < 0 {
			return fmt.Errorf("billing group ratio cannot be negative: %s", name)
		}
	}

	for name, ratio := range parsed.topupRatios {
		if _, ok := parsed.userGroups[name]; !ok {
			return fmt.Errorf("top-up ratio references an unregistered user group: %s", name)
		}
		if ratio < 0 {
			return fmt.Errorf("top-up ratio cannot be negative: %s", name)
		}
	}

	for name := range parsed.selectableGroups {
		if _, ok := parsed.billingGroups[name]; !ok {
			return fmt.Errorf("user-selectable group is not a billing group: %s", name)
		}
	}

	for userGroup, overrides := range parsed.groupRatios {
		if _, ok := parsed.userGroups[userGroup]; !ok {
			return fmt.Errorf("ratio override references an unregistered user group: %s", userGroup)
		}
		for billingGroup, ratio := range overrides {
			if _, ok := parsed.billingGroups[billingGroup]; !ok {
				return fmt.Errorf("ratio override target is not a billing group: %s", billingGroup)
			}
			if ratio < 0 {
				return fmt.Errorf("ratio override cannot be negative: %s -> %s", userGroup, billingGroup)
			}
		}
	}

	for userGroup, usingGroups := range parsed.imageSizePrices {
		if _, ok := parsed.userGroups[userGroup]; !ok {
			return fmt.Errorf("image size price references an unregistered user group: %s", userGroup)
		}
		for billingGroup, models := range usingGroups {
			if _, ok := parsed.billingGroups[billingGroup]; !ok {
				return fmt.Errorf("image size price target is not a billing group: %s", billingGroup)
			}
			if billingGroup != native4KImageGroup {
				continue
			}
			for _, tiers := range models {
				for tier := range tiers {
					if tier != "4K" {
						return fmt.Errorf("native 4K image group only supports the 4K price tier: %s", tier)
					}
				}
			}
		}
	}

	seenAutoGroups := make(map[string]struct{}, len(parsed.autoGroups))
	for _, billingGroup := range parsed.autoGroups {
		if _, ok := parsed.billingGroups[billingGroup]; !ok {
			return fmt.Errorf("auto group is not a billing group: %s", billingGroup)
		}
		if _, exists := seenAutoGroups[billingGroup]; exists {
			return fmt.Errorf("auto group is duplicated: %s", billingGroup)
		}
		seenAutoGroups[billingGroup] = struct{}{}
	}

	for userGroup, rules := range parsed.specialGroups {
		if _, ok := parsed.userGroups[userGroup]; !ok {
			return fmt.Errorf("special usable rule references an unregistered user group: %s", userGroup)
		}
		for rawBillingGroup := range rules {
			billingGroup := strings.TrimPrefix(strings.TrimPrefix(rawBillingGroup, "+:"), "-:")
			if _, ok := parsed.billingGroups[billingGroup]; !ok {
				return fmt.Errorf("special usable rule target is not a billing group: %s", billingGroup)
			}
		}
	}

	return nil
}

func validateRemovedUserGroups(current, next map[string]string) error {
	usageCounts, err := model.GetUserGroupUsageCounts()
	if err != nil {
		return err
	}
	for name := range current {
		if _, remains := next[name]; remains {
			continue
		}
		if count := usageCounts[name]; count > 0 {
			return fmt.Errorf("cannot delete user group %s: %d users still reference it", name, count)
		}
	}
	return nil
}

func ValidateGroupSettings(values GroupSettingsValues) error {
	parsed, err := parseGroupSettings(values)
	if err != nil {
		return err
	}
	return validateParsedGroupSettings(parsed)
}

func UpdateGroupSettings(values GroupSettingsValues) error {
	parsed, err := parseGroupSettings(values)
	if err != nil {
		return err
	}
	if err := validateParsedGroupSettings(parsed); err != nil {
		return err
	}
	if err := validateRemovedUserGroups(setting.GetUserGroupsCopy(), parsed.userGroups); err != nil {
		return err
	}

	return model.UpdateOptionsBulk(map[string]string{
		"UserGroups":                     values.UserGroups,
		"GroupRatio":                     values.GroupRatio,
		"TopupGroupRatio":                values.TopupGroupRatio,
		"UserUsableGroups":               values.UserUsableGroups,
		"GroupGroupRatio":                values.GroupGroupRatio,
		"ImageSizeGroupPrices":           normalizedImageSizeGroupPrices(values.ImageSizeGroupPrices),
		"AutoGroups":                     values.AutoGroups,
		"DefaultUseAutoGroup":            common.Interface2String(values.DefaultUseAutoGroup),
		GroupSpecialUsableGroupOptionKey: values.GroupSpecialUsableGroup,
	})
}

func IsGroupSettingsOptionKey(key string) bool {
	switch key {
	case "UserGroups", "GroupRatio", "TopupGroupRatio", "UserUsableGroups", "GroupGroupRatio", "ImageSizeGroupPrices", "AutoGroups", GroupSpecialUsableGroupOptionKey:
		return true
	default:
		return false
	}
}

func ValidateGroupOptionUpdate(key, value string) error {
	if !IsGroupSettingsOptionKey(key) {
		return nil
	}

	current := CurrentGroupSettingsValues()
	switch key {
	case "UserGroups":
		current.UserGroups = value
	case "GroupRatio":
		current.GroupRatio = value
	case "TopupGroupRatio":
		current.TopupGroupRatio = value
	case "UserUsableGroups":
		current.UserUsableGroups = value
	case "GroupGroupRatio":
		current.GroupGroupRatio = value
	case "ImageSizeGroupPrices":
		current.ImageSizeGroupPrices = value
	case "AutoGroups":
		current.AutoGroups = value
	case GroupSpecialUsableGroupOptionKey:
		current.GroupSpecialUsableGroup = value
	}

	parsed, err := parseGroupSettings(current)
	if err != nil {
		return err
	}
	if err := validateParsedGroupSettings(parsed); err != nil {
		return err
	}
	if key == "UserGroups" {
		return validateRemovedUserGroups(setting.GetUserGroupsCopy(), parsed.userGroups)
	}
	return nil
}

func normalizedImageSizeGroupPrices(value string) string {
	if strings.TrimSpace(value) == "" {
		return "{}"
	}
	return value
}
