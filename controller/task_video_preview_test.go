package controller

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestTaskVideoPreviewAccessAndRanges(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Task{}))
	previousMemory := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	fetch := system_setting.GetFetchSetting()
	previousFetch := *fetch
	fetch.EnableSSRFProtection = false
	t.Cleanup(func() { common.MemoryCacheEnabled = previousMemory; *fetch = previousFetch })
	service.InitHttpClient()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/videos/upstream-task/content", r.URL.Path)
		require.Equal(t, "Bearer fixture-channel-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "video/mp4")
		http.ServeContent(w, r, "video.mp4", time.Time{}, strings.NewReader("0123456789"))
	}))
	defer upstream.Close()
	baseURL := upstream.URL
	channel := model.Channel{Id: 991, Type: constant.ChannelTypeOpenAI, Key: "fixture-channel-key", BaseURL: &baseURL}
	require.NoError(t, db.Create(&channel).Error)
	for _, user := range []model.User{
		{Id: 1, Username: "admin", Role: common.RoleAdminUser, Status: common.UserStatusEnabled},
		{Id: 2, Username: "owner", Role: common.RoleCommonUser, Status: common.UserStatusEnabled},
		{Id: 3, Username: "other", Role: common.RoleCommonUser, Status: common.UserStatusEnabled},
		{Id: 4, Username: "disabled-admin", Role: common.RoleAdminUser, Status: common.UserStatusDisabled},
		{Id: 5, Username: "demoted-admin", Role: common.RoleCommonUser, Status: common.UserStatusEnabled},
	} {
		user.AffCode = "preview-" + strconv.Itoa(user.Id)
		require.NoError(t, db.Create(&user).Error)
	}
	for _, task := range []model.Task{
		{TaskID: "task_success", UserId: 2, ChannelId: 991, Status: model.TaskStatusSuccess, PrivateData: model.TaskPrivateData{UpstreamTaskID: "upstream-task"}},
		{TaskID: "task_queued", UserId: 2, ChannelId: 991, Status: model.TaskStatusQueued},
		{TaskID: "task_failed", UserId: 2, ChannelId: 991, Status: model.TaskStatusFailure},
	} {
		require.NoError(t, db.Create(&task).Error)
	}

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("task-preview-test-secret"))))
	engine.GET("/test-login/:id", func(c *gin.Context) {
		id, _ := strconv.Atoi(c.Param("id"))
		session := sessions.Default(c)
		session.Set("id", id)
		session.Set("status", common.UserStatusEnabled)
		session.Set("role", common.RoleAdminUser) // Stale/elevated session role must not grant access.
		require.NoError(t, session.Save())
		c.Status(http.StatusOK)
	})
	engine.GET("/api/task/:task_id/content", middleware.AdminMediaAuth(), AdminVideoProxy)
	engine.GET("/v1/videos/:task_id/content", middleware.TokenOrUserAuth(), VideoProxy)

	for _, test := range []struct {
		name, path   string
		user, status int
		ranged       bool
	}{
		{"admin-other-user", "/api/task/task_success/content", 1, 200, false},
		{"admin-seek", "/api/task/task_success/content", 1, 206, true},
		{"owner", "/v1/videos/task_success/content", 2, 200, false},
		{"owner-seek", "/v1/videos/task_success/content", 2, 206, true},
		{"other-user", "/v1/videos/task_success/content", 3, 404, false},
		{"admin-api-still-owner-only", "/v1/videos/task_success/content", 1, 404, false},
		{"common-user-admin-route", "/api/task/task_success/content", 2, 403, false},
		{"disabled-admin", "/api/task/task_success/content", 4, 403, false},
		{"demoted-admin", "/api/task/task_success/content", 5, 403, false},
		{"deleted-admin", "/api/task/task_success/content", 6, 403, false},
		{"anonymous", "/api/task/task_success/content", 0, 401, false},
		{"queued", "/api/task/task_queued/content", 1, 400, false},
		{"failed", "/api/task/task_failed/content", 1, 400, false},
		{"missing", "/api/task/task_missing/content", 1, 404, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, test.path, nil)
			if test.user > 0 {
				login := httptest.NewRecorder()
				engine.ServeHTTP(login, httptest.NewRequest(http.MethodGet, "/test-login/"+strconv.Itoa(test.user), nil))
				for _, c := range login.Result().Cookies() {
					req.AddCookie(c)
				}
			} else {
				req.Header.Set("Authorization", "Bearer fixture-channel-key")
				req.Header.Set("New-Api-User", "1") // Headers alone never authenticate the admin media route.
			}
			if test.ranged {
				req.Header.Set("Range", "bytes=2-5")
			}
			result := httptest.NewRecorder()
			engine.ServeHTTP(result, req)
			require.Equal(t, test.status, result.Code, result.Body.String())
			if test.status == 200 || test.status == 206 {
				require.Equal(t, "private, no-store", result.Header().Get("Cache-Control"))
				require.Equal(t, "video/mp4", result.Header().Get("Content-Type"))
				require.NotContains(t, result.Body.String(), "fixture-channel-key")
				if test.ranged {
					require.Equal(t, "2345", result.Body.String())
					require.Equal(t, "bytes 2-5/10", result.Header().Get("Content-Range"))
				} else {
					require.Equal(t, "0123456789", result.Body.String())
				}
			}
		})
	}
}
