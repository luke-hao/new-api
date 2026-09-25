package controller

import (
	"context"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestKeyConsoleFiltersAndStatusOnly(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	now := time.Now().Unix()
	tokens := []model.Token{
		{Id: 1, UserId: 1, Key: "fixture-a", Name: "alpha one", Status: 1, ExpiredTime: -1, RemainQuota: 100, Group: "fast"},
		{Id: 2, UserId: 1, Key: "fixture-b", Name: "alpha two", Status: 2, ExpiredTime: -1, RemainQuota: 200, Group: "fast"},
		{Id: 3, UserId: 1, Key: "fixture-c", Name: "alpha three", Status: 1, ExpiredTime: now - 10, RemainQuota: 300, Group: "slow"},
		{Id: 4, UserId: 2, Key: "fixture-d", Name: "alpha foreign", Status: 1, ExpiredTime: -1, RemainQuota: 999, Group: "fast"},
	}
	if err := db.Create(&tokens).Error; err != nil {
		t.Fatal(err)
	}
	rows, total, err := model.SearchUserTokens(1, "alpha", "", 0, 1, model.TokenListOptions{ContainsName: true, Sort: "remain_quota", Desc: true})
	if err != nil || total != 3 || len(rows) != 1 || rows[0].Id != 3 {
		t.Fatalf("%+v %d %v", rows, total, err)
	}
	rows, total, err = model.SearchUserTokens(1, "", "", 0, 20, model.TokenListOptions{Status: []int{1}})
	if err != nil || total != 1 || rows[0].Id != 1 {
		t.Fatalf("%+v %d %v", rows, total, err)
	}
	rows, total, err = model.SearchUserTokens(1, "", "", 0, 20, model.TokenListOptions{Status: []int{3}})
	if err != nil || total != 1 || rows[0].Id != 3 {
		t.Fatalf("%+v %d %v", rows, total, err)
	}
	if _, _, err = model.SearchUserTokens(1, "", "", 0, 20, model.TokenListOptions{Sort: "id; DROP TABLE tokens"}); err == nil {
		t.Fatal("accepted arbitrary sort")
	}
	db.Model(&model.Token{}).Where("id = ?", 1).Updates(map[string]interface{}{"remain_quota": 75, "used_quota": 25})
	updated, err := model.UpdateTokenStatusOnly(1, 1, 2)
	if err != nil || updated.RemainQuota != 75 || updated.UsedQuota != 25 || updated.Status != 2 {
		t.Fatalf("%+v %v", updated, err)
	}
	if _, err = model.UpdateTokenStatusOnly(4, 1, 2); err == nil {
		t.Fatal("cross user update")
	}
	if _, err = model.UpdateTokenStatusOnly(3, 1, 1); err == nil {
		t.Fatal("enabled expired key")
	}
	if _, err = model.UpdateTokenStatusOnly(1, 1, 99); err == nil {
		t.Fatal("accepted invalid status")
	}
}
func TestKeyConsoleDailyUsageIsolationAndRefund(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	if err := db.AutoMigrate(&model.Log{}); err != nil {
		t.Fatal(err)
	}
	start, end := service.TokenDayBounds(time.Now())
	logs := []model.Log{
		{UserId: 1, TokenId: 10, TokenName: "same", Type: model.LogTypeConsume, CreatedAt: start, Quota: 100, PromptTokens: 4, CompletionTokens: 6},
		{UserId: 1, TokenId: 10, TokenName: "same", Type: model.LogTypeRefund, CreatedAt: start + 1, Quota: 30},
		{UserId: 1, TokenId: 11, TokenName: "same", Type: model.LogTypeConsume, CreatedAt: start + 1, Quota: 999},
		{UserId: 2, TokenId: 10, Type: model.LogTypeConsume, CreatedAt: start + 1, Quota: 999},
		{UserId: 1, TokenId: 10, Type: model.LogTypeConsume, CreatedAt: start - 1, Quota: 999},
		{UserId: 1, TokenId: 10, Type: model.LogTypeConsume, CreatedAt: end, Quota: 999},
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatal(err)
	}
	rows, err := model.GetTokenDailyUsage(context.Background(), 1, []int{10}, start, end)
	if err != nil || len(rows) != 1 || rows[0].Quota != 70 || rows[0].Tokens != 10 {
		t.Fatalf("%+v %v", rows, err)
	}
}
func TestKeyConsoleMetricsOwnershipAndBatch(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	db.Create(&model.Token{Id: 1, UserId: 1, Key: "test-one", Status: 1, ExpiredTime: -1, UnlimitedQuota: true})
	db.Create(&model.Token{Id: 2, UserId: 2, Key: "test-two", Status: 1, ExpiredTime: -1, UnlimitedQuota: true})
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("id", 1) })
	r.GET("/metrics", GetTokenMetrics)
	r.PUT("/batch", UpdateTokenStatusBatch)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics?ids=2", nil))
	if w.Code != 403 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/batch", strings.NewReader("{\"ids\":[1,2],\"status\":2}"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if !strings.Contains(w.Body.String(), "\"success\":false") || !strings.Contains(w.Body.String(), "\"success\":true") {
		t.Fatal(w.Body.String())
	}
	one, _ := model.GetTokenByIds(1, 1)
	two, _ := model.GetTokenByIds(2, 2)
	if one.Status != 2 || two.Status != 1 {
		t.Fatalf("%+v %+v", one, two)
	}
}

func TestKeyConsoleActivityOnlySkipsConsumptionQuery(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	if err := db.Create(&model.Token{Id: 1, UserId: 1, Key: "activity-only", Status: 1, ExpiredTime: -1}).Error; err != nil {
		t.Fatal(err)
	}
	previous := common.LogConsumeEnabled
	common.LogConsumeEnabled = true
	t.Cleanup(func() { common.LogConsumeEnabled = previous })
	// No logs table exists in this fixture, so a consumption query would fail.
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("id", 1) })
	r.GET("/metrics", GetTokenMetrics)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics?ids=1&activity_only=1", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"consumption_status":"not_requested"`) || !strings.Contains(w.Body.String(), `"today_quota":null`) || !strings.Contains(w.Body.String(), `"active":0`) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}
