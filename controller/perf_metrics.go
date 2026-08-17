package controller

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

func GetPerfMetricsSummary(c *gin.Context) {
	setPerfMetricsPrivacyHeaders(c)

	hours := 24
	if rawHours := c.Query("hours"); rawHours != "" {
		if parsed, err := strconv.Atoi(rawHours); err == nil {
			hours = parsed
		}
	}

	visibleGroups, _, err := resolveVisiblePerfGroups(c)
	if err != nil {
		writePerfMetricsAccessError(c, err)
		return
	}

	result, err := perfmetrics.QuerySummaryAll(hours, visibleGroups)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

func GetPerfMetrics(c *gin.Context) {
	setPerfMetricsPrivacyHeaders(c)

	modelName := c.Query("model")
	if modelName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "model is required",
		})
		return
	}

	hours := 24
	if rawHours := c.Query("hours"); rawHours != "" {
		if parsed, err := strconv.Atoi(rawHours); err == nil {
			hours = parsed
		}
	}

	_, visibleGroupSet, err := resolveVisiblePerfGroups(c)
	if err != nil {
		writePerfMetricsAccessError(c, err)
		return
	}

	result, err := perfmetrics.Query(perfmetrics.QueryParams{
		Model: modelName,
		Group: c.Query("group"),
		Hours: hours,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	result.Groups = filterVisibleGroups(result.Groups, visibleGroupSet)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

func setPerfMetricsPrivacyHeaders(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	c.Header("Vary", "Cookie, Authorization, New-Api-User")
}

func resolveVisiblePerfGroups(c *gin.Context) ([]string, map[string]struct{}, error) {
	activeRatios := ratio_setting.GetGroupRatioCopy()
	visible := make(map[string]struct{}, len(activeRatios)+1)

	addUsableGroups := func(usable map[string]string) {
		for group := range activeRatios {
			if _, ok := usable[group]; ok {
				visible[group] = struct{}{}
			}
		}
		if _, ok := usable["auto"]; ok {
			visible["auto"] = struct{}{}
		}
	}

	rawUserID, authenticated := c.Get("id")
	if !authenticated {
		addUsableGroups(service.GetUserUsableGroups(""))
	} else {
		userID, ok := rawUserID.(int)
		if !ok || userID <= 0 {
			return nil, nil, fmt.Errorf("invalid authenticated user id")
		}

		user, err := model.GetUserCache(userID)
		if err != nil {
			return nil, nil, fmt.Errorf("load authenticated user %d: %w", userID, err)
		}

		if model.IsAdmin(userID) {
			for group := range activeRatios {
				visible[group] = struct{}{}
			}
			visible["auto"] = struct{}{}
		} else {
			addUsableGroups(service.GetUserUsableGroups(user.Group))
		}
	}

	groups := make([]string, 0, len(visible))
	for group := range visible {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	return groups, visible, nil
}

func filterVisibleGroups(groups []perfmetrics.GroupResult, visible map[string]struct{}) []perfmetrics.GroupResult {
	filtered := make([]perfmetrics.GroupResult, 0, len(groups))
	for _, group := range groups {
		if _, ok := visible[group.Group]; ok {
			filtered = append(filtered, group)
		}
	}
	return filtered
}

func writePerfMetricsAccessError(c *gin.Context, err error) {
	common.SysError("failed to resolve performance metric group access: " + err.Error())
	c.JSON(http.StatusInternalServerError, gin.H{
		"success": false,
		"message": "failed to resolve performance metric group access",
	})
}
