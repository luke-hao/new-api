package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/require"
)

func TestBootstrapUserGroupsOptionIsCompleteAndIdempotent(t *testing.T) {
	truncateTables(t)

	originalUserGroups := setting.UserGroups2JSONString()
	originalTopupRatios := common.TopupGroupRatio2JSONString()
	originalGroupRatios := ratio_setting.GroupGroupRatio2JSONString()
	originalSpecialGroups := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.ReadAll()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserGroupsByJSONString(originalUserGroups))
		require.NoError(t, common.UpdateTopupGroupRatioByJSONString(originalTopupRatios))
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(originalGroupRatios))
		ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.Clear()
		ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.AddAll(originalSpecialGroups)
	})

	require.NoError(t, setting.UpdateUserGroupsByJSONString(`{"default":"Default users"}`))
	require.NoError(t, common.UpdateTopupGroupRatioByJSONString(`{"default":1,"topup-source":1.2}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"override-source":{"billing":0.5}}`))
	ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.Clear()
	ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.AddAll(map[string]map[string]string{
		"special-source": {"+:billing": "Billing"},
	})

	activeUser := &User{Username: "active-user", Group: "active-source", AffCode: "active-aff"}
	deletedUser := &User{Username: "deleted-user", Group: "deleted-source", AffCode: "deleted-aff"}
	require.NoError(t, DB.Create(activeUser).Error)
	require.NoError(t, DB.Create(deletedUser).Error)
	require.NoError(t, DB.Delete(deletedUser).Error)

	require.NoError(t, bootstrapUserGroupsOption())

	var option Option
	require.NoError(t, DB.Where(commonKeyCol+" = ?", "UserGroups").First(&option).Error)
	var groups map[string]string
	require.NoError(t, common.UnmarshalJsonStr(option.Value, &groups))
	for _, name := range []string{
		"default",
		"active-source",
		"deleted-source",
		"topup-source",
		"override-source",
		"special-source",
	} {
		require.Contains(t, groups, name)
	}

	require.NoError(t, DB.Create(&User{Username: "later-user", Group: "later-source", AffCode: "later-aff"}).Error)
	require.NoError(t, bootstrapUserGroupsOption())
	require.NoError(t, DB.Where(commonKeyCol+" = ?", "UserGroups").First(&option).Error)
	require.NoError(t, common.UnmarshalJsonStr(option.Value, &groups))
	require.NotContains(t, groups, "later-source")
}
