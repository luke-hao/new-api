package service

import (
	"fmt"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"strings"
)

// IsTextAutoModel is shared by candidate discovery, model listing and routing.
func IsTextAutoModel(name string) bool {
	value := strings.ToLower(name)
	if common.IsImageGenerationModel(name) || model_setting.IsGeminiModelSupportImagine(name) {
		return false
	}
	if _, video := common.GetVideoModelContract(name); video {
		return false
	}
	for _, marker := range []string{"image", "imagen", "seedream", "nano-banana", "embedding", "embed", "rerank", "tts", "whisper", "audio", "video", "sora"} {
		if strings.Contains(value, marker) {
			return false
		}
	}
	for _, endpoint := range model.GetModelSupportEndpointTypes(name) {
		switch endpoint {
		case constant.EndpointTypeOpenAI, constant.EndpointTypeOpenAIResponse, constant.EndpointTypeOpenAIResponseCompact, constant.EndpointTypeAnthropic, constant.EndpointTypeGemini:
			return true
		}
	}
	return false
}

func GetUserTextAutoGroups(userGroup string) (map[string]bool, error) {
	model.GetPricing()
	groups := GetUserUsableGroups(userGroup)
	names := make([]string, 0, len(groups))
	for name := range groups {
		if name != "auto" && ratio_setting.ContainsGroupRatio(name) {
			names = append(names, name)
		}
	}
	rows, err := model.GetGroupsEnabledModels(names)
	if err != nil {
		return nil, err
	}
	result := make(map[string]bool)
	for _, row := range rows {
		if IsTextAutoModel(row.Model) {
			result[row.Group] = true
		}
	}
	return result, nil
}

func ValidateTokenAutoGroups(userGroup string, configured model.TokenAutoGroups) error {
	if configured == "" {
		return nil
	}
	groups := configured.Groups()
	if len(groups) == 0 {
		return fmt.Errorf("select at least one auto group")
	}
	allowed, err := GetUserTextAutoGroups(userGroup)
	if err != nil {
		return err
	}
	seen := make(map[string]bool)
	for _, name := range groups {
		if name == "" || seen[name] {
			return fmt.Errorf("auto_groups contains an empty or duplicate group: %s", name)
		}
		if !allowed[name] {
			return fmt.Errorf("auto group is unavailable or has no text models: %s", name)
		}
		seen[name] = true
	}
	return nil
}

func HasCustomAutoGroups(c *gin.Context) bool {
	return len(c.GetStringSlice(string(constant.ContextKeyTokenAutoGroups))) > 0
}

func GetTokenAutoGroups(c *gin.Context, userGroup string) []string {
	configured := c.GetStringSlice(string(constant.ContextKeyTokenAutoGroups))
	if len(configured) == 0 {
		return GetUserAutoGroup(userGroup)
	}
	allowed := GetUserUsableGroups(userGroup)
	result := make([]string, 0, len(configured))
	for _, name := range configured {
		if _, ok := allowed[name]; ok && name != "auto" && ratio_setting.ContainsGroupRatio(name) {
			result = append(result, name)
		}
	}
	return result
}

func TokenMayUseAuto(userGroup string) bool {
	if GroupInUserUsableGroups(userGroup, "auto") {
		return true
	}
	groups, err := GetUserTextAutoGroups(userGroup)
	return err == nil && len(groups) > 0
}
