package controller

import (
	"net/http"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

type playgroundImageModelCapability struct {
	Model          string   `json:"model"`
	Protocol       string   `json:"protocol"`
	Profile        string   `json:"profile"`
	Modes          []string `json:"modes"`
	FixedImageSize string   `json:"fixed_image_size,omitempty"`
}

type playgroundImageGroupCapability struct {
	Group  string                           `json:"group"`
	Desc   string                           `json:"desc"`
	Ratio  any                              `json:"ratio"`
	Models []playgroundImageModelCapability `json:"models"`
}

func getPlaygroundImageModelCapability(modelName string) (playgroundImageModelCapability, bool) {
	value := strings.ToLower(strings.TrimSpace(modelName))
	capability := playgroundImageModelCapability{
		Model: modelName,
		Modes: []string{"generate", "edit"},
	}

	if model_setting.IsGeminiModelSupportImagine(modelName) {
		capability.Protocol = "gemini_chat"
		capability.Profile = "gemini_image"
		switch {
		case strings.HasSuffix(value, "-2k"):
			capability.FixedImageSize = "2K"
		case strings.HasSuffix(value, "-4k"):
			capability.FixedImageSize = "4K"
		}
		return capability, true
	}

	if value == "gpt-image-2" || strings.HasPrefix(value, "gpt-image-2-") {
		capability.Protocol = "image_api"
		capability.Profile = "gpt_image_2"
		return capability, true
	}

	if strings.Contains(value, "gpt-image-") ||
		strings.Contains(value, "chatgpt-image") ||
		strings.Contains(value, "dall-e") ||
		strings.HasPrefix(value, "imagen-") ||
		strings.Contains(value, "imagen-") ||
		strings.HasPrefix(value, "flux") ||
		strings.Contains(value, "seedream") {
		capability.Protocol = "image_api"
		capability.Profile = "legacy_image"
		return capability, true
	}

	return playgroundImageModelCapability{}, false
}

func collectPlaygroundImageModels(modelNames []string) []playgroundImageModelCapability {
	models := make([]playgroundImageModelCapability, 0, len(modelNames))
	seen := make(map[string]struct{}, len(modelNames))
	for _, modelName := range modelNames {
		if _, ok := seen[modelName]; ok {
			continue
		}
		capability, ok := getPlaygroundImageModelCapability(modelName)
		if !ok {
			continue
		}
		seen[modelName] = struct{}{}
		models = append(models, capability)
	}
	sort.Slice(models, func(i, j int) bool {
		profileRank := func(profile string) int {
			switch profile {
			case "gpt_image_2":
				return 0
			case "gemini_image":
				return 1
			default:
				return 2
			}
		}
		leftRank := profileRank(models[i].Profile)
		rightRank := profileRank(models[j].Profile)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		return models[i].Model < models[j].Model
	})
	return models
}

func GetPlaygroundImageCapabilities(c *gin.Context) {
	user, err := model.GetUserCache(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}

	usableGroups := service.GetUserUsableGroups(user.Group)
	visibleGroups := make([]string, 0, len(usableGroups))
	for groupName := range ratio_setting.GetGroupRatioCopy() {
		if _, ok := usableGroups[groupName]; ok {
			visibleGroups = append(visibleGroups, groupName)
		}
	}
	if _, ok := usableGroups["auto"]; ok {
		visibleGroups = append(visibleGroups, "auto")
	}
	sort.Strings(visibleGroups)

	queryGroups := make([]string, 0, len(visibleGroups))
	queryGroupSet := make(map[string]struct{}, len(visibleGroups))
	addQueryGroup := func(groupName string) {
		if groupName == "" || groupName == "auto" {
			return
		}
		if _, ok := queryGroupSet[groupName]; ok {
			return
		}
		queryGroupSet[groupName] = struct{}{}
		queryGroups = append(queryGroups, groupName)
	}
	for _, groupName := range visibleGroups {
		addQueryGroup(groupName)
	}
	autoGroups := service.GetUserAutoGroup(user.Group)
	for _, groupName := range autoGroups {
		addQueryGroup(groupName)
	}

	rows, err := model.GetGroupsEnabledModels(queryGroups)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	modelsByGroup := make(map[string][]string, len(queryGroups))
	for _, row := range rows {
		modelsByGroup[row.Group] = append(modelsByGroup[row.Group], row.Model)
	}

	groups := make([]playgroundImageGroupCapability, 0, len(visibleGroups))
	for _, groupName := range visibleGroups {
		modelNames := modelsByGroup[groupName]
		desc := usableGroups[groupName]
		var ratio any = service.GetUserGroupRatio(user.Group, groupName)
		if groupName == "auto" {
			modelNames = nil
			for _, autoGroup := range autoGroups {
				modelNames = append(modelNames, modelsByGroup[autoGroup]...)
			}
			desc = setting.GetUsableGroupDescription("auto")
			ratio = "自动"
		}

		imageModels := collectPlaygroundImageModels(modelNames)
		if len(imageModels) == 0 {
			continue
		}
		groups = append(groups, playgroundImageGroupCapability{
			Group:  groupName,
			Desc:   desc,
			Ratio:  ratio,
			Models: imageModels,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    groups,
	})
}
