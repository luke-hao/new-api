package controller

import (
	"net/http"
	"sort"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

func GetGroups(c *gin.Context) {
	groupNames := make([]string, 0)
	for groupName := range ratio_setting.GetGroupRatioCopy() {
		groupNames = append(groupNames, groupName)
	}
	sort.Strings(groupNames)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    groupNames,
	})
}

type adminUserGroupInfo struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	UserCount    int64  `json:"user_count"`
	BillingGroup bool   `json:"billing_group"`
}

func GetAdminUserGroups(c *gin.Context) {
	usageCounts, err := model.GetUserGroupUsageCounts()
	if err != nil {
		common.ApiError(c, err)
		return
	}

	groups := setting.GetUserGroupsCopy()
	result := make([]adminUserGroupInfo, 0, len(groups))
	for name, description := range groups {
		result = append(result, adminUserGroupInfo{
			Name:         name,
			Description:  description,
			UserCount:    usageCounts[name],
			BillingGroup: ratio_setting.ContainsGroupRatio(name),
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    result,
	})
}

func UpdateGroupSettings(c *gin.Context) {
	var values service.GroupSettingsValues
	if err := common.DecodeJson(c.Request.Body, &values); err != nil {
		common.ApiErrorMsg(c, "invalid group settings payload")
		return
	}
	if err := service.UpdateGroupSettings(values); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	recordManageAudit(c, "group.settings.update", map[string]interface{}{
		"user_group_count":    len(setting.GetUserGroupsCopy()),
		"billing_group_count": len(ratio_setting.GetGroupRatioCopy()),
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func GetUserGroups(c *gin.Context) {
	usableGroups := make(map[string]map[string]interface{})
	userGroup := ""
	userId := c.GetInt("id")
	userGroup, _ = model.GetUserGroup(userId, false)
	userUsableGroups := service.GetUserUsableGroups(userGroup)
	for groupName, _ := range ratio_setting.GetGroupRatioCopy() {
		// UserUsableGroups contains the groups that the user can use
		if desc, ok := userUsableGroups[groupName]; ok {
			usableGroups[groupName] = map[string]interface{}{
				"ratio": service.GetUserGroupRatio(userGroup, groupName),
				"desc":  desc,
			}
		}
	}
	if _, ok := userUsableGroups["auto"]; ok {
		usableGroups["auto"] = map[string]interface{}{
			"ratio": "自动",
			"desc":  setting.GetUsableGroupDescription("auto"),
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    usableGroups,
	})
}
