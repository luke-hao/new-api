package service

import (
	"fmt"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"strings"
)

const tokenAutoSelected = "token_auto_selected"
const tokenAutoCandidates = "token_auto_candidates"
const tokenAutoPreferred = "token_auto_preferred"

// PrepareTokenAutoRoute freezes the authorized order for one request.
func PrepareTokenAutoRoute(c *gin.Context, modelName string) error {
	if !HasCustomAutoGroups(c) {
		return nil
	}
	model.GetPricing()
	path := strings.TrimSuffix(c.Request.URL.Path, "/")
	textPath := strings.HasSuffix(path, "/chat/completions") || strings.HasSuffix(path, "/completions") ||
		strings.HasSuffix(path, "/responses") || strings.HasSuffix(path, "/responses/compact") ||
		strings.HasSuffix(path, "/messages") || strings.HasSuffix(path, "/messages/count_tokens") ||
		strings.HasSuffix(path, ":generateContent") || strings.HasSuffix(path, ":streamGenerateContent") || strings.HasSuffix(path, ":countTokens")
	if !textPath || !IsTextAutoModel(modelName) {
		return fmt.Errorf("custom auto groups support text models and text endpoints only")
	}
	groups := GetTokenAutoGroups(c, common.GetContextKeyString(c, constant.ContextKeyUserGroup))
	if len(groups) == 0 {
		return fmt.Errorf("no permitted auto groups remain; edit this API key")
	}
	c.Set(tokenAutoCandidates, groups)
	return nil
}

func SetTokenAutoPreferredChannel(c *gin.Context, id int) {
	c.Set(tokenAutoPreferred, id)
	c.Set(ginKeyChannelAffinitySkipRetry, false)
}

func selectTokenAutoChannel(param *RetryParam) (*model.Channel, string, error) {
	c := param.Ctx
	groups := c.GetStringSlice(tokenAutoCandidates)
	if groups == nil {
		if err := PrepareTokenAutoRoute(c, param.ModelName); err != nil {
			return nil, "auto", err
		}
		groups = c.GetStringSlice(tokenAutoCandidates)
	}
	selected := c.GetString(tokenAutoSelected)
	cross := common.GetContextKeyBool(c, constant.ContextKeyTokenCrossGroupRetry)
	start := c.GetInt(string(constant.ContextKeyAutoGroupIndex))
	for i := start; i < len(groups); i++ {
		group := groups[i]
		if selected != "" && !cross && group != selected {
			break
		}
		retry := param.GetRetry()
		if i > start {
			retry = 0
			param.SetRetry(0)
		}
		channel, err := selectResponsesFailoverChannel(param, group, retry)
		if err != nil {
			return nil, group, err
		}
		if channel == nil {
			if selected != "" && !cross {
				break
			}
			c.Set(string(constant.ContextKeyAutoGroupIndex), i+1)
			continue
		}
		// An affinity channel may only replace the chosen channel within this group.
		if selected == "" {
			id := c.GetInt(tokenAutoPreferred)
			if id > 0 && model.IsChannelEnabledForGroupModel(group, param.ModelName, id) && !IsResponsesChannelCoolingDown(c, group, param.ModelName, id) {
				if preferred, err := model.CacheGetChannel(id); err == nil && preferred != nil && preferred.Status == common.ChannelStatusEnabled {
					channel = preferred
					MarkChannelAffinityUsed(c, group, id)
				}
			}
		}
		common.SetContextKey(c, constant.ContextKeyAutoGroup, group)
		c.Set(string(constant.ContextKeyAutoGroupIndex), i)
		if param.Preselect {
			return channel, group, nil
		}
		c.Set(tokenAutoSelected, group)
		if cross && retry >= common.RetryTimes {
			c.Set(string(constant.ContextKeyAutoGroupIndex), i+1)
			param.SetRetry(0)
			param.ResetRetryNextTry()
		}
		return channel, group, nil
	}
	return nil, "auto", fmt.Errorf("no available channel in the selected auto groups for model %s", param.ModelName)
}

// HasNextTokenAutoGroup preserves the first attempt of each remaining personal group.
func HasNextTokenAutoGroup(c *gin.Context) bool {
	return common.GetContextKeyString(c, constant.ContextKeyTokenGroup) == "auto" && HasCustomAutoGroups(c) &&
		common.GetContextKeyBool(c, constant.ContextKeyTokenCrossGroupRetry) &&
		c.GetInt(string(constant.ContextKeyAutoGroupIndex)) < len(c.GetStringSlice(tokenAutoCandidates))
}
