package relay

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupPlaygroundVideoTaskDB(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.Task{}))
	t.Cleanup(func() {
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestPlaygroundVideoFetchRestrictsTaskOwnership(t *testing.T) {
	db := setupPlaygroundVideoTaskDB(t)
	taskData, err := json.Marshal(map[string]any{
		"id":         "upstream-task",
		"object":     "video",
		"model":      "sora-2",
		"status":     "queued",
		"progress":   0,
		"created_at": 1,
	})
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.Task{
		TaskID:   "task_owned",
		UserId:   41,
		Platform: constant.TaskPlatform("55"),
		Data:     taskData,
		Status:   model.TaskStatusQueued,
	}).Error)

	ownedRecorder := httptest.NewRecorder()
	ownedContext, _ := gin.CreateTestContext(ownedRecorder)
	ownedContext.Request = httptest.NewRequest(http.MethodGet, "/pg/videos/task_owned", nil)
	ownedContext.Params = gin.Params{{Key: "task_id", Value: "task_owned"}}
	ownedContext.Set("id", 41)
	require.Nil(t, RelayTaskFetch(ownedContext, relayconstant.RelayModeVideoFetchByID))
	require.Contains(t, ownedRecorder.Body.String(), `"id":"task_owned"`)

	otherRecorder := httptest.NewRecorder()
	otherContext, _ := gin.CreateTestContext(otherRecorder)
	otherContext.Request = httptest.NewRequest(http.MethodGet, "/pg/videos/task_owned", nil)
	otherContext.Params = gin.Params{{Key: "task_id", Value: "task_owned"}}
	otherContext.Set("id", 42)
	taskErr := RelayTaskFetch(otherContext, relayconstant.RelayModeVideoFetchByID)
	require.NotNil(t, taskErr)
	require.Equal(t, "task_not_exist", taskErr.Code)
}
