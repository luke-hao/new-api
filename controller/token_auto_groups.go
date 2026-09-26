package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func validateTokenAutoGroups(c *gin.Context, token *model.Token, previous *model.Token) error {
	if token.AutoGroups == "" || (token.Group != "auto" && len(token.AutoGroups.Groups()) == 0) {
		return nil
	}
	if previous != nil && token.Group != "auto" && string(token.AutoGroups) == string(previous.AutoGroups) {
		return nil
	}
	userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
	if userGroup == "" {
		var err error
		userGroup, err = model.GetUserGroup(c.GetInt("id"), false)
		if err != nil {
			return err
		}
	}
	return service.ValidateTokenAutoGroups(userGroup, token.AutoGroups)
}
