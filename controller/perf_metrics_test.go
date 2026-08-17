package controller

import (
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const (
	perfRegularUserID = 2401
	perfAdminUserID   = 2402
)

func setupPerfMetricsControllerTest(t *testing.T) {
	t.Helper()

	savedUsableGroups := setting.UserUsableGroups2JSONString()
	savedGroupRatios := ratio_setting.GroupRatio2JSONString()
	savedSpecialGroups := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.ReadAll()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(savedUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(savedGroupRatios))
		ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.Clear()
		ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.AddAll(savedSpecialGroups)
	})

	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{
		"public":"Public",
		"auto":"Automatic"
	}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{
		"public":1,
		"assigned":1,
		"special":1,
		"hidden":1
	}`))
	ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.Clear()
	ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.AddAll(map[string]map[string]string{
		"assigned": {
			"+:special": "Special",
			"-:public":  "",
		},
	})

	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.PerfMetric{}))
	require.NoError(t, db.Create([]model.User{
		{
			Id:       perfRegularUserID,
			Username: "perf-regular-user",
			Password: "password",
			Role:     common.RoleCommonUser,
			Status:   common.UserStatusEnabled,
			Group:    "assigned",
			AffCode:  "perf-regular-aff",
		},
		{
			Id:       perfAdminUserID,
			Username: "perf-admin-user",
			Password: "password",
			Role:     common.RoleAdminUser,
			Status:   common.UserStatusEnabled,
			Group:    "assigned",
			AffCode:  "perf-admin-aff",
		},
	}).Error)

	bucket := time.Now().Add(-30 * time.Minute).Unix()
	require.NoError(t, db.Create([]model.PerfMetric{
		perfMetricFixture("shared-model", "public", bucket, 1, 100),
		perfMetricFixture("shared-model", "assigned", bucket, 2, 400),
		perfMetricFixture("shared-model", "special", bucket, 1, 300),
		perfMetricFixture("shared-model", "hidden", bucket, 1, 1000),
		perfMetricFixture("shared-model", "auto", bucket, 1, 700),
		perfMetricFixture("public-only-model", "public", bucket, 1, 150),
		perfMetricFixture("hidden-only-model", "hidden", bucket, 1, 900),
	}).Error)
}

func perfMetricFixture(modelName, group string, bucket int64, requests, totalLatency int64) model.PerfMetric {
	return model.PerfMetric{
		ModelName:      modelName,
		Group:          group,
		BucketTs:       bucket,
		RequestCount:   requests,
		SuccessCount:   requests,
		TotalLatencyMs: totalLatency,
		TtftSumMs:      requests * 20,
		TtftCount:      requests,
		OutputTokens:   requests * 100,
		GenerationMs:   requests * 1000,
	}
}

type perfMetricsControllerResponse struct {
	Success bool                    `json:"success"`
	Message string                  `json:"message"`
	Data    perfmetrics.QueryResult `json:"data"`
}

type perfMetricsSummaryControllerResponse struct {
	Success bool                         `json:"success"`
	Message string                       `json:"message"`
	Data    perfmetrics.SummaryAllResult `json:"data"`
}

func performPerfMetricsRequest(t *testing.T, target string, userID int) (*httptest.ResponseRecorder, perfMetricsControllerResponse) {
	t.Helper()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, target, nil)
	if userID > 0 {
		ctx.Set("id", userID)
	}

	GetPerfMetrics(ctx)
	var response perfMetricsControllerResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return recorder, response
}

func performPerfMetricsSummaryRequest(t *testing.T, userID int) (*httptest.ResponseRecorder, perfMetricsSummaryControllerResponse) {
	t.Helper()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/perf-metrics/summary?hours=24", nil)
	if userID > 0 {
		ctx.Set("id", userID)
	}

	GetPerfMetricsSummary(ctx)
	var response perfMetricsSummaryControllerResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return recorder, response
}

func groupNames(groups []perfmetrics.GroupResult) []string {
	names := make([]string, 0, len(groups))
	for _, group := range groups {
		names = append(names, group.Group)
	}
	sort.Strings(names)
	return names
}

func summaryByModel(models []perfmetrics.ModelSummary) map[string]perfmetrics.ModelSummary {
	result := make(map[string]perfmetrics.ModelSummary, len(models))
	for _, item := range models {
		result[item.ModelName] = item
	}
	return result
}

func TestGetPerfMetricsFiltersGroupsByViewerAccess(t *testing.T) {
	setupPerfMetricsControllerTest(t)

	tests := []struct {
		name   string
		userID int
		groups []string
	}{
		{name: "anonymous", groups: []string{"auto", "public"}},
		{name: "regular", userID: perfRegularUserID, groups: []string{"assigned", "auto", "special"}},
		{name: "admin", userID: perfAdminUserID, groups: []string{"assigned", "auto", "hidden", "public", "special"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder, response := performPerfMetricsRequest(t, "/api/perf-metrics?model=shared-model&hours=24", test.userID)
			require.Equal(t, http.StatusOK, recorder.Code)
			require.Equal(t, "private, no-store", recorder.Header().Get("Cache-Control"))
			require.Contains(t, recorder.Header().Get("Vary"), "Cookie")
			require.True(t, response.Success)
			require.NotEmpty(t, response.Data.SeriesSchema)
			require.Equal(t, test.groups, groupNames(response.Data.Groups))
		})
	}
}

func TestGetPerfMetricsHiddenGroupQueryReturnsEmpty(t *testing.T) {
	setupPerfMetricsControllerTest(t)

	_, response := performPerfMetricsRequest(t, "/api/perf-metrics?model=shared-model&group=hidden&hours=24", perfRegularUserID)
	require.True(t, response.Success)
	require.NotEmpty(t, response.Data.SeriesSchema)
	require.Empty(t, response.Data.Groups)
}

func TestGetPerfMetricsSummaryExcludesHiddenGroupData(t *testing.T) {
	setupPerfMetricsControllerTest(t)

	tests := []struct {
		name           string
		userID         int
		sharedLatency  int64
		wantPublicOnly bool
		wantHiddenOnly bool
	}{
		{name: "anonymous", sharedLatency: 400, wantPublicOnly: true},
		{name: "regular", userID: perfRegularUserID, sharedLatency: 350},
		{name: "admin", userID: perfAdminUserID, sharedLatency: 416, wantPublicOnly: true, wantHiddenOnly: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder, response := performPerfMetricsSummaryRequest(t, test.userID)
			require.Equal(t, http.StatusOK, recorder.Code)
			require.True(t, response.Success)
			models := summaryByModel(response.Data.Models)
			require.Equal(t, test.sharedLatency, models["shared-model"].AvgLatencyMs)
			_, hasPublicOnly := models["public-only-model"]
			_, hasHiddenOnly := models["hidden-only-model"]
			require.Equal(t, test.wantPublicOnly, hasPublicOnly)
			require.Equal(t, test.wantHiddenOnly, hasHiddenOnly)
		})
	}
}

func TestGetPerfMetricsFailsClosedWhenAuthenticatedUserIsMissing(t *testing.T) {
	setupPerfMetricsControllerTest(t)

	recorder, response := performPerfMetricsRequest(t, "/api/perf-metrics?model=shared-model&hours=24", 999999)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.False(t, response.Success)
	require.Equal(t, "failed to resolve performance metric group access", response.Message)
	require.Empty(t, response.Data.Groups)
}
