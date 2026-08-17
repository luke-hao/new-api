package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/require"
)

func mustJSONString(t *testing.T, value any) string {
	t.Helper()
	data, err := common.Marshal(value)
	require.NoError(t, err)
	return string(data)
}

func validGroupSettings(t *testing.T) GroupSettingsValues {
	t.Helper()
	return GroupSettingsValues{
		UserGroups: mustJSONString(t, map[string]string{
			"default": "Default users",
			"vip":     "VIP users",
		}),
		GroupRatio: mustJSONString(t, map[string]float64{
			"default": 1,
			"vip":     0.8,
			"premium": 0.5,
		}),
		TopupGroupRatio: mustJSONString(t, map[string]float64{
			"default": 1,
			"vip":     1.2,
		}),
		UserUsableGroups: mustJSONString(t, map[string]string{
			"default": "Standard billing",
			"premium": "Premium billing",
		}),
		GroupGroupRatio: mustJSONString(t, map[string]map[string]float64{
			"vip": {"premium": 0.3},
		}),
		ImageSizeGroupPrices: mustJSONString(t, ratio_setting.ImageSizeGroupPrices{
			"vip": {
				"premium": {
					"gpt-image-2": {"1K": 0.05, "2K": 0.11, "4K": 0.17},
				},
			},
		}),
		AutoGroups:          mustJSONString(t, []string{"default", "premium"}),
		DefaultUseAutoGroup: true,
		GroupSpecialUsableGroup: mustJSONString(t, map[string]map[string]string{
			"vip": {"+:premium": "Premium billing"},
		}),
	}
}

func TestValidateGroupSettingsValidatesImageSizePrices(t *testing.T) {
	values := validGroupSettings(t)
	values.ImageSizeGroupPrices = mustJSONString(t, ratio_setting.ImageSizeGroupPrices{
		"missing": {"premium": {"gpt-image-2": {"4K": 0.17}}},
	})
	require.ErrorContains(t, ValidateGroupSettings(values), "unregistered user group")

	values = validGroupSettings(t)
	values.ImageSizeGroupPrices = mustJSONString(t, ratio_setting.ImageSizeGroupPrices{
		"vip": {"missing": {"gpt-image-2": {"4K": 0.17}}},
	})
	require.ErrorContains(t, ValidateGroupSettings(values), "not a billing group")

	values = validGroupSettings(t)
	values.GroupRatio = mustJSONString(t, map[string]float64{
		"default":          1,
		"vip":              0.8,
		"premium":          0.5,
		native4KImageGroup: 1,
	})
	values.ImageSizeGroupPrices = mustJSONString(t, ratio_setting.ImageSizeGroupPrices{
		"vip": {native4KImageGroup: {"gpt-image-2": {"2K": 0.11}}},
	})
	require.ErrorContains(t, ValidateGroupSettings(values), "only supports the 4K price tier")
}

func TestValidateGroupSettingsAllowsSameNameInBothRoles(t *testing.T) {
	values := validGroupSettings(t)
	require.NoError(t, ValidateGroupSettings(values))
}

func TestValidateGroupSettingsRequiresUserGroupSources(t *testing.T) {
	values := validGroupSettings(t)
	values.TopupGroupRatio = mustJSONString(t, map[string]float64{"premium": 1.1})
	require.ErrorContains(t, ValidateGroupSettings(values), "unregistered user group")
}

func TestValidateGroupSettingsRequiresBillingTargets(t *testing.T) {
	values := validGroupSettings(t)
	values.GroupGroupRatio = mustJSONString(t, map[string]map[string]float64{
		"vip": {"user-only": 0.2},
	})
	require.ErrorContains(t, ValidateGroupSettings(values), "not a billing group")
}

func TestValidateGroupSettingsConstrainsAutoAndSpecialGroups(t *testing.T) {
	values := validGroupSettings(t)
	values.AutoGroups = mustJSONString(t, []string{"default", "missing"})
	require.ErrorContains(t, ValidateGroupSettings(values), "auto group is not a billing group")

	values = validGroupSettings(t)
	values.GroupSpecialUsableGroup = mustJSONString(t, map[string]map[string]string{
		"vip": {"+:missing": "Missing"},
	})
	require.ErrorContains(t, ValidateGroupSettings(values), "target is not a billing group")
}
