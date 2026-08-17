package controller

import (
	"errors"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func ProbeChannelUpstreamBilling(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil || channelID <= 0 {
		common.ApiErrorMsg(c, "invalid channel id")
		return
	}

	channel, err := model.GetChannelById(channelID, true)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiErrorMsg(c, "channel not found")
			return
		}
		common.ApiError(c, err)
		return
	}

	previous := service.DecodeChannelUpstreamBillingProbeSnapshot(channel.GetOtherInfo())
	snapshot, err := service.ProbeChannelUpstreamBilling(c.Request.Context(), channel, previous)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	_, err = model.MergeChannelOtherInfoAndName(channelID, func(currentChannel *model.Channel, otherInfo map[string]interface{}) error {
		current := service.DecodeChannelUpstreamBillingProbeSnapshot(otherInfo)
		if current != nil && current.AttemptedAt > snapshot.AttemptedAt {
			snapshot = current
			return nil
		}
		snapshot.NameUpdated = false
		snapshot.ChannelName = currentChannel.Name
		if multiplier, ok := service.ChannelUpstreamBillingNameMultiplier(snapshot); ok {
			if updatedName, changed := service.FormatChannelNameWithBillingMultiplier(currentChannel.Name, multiplier); changed {
				currentChannel.Name = updatedName
				snapshot.NameUpdated = true
				snapshot.ChannelName = updatedName
			}
		}
		otherInfo[service.ChannelUpstreamBillingProbeOtherInfoKey] = snapshot
		return nil
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, snapshot)
}
