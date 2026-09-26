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

// Image output is different from text models that merely accept image input.
func IsImageAutoModel(name string) bool {
	if _, video := common.GetVideoModelContract(name); video {
		return false
	}
	if common.IsImageGenerationModel(name) || model_setting.IsGeminiModelSupportImagine(name) ||
		strings.Contains(strings.ToLower(name), "seedream") {
		return true
	}
	for _, endpoint := range model.GetModelSupportEndpointTypes(name) {
		if endpoint == constant.EndpointTypeImageGeneration {
			return true
		}
	}
	return false
}

func IsPersonalAutoModel(name string) bool {
	return IsTextAutoModel(name) || IsImageAutoModel(name)
}

type AutoGroupCapabilities struct {
	Text  bool
	Image bool
}

func GetUserAutoGroupCapabilities(userGroup string) (map[string]AutoGroupCapabilities, error) {
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
	result := make(map[string]AutoGroupCapabilities)
	for _, row := range rows {
		capability := result[row.Group]
		capability.Text = capability.Text || IsTextAutoModel(row.Model)
		capability.Image = capability.Image || IsImageAutoModel(row.Model)
		if capability.Text || capability.Image {
			result[row.Group] = capability
		}
	}
	return result, nil
}

func GetUserTextAutoGroups(userGroup string) (map[string]bool, error) {
	capabilities, err := GetUserAutoGroupCapabilities(userGroup)
	if err != nil {
		return nil, err
	}
	result := make(map[string]bool)
	for name, capability := range capabilities {
		if capability.Text {
			result[name] = true
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
	allowed, err := GetUserAutoGroupCapabilities(userGroup)
	if err != nil {
		return err
	}
	seen := make(map[string]bool)
	for _, name := range groups {
		if name == "" || seen[name] {
			return fmt.Errorf("auto_groups contains an empty or duplicate group: %s", name)
		}
		if !allowed[name].Text && !allowed[name].Image {
			return fmt.Errorf("auto group is unavailable or has no text or image models: %s", name)
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
	groups, err := GetUserAutoGroupCapabilities(userGroup)
	if err != nil {
		return false
	}
	for _, capability := range groups {
		if capability.Text || capability.Image {
			return true
		}
	}
	return false
}
