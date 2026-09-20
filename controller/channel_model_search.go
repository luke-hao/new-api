package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"strings"
)

// Existing fuzzy search and precise routing-model filtering are independent.
func searchModelChannels(c *gin.Context, name string) {
	group := model.NormalizeChannelGroupFilter(c.Query("group"))
	if group == "" {
		common.ApiErrorMsg(c, "请先选择单个分组")
		return
	}
	idSort, _ := strconv.ParseBool(c.Query("id_sort"))
	options := model.NewChannelSortOptions(c.Query("sort_by"), c.Query("sort_order"), idSort)
	channels, err := model.SearchChannels(c.Query("keyword"), group, c.Query("model"), idSort)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	ids := make([]int, 0, len(channels))
	for _, channel := range channels {
		ids = append(ids, channel.Id)
	}
	kind := -1
	if value := c.Query("type"); value != "" {
		if n, e := strconv.Atoi(value); e == nil {
			kind = n
		}
	}
	query := model.ApplyRoutingModelFilter(buildChannelListQuery(group, parseStatusFilter(c.Query("status")), kind), group, name).Where("channels.id IN ?", ids)
	channels = nil
	if err := options.ApplyForModel(query, group, name).Omit("key").Find(&channels).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	counts := map[int]int{}
	for _, channel := range channels {
		counts[channel.Type]++
	}
	page := common.GetPageQuery(c)
	total := len(channels)
	start := page.GetStartIdx()
	if start > total {
		start = total
	}
	end := start + page.GetPageSize()
	if end > total {
		end = total
	}
	channels = channels[start:end]
	if err := model.PopulateEffectiveModelRoutings(channels, group, strings.TrimSpace(name)); err != nil {
		common.ApiError(c, err)
		return
	}
	for _, channel := range channels {
		clearChannelInfo(channel)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"items": channels, "total": total, "type_counts": counts}})
}
