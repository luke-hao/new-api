package controller

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func Playground(c *gin.Context) {
	playgroundRelay(c, types.RelayFormatOpenAI)
}

func PlaygroundImage(c *gin.Context) {
	playgroundRelay(c, types.RelayFormatOpenAIImage)
}

func PlaygroundVideo(c *gin.Context) {
	if newAPIError := preparePlaygroundContext(c, requestedPlaygroundGroup(c)); newAPIError != nil {
		c.JSON(newAPIError.StatusCode, gin.H{"error": newAPIError.ToOpenAIError()})
		return
	}
	c.Set("relay_mode", relayconstant.RelayModeVideoSubmit)
	RelayTask(c)
}

func requestedPlaygroundGroup(c *gin.Context) string {
	// Distribute parses the requested /pg group into TokenGroup before the
	// controller runs. Prefer it to the user's default routing group.
	group := common.GetContextKeyString(c, constant.ContextKeyTokenGroup)
	if group != "" {
		return group
	}
	return common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
}

func PlaygroundVideoTask(c *gin.Context) {
	c.Set("relay_mode", relayconstant.RelayModeVideoFetchByID)
	RelayTaskFetch(c)
}

func preparePlaygroundContext(c *gin.Context, group string) *types.NewAPIError {
	if c.GetBool("use_access_token") {
		return types.NewError(errors.New("鏆備笉鏀寔浣跨敤 access token"), types.ErrorCodeAccessDenied, types.ErrOptionWithSkipRetry())
	}

	userID := c.GetInt("id")
	userCache, err := model.GetUserCache(userID)
	if err != nil {
		return types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
	}
	userCache.WriteContext(c)

	tempToken := &model.Token{
		UserId: userID,
		Name:   fmt.Sprintf("playground-%s", group),
		Group:  group,
	}
	if err := middleware.SetupContextForToken(c, tempToken); err != nil {
		return types.NewError(err, types.ErrorCodeAccessDenied, types.ErrOptionWithSkipRetry())
	}
	return nil
}

func playgroundRelay(c *gin.Context, relayFormat types.RelayFormat) {
	var newAPIError *types.NewAPIError

	defer func() {
		if newAPIError != nil {
			c.JSON(newAPIError.StatusCode, gin.H{
				"error": newAPIError.ToOpenAIError(),
			})
		}
	}()

	relayInfo, err := relaycommon.GenRelayInfo(c, relayFormat, nil, nil)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
		return
	}

	if newAPIError = preparePlaygroundContext(c, relayInfo.UsingGroup); newAPIError != nil {
		return
	}

	Relay(c, relayFormat)
}
