package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"strconv"
	"strings"
	"time"
)

func GetTokenMetrics(c *gin.Context) {
	raw := strings.Split(c.Query("ids"), ",")
	ids := make([]int, 0, len(raw))
	for _, v := range raw {
		id, e := strconv.Atoi(v)
		if e != nil {
			common.ApiErrorMsg(c, "invalid ids")
			return
		}
		ids = append(ids, id)
	}
	ids, ok := normalizeTokenBatchIDs(ids)
	if !ok {
		common.ApiErrorMsg(c, "invalid ids")
		return
	}
	userID := c.GetInt("id")
	var count int64
	if err := model.DB.Model(&model.Token{}).Where("user_id = ? AND id IN ?", userID, ids).Count(&count).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if int(count) != len(ids) {
		c.JSON(403, gin.H{"success": false, "message": "Token unavailable"})
		return
	}
	state := "available"
	updated := int64(0)
	usage := map[int]model.TokenDailyUsage{}
	if !common.LogConsumeEnabled {
		state = "disabled"
	} else {
		rows, at, err := service.CachedTokenDailyUsage(c.Request.Context(), userID, ids)
		if err != nil {
			state = "unavailable"
			common.SysError("token daily metrics query failed: " + err.Error())
		} else {
			updated = at
			for _, r := range rows {
				usage[r.TokenID] = r
			}
		}
	}
	activity := service.LiveTokenActivity.Snapshot(ids)
	items := make([]gin.H, 0, len(ids))
	for _, id := range ids {
		item := gin.H{"id": id, "active": activity[id].Active, "rpm": activity[id].RPM, "today_quota": nil, "today_tokens": nil}
		if state == "available" {
			item["today_quota"] = usage[id].Quota
			item["today_tokens"] = usage[id].Tokens
		}
		items = append(items, item)
	}
	c.Header("Cache-Control", "no-store")
	common.ApiSuccess(c, gin.H{"items": items, "consumption_status": state, "consumption_updated_at": updated, "as_of": time.Now().Unix(), "timezone": "Asia/Shanghai", "activity_scope": "instance"})
}
