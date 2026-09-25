package controller

import (
	"fmt"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"strconv"
	"strings"
)

func parseTokenListOptions(c *gin.Context) (model.TokenListOptions, error) {
	o := model.TokenListOptions{Sort: c.Query("sort"), Desc: c.DefaultQuery("order", "desc") == "desc", ContainsName: c.Query("name_match") == "contains"}
	if order := c.Query("order"); order != "" && order != "asc" && order != "desc" {
		return o, fmt.Errorf("invalid sort direction")
	}
	if raw := c.Query("status"); raw != "" {
		for _, s := range strings.Split(raw, ",") {
			n, e := strconv.Atoi(s)
			if e != nil || n < 1 || n > 4 {
				return o, fmt.Errorf("invalid token status")
			}
			o.Status = append(o.Status, n)
		}
	}
	if g, ok := c.GetQuery("group"); ok {
		o.Group = &g
	}
	return o, nil
}

type tokenStatusResult struct {
	ID      int    `json:"id"`
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

func UpdateTokenStatusBatch(c *gin.Context) {
	var req struct {
		IDs    []int `json:"ids"`
		Status int   `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "invalid request")
		return
	}
	ids, ok := normalizeTokenBatchIDs(req.IDs)
	if !ok || (req.Status != 1 && req.Status != 2) {
		common.ApiErrorMsg(c, "invalid ids or status")
		return
	}
	results := make([]tokenStatusResult, 0, len(ids))
	for _, id := range ids {
		_, err := model.UpdateTokenStatusOnly(id, c.GetInt("id"), req.Status)
		item := tokenStatusResult{ID: id, Success: err == nil}
		if err != nil {
			item.Message = "Token unavailable, expired, or quota exhausted"
		}
		results = append(results, item)
	}
	common.ApiSuccess(c, results)
}
