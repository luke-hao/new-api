package controller

import (
	"net/http"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const (
	playgroundVideoModeText      = "text"
	playgroundVideoModeImage     = "first_frame"
	playgroundVideoModeFirstLast = "first_last"
	playgroundVideoModeReference = "reference"
)

type playgroundVideoParameters struct {
	Durations          []int    `json:"durations,omitempty"`
	AspectRatios       []string `json:"aspect_ratios,omitempty"`
	Resolutions        []string `json:"resolutions,omitempty"`
	SupportsSeed       bool     `json:"supports_seed"`
	MaxInputReferences int      `json:"max_input_references"`
	MaxImageReferences int      `json:"max_image_references"`
	MaxVideoReferences int      `json:"max_video_references"`
	MaxAudioReferences int      `json:"max_audio_references"`
	MaxImageBytes      int64    `json:"max_image_bytes"`
	MaxVideoBytes      int64    `json:"max_video_bytes"`
	MaxAudioBytes      int64    `json:"max_audio_bytes"`
	MaxVideoEditBytes  int64    `json:"max_video_edit_bytes"`
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

func fullVideoModes() []string {
	return []string{playgroundVideoModeText, playgroundVideoModeImage, playgroundVideoModeFirstLast, playgroundVideoModeReference}
}

func makeIntRange(start, end int) []int {
	if end < start {
		return nil
	}
	values := make([]int, 0, end-start+1)
	for value := start; value <= end; value++ {
		values = append(values, value)
	}
	return values
}

func getPlaygroundVideoModelCapability(channelType int, modelName string) (playgroundVideoModelCapability, bool) {
	if len(videoModesForModel(channelType, modelName)) == 0 {
		return playgroundVideoModelCapability{}, false
	}
	profile, ok := common.GetPlaygroundVideoCapability(channelType, modelName)
	if !ok {
		return playgroundVideoModelCapability{}, false
	}

	capability := playgroundVideoModelCapability{
		Model:   modelName,
		Profile: strings.ToLower(constant.GetChannelTypeName(channelType)),
		Modes:   profile.Modes,
		Parameters: playgroundVideoParameters{
			Durations:          profile.Durations,
			AspectRatios:       profile.AspectRatios,
			Resolutions:        profile.Resolutions,
			SupportsSeed:       profile.SupportsSeed,
			MaxInputReferences: profile.MaxInputReferences,
			MaxImageReferences: profile.MaxImageReferences,
			MaxVideoReferences: profile.MaxVideoReferences,
			MaxAudioReferences: profile.MaxAudioReferences,
			MaxImageBytes:      profile.MaxImageBytes,
			MaxVideoBytes:      profile.MaxVideoBytes,
			MaxAudioBytes:      profile.MaxAudioBytes,
			MaxVideoEditBytes:  profile.MaxVideoEditBytes,
		},
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
	if right.Parameters.MaxImageReferences < left.Parameters.MaxImageReferences {
		left.Parameters.MaxImageReferences = right.Parameters.MaxImageReferences
	}
	if right.Parameters.MaxVideoReferences < left.Parameters.MaxVideoReferences {
		left.Parameters.MaxVideoReferences = right.Parameters.MaxVideoReferences
	}
	if right.Parameters.MaxAudioReferences < left.Parameters.MaxAudioReferences {
		left.Parameters.MaxAudioReferences = right.Parameters.MaxAudioReferences
	}
	if right.Parameters.MaxImageBytes < left.Parameters.MaxImageBytes {
		left.Parameters.MaxImageBytes = right.Parameters.MaxImageBytes
	}
	if right.Parameters.MaxVideoBytes < left.Parameters.MaxVideoBytes {
		left.Parameters.MaxVideoBytes = right.Parameters.MaxVideoBytes
	}
	if right.Parameters.MaxAudioBytes < left.Parameters.MaxAudioBytes {
		left.Parameters.MaxAudioBytes = right.Parameters.MaxAudioBytes
	}
	if right.Parameters.MaxVideoEditBytes < left.Parameters.MaxVideoEditBytes {
		left.Parameters.MaxVideoEditBytes = right.Parameters.MaxVideoEditBytes
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
	visibleGroups := make([]string, 0, 1)
	if _, ok := usableGroups["视频生成"]; ok {
		visibleGroups = append(visibleGroups, "视频生成")
	}

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
