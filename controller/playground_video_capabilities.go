package controller

import (
	"net/http"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

const (
	playgroundVideoModeText      = "text"
	playgroundVideoModeImage     = "image"
	playgroundVideoModeFirstLast = "first_last"
)

type playgroundVideoParameters struct {
	Durations          []int    `json:"durations,omitempty"`
	AspectRatios       []string `json:"aspect_ratios,omitempty"`
	Resolutions        []string `json:"resolutions,omitempty"`
	SupportsSeed       bool     `json:"supports_seed"`
	MaxInputReferences int      `json:"max_input_references"`
}

type playgroundVideoModelCapability struct {
	Model      string                    `json:"model"`
	Profile    string                    `json:"profile"`
	Modes      []string                  `json:"modes"`
	Parameters playgroundVideoParameters `json:"parameters"`
}

type playgroundVideoGroupCapability struct {
	Group  string                           `json:"group"`
	Desc   string                           `json:"desc"`
	Ratio  any                              `json:"ratio"`
	Models []playgroundVideoModelCapability `json:"models"`
}

func resolvePlaygroundMappedModel(modelName, rawMapping string) (string, bool) {
	if strings.TrimSpace(rawMapping) == "" || strings.TrimSpace(rawMapping) == "{}" {
		return modelName, true
	}
	mapping := map[string]string{}
	if err := common.Unmarshal([]byte(rawMapping), &mapping); err != nil {
		return "", false
	}

	current := modelName
	visited := map[string]bool{current: true}
	for {
		next, ok := mapping[current]
		if !ok || strings.TrimSpace(next) == "" || next == current {
			return current, true
		}
		if visited[next] {
			return "", false
		}
		visited[next] = true
		current = next
	}
}

func videoModesForModel(channelType int, modelName string) []string {
	name := strings.ToLower(strings.TrimSpace(modelName))
	// A channel can expose both image and video abilities. Keep image-only
	// models out of the video workspace even when the channel type itself also
	// supports video (for example Gemini/Veo or OpenAI/Sora channels).
	if _, imageCapability := getPlaygroundImageModelCapability(modelName); imageCapability {
		return nil
	}
	switch channelType {
	case constant.ChannelTypeAli:
		switch {
		case strings.Contains(name, "flf2v"), strings.Contains(name, "first-last"), strings.Contains(name, "first_tail"):
			return []string{playgroundVideoModeFirstLast}
		case strings.Contains(name, "i2v"):
			return []string{playgroundVideoModeImage}
		case strings.Contains(name, "t2v"):
			return []string{playgroundVideoModeText}
		default:
			return []string{playgroundVideoModeText, playgroundVideoModeImage, playgroundVideoModeFirstLast}
		}
	case constant.ChannelTypeGemini, constant.ChannelTypeVertexAi, constant.ChannelTypeSora, constant.ChannelTypeOpenAI:
		return []string{playgroundVideoModeText, playgroundVideoModeImage}
	case constant.ChannelTypeMiniMax:
		switch {
		case strings.HasPrefix(name, "t2v-"):
			return []string{playgroundVideoModeText}
		case strings.HasPrefix(name, "i2v-"):
			return []string{playgroundVideoModeImage, playgroundVideoModeFirstLast}
		case strings.HasPrefix(name, "s2v-"):
			return []string{playgroundVideoModeImage}
		default:
			return []string{playgroundVideoModeText, playgroundVideoModeImage, playgroundVideoModeFirstLast}
		}
	case constant.ChannelTypeKling, constant.ChannelTypeJimeng, constant.ChannelTypeVidu,
		constant.ChannelTypeDoubaoVideo, constant.ChannelTypeVolcEngine:
		return []string{playgroundVideoModeText, playgroundVideoModeImage, playgroundVideoModeFirstLast}
	default:
		return nil
	}
}

func getPlaygroundVideoModelCapability(channelType int, modelName string) (playgroundVideoModelCapability, bool) {
	modes := videoModesForModel(channelType, modelName)
	if len(modes) == 0 {
		return playgroundVideoModelCapability{}, false
	}

	capability := playgroundVideoModelCapability{
		Model:   modelName,
		Profile: strings.ToLower(constant.GetChannelTypeName(channelType)),
		Modes:   modes,
		Parameters: playgroundVideoParameters{
			MaxInputReferences: 2,
		},
	}

	switch channelType {
	case constant.ChannelTypeAli:
		capability.Parameters.Durations = []int{5, 10}
		capability.Parameters.AspectRatios = []string{"16:9", "9:16", "1:1"}
		capability.Parameters.Resolutions = []string{"480p", "720p", "1080p"}
		capability.Parameters.SupportsSeed = true
	case constant.ChannelTypeKling:
		capability.Parameters.Durations = []int{5, 10}
		capability.Parameters.AspectRatios = []string{"16:9", "9:16", "1:1"}
	case constant.ChannelTypeJimeng:
		capability.Parameters.Durations = []int{5, 10}
		capability.Parameters.AspectRatios = []string{"16:9", "9:16", "1:1", "4:3", "3:4"}
		capability.Parameters.SupportsSeed = true
	case constant.ChannelTypeVidu:
		capability.Parameters.Durations = []int{4, 5, 8}
		capability.Parameters.Resolutions = []string{"720p", "1080p"}
		capability.Parameters.SupportsSeed = true
	case constant.ChannelTypeDoubaoVideo, constant.ChannelTypeVolcEngine:
		capability.Parameters.Durations = []int{5, 10}
		capability.Parameters.AspectRatios = []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9"}
		capability.Parameters.Resolutions = []string{"480p", "720p", "1080p"}
		capability.Parameters.SupportsSeed = true
	case constant.ChannelTypeGemini, constant.ChannelTypeVertexAi:
		capability.Parameters.Durations = []int{4, 5, 6, 8}
		capability.Parameters.AspectRatios = []string{"16:9", "9:16"}
		capability.Parameters.Resolutions = []string{"720p", "1080p"}
		capability.Parameters.MaxInputReferences = 1
	case constant.ChannelTypeSora, constant.ChannelTypeOpenAI:
		capability.Parameters.Durations = []int{4, 8, 12}
		capability.Parameters.AspectRatios = []string{"16:9", "9:16"}
		capability.Parameters.Resolutions = []string{"720p", "1080p"}
		capability.Parameters.MaxInputReferences = 1
	case constant.ChannelTypeMiniMax:
		capability.Parameters.Durations = []int{6, 10}
		capability.Parameters.Resolutions = []string{"720p", "768p", "1080p"}
	}

	return capability, true
}

func intersectStrings(left, right []string) []string {
	allowed := make(map[string]struct{}, len(right))
	for _, value := range right {
		allowed[value] = struct{}{}
	}
	result := make([]string, 0, len(left))
	for _, value := range left {
		if _, ok := allowed[value]; ok {
			result = append(result, value)
		}
	}
	return result
}

func intersectInts(left, right []int) []int {
	allowed := make(map[int]struct{}, len(right))
	for _, value := range right {
		allowed[value] = struct{}{}
	}
	result := make([]int, 0, len(left))
	for _, value := range left {
		if _, ok := allowed[value]; ok {
			result = append(result, value)
		}
	}
	return result
}

func intersectVideoCapabilities(left, right playgroundVideoModelCapability) playgroundVideoModelCapability {
	left.Modes = intersectStrings(left.Modes, right.Modes)
	left.Parameters.Durations = intersectInts(left.Parameters.Durations, right.Parameters.Durations)
	left.Parameters.AspectRatios = intersectStrings(left.Parameters.AspectRatios, right.Parameters.AspectRatios)
	left.Parameters.Resolutions = intersectStrings(left.Parameters.Resolutions, right.Parameters.Resolutions)
	left.Parameters.SupportsSeed = left.Parameters.SupportsSeed && right.Parameters.SupportsSeed
	if right.Parameters.MaxInputReferences < left.Parameters.MaxInputReferences {
		left.Parameters.MaxInputReferences = right.Parameters.MaxInputReferences
	}
	if left.Profile != right.Profile {
		left.Profile = "mixed"
	}
	return left
}

func collectPlaygroundVideoModels(abilities []model.AbilityWithChannel) []playgroundVideoModelCapability {
	byModel := make(map[string]playgroundVideoModelCapability)
	for _, ability := range abilities {
		mappedModel, ok := resolvePlaygroundMappedModel(ability.Model, ability.ChannelModelMapping)
		if !ok {
			continue
		}
		candidate, ok := getPlaygroundVideoModelCapability(ability.ChannelType, mappedModel)
		if !ok {
			continue
		}
		candidate.Model = ability.Model
		if current, exists := byModel[ability.Model]; exists {
			byModel[ability.Model] = intersectVideoCapabilities(current, candidate)
		} else {
			byModel[ability.Model] = candidate
		}
	}

	models := make([]playgroundVideoModelCapability, 0, len(byModel))
	for _, capability := range byModel {
		if len(capability.Modes) > 0 {
			models = append(models, capability)
		}
	}
	sort.Slice(models, func(i, j int) bool { return models[i].Model < models[j].Model })
	return models
}

func GetPlaygroundVideoCapabilities(c *gin.Context) {
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

	rows, err := model.GetGroupsEnabledAbilitiesWithChannels(queryGroups)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	abilitiesByGroup := make(map[string][]model.AbilityWithChannel, len(queryGroups))
	for _, row := range rows {
		abilitiesByGroup[row.Group] = append(abilitiesByGroup[row.Group], row)
	}

	groups := make([]playgroundVideoGroupCapability, 0, len(visibleGroups))
	for _, groupName := range visibleGroups {
		groupAbilities := abilitiesByGroup[groupName]
		desc := usableGroups[groupName]
		var ratio any = service.GetUserGroupRatio(user.Group, groupName)
		if groupName == "auto" {
			groupAbilities = nil
			for _, autoGroup := range autoGroups {
				groupAbilities = append(groupAbilities, abilitiesByGroup[autoGroup]...)
			}
			desc = setting.GetUsableGroupDescription("auto")
			ratio = "自动"
		}

		videoModels := collectPlaygroundVideoModels(groupAbilities)
		if len(videoModels) == 0 {
			continue
		}
		groups = append(groups, playgroundVideoGroupCapability{
			Group:  groupName,
			Desc:   desc,
			Ratio:  ratio,
			Models: videoModels,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    groups,
	})
}
