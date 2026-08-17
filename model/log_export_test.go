package model

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupLogExportTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	originalDB := DB
	originalLogDB := LOG_DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalCommonGroupCol := commonGroupCol
	originalLogGroupCol := logGroupCol

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.MemoryCacheEnabled = false
	commonGroupCol = "`group`"
	logGroupCol = "`group`"

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	LOG_DB = db
	require.NoError(t, db.AutoMigrate(&Log{}))
	require.NoError(t, db.Exec("CREATE TABLE channels (id INTEGER PRIMARY KEY, name TEXT)").Error)

	t.Cleanup(func() {
		DB = originalDB
		LOG_DB = originalLogDB
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		commonGroupCol = originalCommonGroupCol
		logGroupCol = originalLogGroupCol
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func TestExportAllLogsAppliesFiltersAndStableOrder(t *testing.T) {
	db := setupLogExportTestDB(t)
	require.NoError(t, db.Exec("INSERT INTO channels (id, name) VALUES (?, ?)", 7, "Primary").Error)

	logs := []*Log{
		{UserId: 1, CreatedAt: 200, Type: LogTypeConsume, Content: "older-id", Username: "alice", TokenName: "main", ModelName: "model-a", ChannelId: 7, Group: "paid", RequestId: "req", UpstreamRequestId: "up"},
		{UserId: 1, CreatedAt: 200, Type: LogTypeConsume, Content: "newer-id", Username: "alice", TokenName: "main", ModelName: "model-b", ChannelId: 7, Group: "paid", RequestId: "req", UpstreamRequestId: "up"},
		{UserId: 2, CreatedAt: 300, Type: LogTypeConsume, Content: "wrong-user", Username: "bob", TokenName: "main", ModelName: "model-c", ChannelId: 7, Group: "paid", RequestId: "req", UpstreamRequestId: "up"},
		{UserId: 1, CreatedAt: 150, Type: LogTypeError, Content: "wrong-type", Username: "alice", TokenName: "main", ModelName: "model-d", ChannelId: 7, Group: "paid", RequestId: "req", UpstreamRequestId: "up"},
	}
	require.NoError(t, db.Create(&logs).Error)

	exported, err := ExportAllLogs(LogQueryParams{
		LogType:           LogTypeConsume,
		StartTimestamp:    100,
		EndTimestamp:      250,
		ModelName:         "model-%",
		Username:          "alice",
		TokenName:         "main",
		Channel:           7,
		Group:             "paid",
		RequestId:         "req",
		UpstreamRequestId: "up",
	})
	require.NoError(t, err)
	require.Len(t, exported, 2)
	require.Equal(t, "newer-id", exported[0].Content)
	require.Equal(t, "older-id", exported[1].Content)
	require.Equal(t, "Primary", exported[0].ChannelName)
}

func TestExportUserLogsScopesAndSanitizes(t *testing.T) {
	db := setupLogExportTestDB(t)
	rawContent := "status_code=503, No available channel for model claude-fable-5 under group F4-AWS-Kiro-2 (distributor) (request id: upstream-secret)"
	other := common.MapToJsonStr(map[string]interface{}{
		"admin_info":    map[string]interface{}{"secret": true},
		"audit_info":    map[string]interface{}{"route": "/admin"},
		"stream_status": map[string]interface{}{"status": "error"},
		"channel_id":    8,
		"channel_name":  "upstream-secret-channel",
		"channel_type":  1,
		"error_type":    "upstream_error",
		"error_code":    "server_error",
		"status_code":   503,
		"base_url":      "https://upstream.example.com",
		"op":            map[string]interface{}{"action": "login"},
	})
	require.NoError(t, db.Create(&[]*Log{
		{UserId: 11, CreatedAt: 100, Type: LogTypeError, Content: rawContent, Username: "owner", ChannelId: 8, ChannelName: "hidden", TokenId: 42, Group: "default", RequestId: "local-request", UpstreamRequestId: "upstream-secret", Other: other},
		{UserId: 12, CreatedAt: 200, Type: LogTypeConsume, Content: "theirs", Username: "other", ChannelId: 9, Group: "default", Other: other},
	}).Error)

	exported, err := ExportUserLogs(11, LogQueryParams{
		Username: "other",
		Channel:  9,
		Group:    "default",
	})
	require.NoError(t, err)
	require.Len(t, exported, 1)
	require.Equal(t, "status_code=503, "+common.PublicPoolChannelUnavailableMessage, exported[0].Content)
	require.Zero(t, exported[0].ChannelId)
	require.Empty(t, exported[0].ChannelName)
	require.Empty(t, exported[0].UpstreamRequestId)
	require.Equal(t, "local-request", exported[0].RequestId)
	require.Equal(t, "default", exported[0].Group)

	cleaned, err := common.StrToMap(exported[0].Other)
	require.NoError(t, err)
	require.NotContains(t, cleaned, "admin_info")
	require.NotContains(t, cleaned, "audit_info")
	require.NotContains(t, cleaned, "stream_status")
	require.NotContains(t, cleaned, "channel_id")
	require.NotContains(t, cleaned, "channel_name")
	require.NotContains(t, cleaned, "channel_type")
	require.NotContains(t, cleaned, "base_url")
	require.Equal(t, "new_api_error", cleaned["error_type"])
	require.Equal(t, string(types.ErrorCodePoolChannelUnavailable), cleaned["error_code"])
	require.EqualValues(t, 503, cleaned["status_code"])
	require.Contains(t, cleaned, "op")

	byToken, err := GetLogByTokenId(42)
	require.NoError(t, err)
	require.Len(t, byToken, 1)
	require.Equal(t, "status_code=503, "+common.PublicPoolChannelUnavailableMessage, byToken[0].Content)
	require.Zero(t, byToken[0].ChannelId)
	require.Empty(t, byToken[0].UpstreamRequestId)

	adminLogs, err := ExportAllLogs(LogQueryParams{LogType: LogTypeError})
	require.NoError(t, err)
	require.Len(t, adminLogs, 1)
	require.Equal(t, rawContent, adminLogs[0].Content)
	require.Equal(t, 8, adminLogs[0].ChannelId)
	require.Equal(t, "upstream-secret", adminLogs[0].UpstreamRequestId)
}

func TestUserLogsAndExportShareStableOrder(t *testing.T) {
	db := setupLogExportTestDB(t)
	logs := []*Log{
		{UserId: 31, CreatedAt: 300, Type: LogTypeConsume, Content: "newest-time"},
		{UserId: 31, CreatedAt: 200, Type: LogTypeConsume, Content: "same-time-older-id"},
		{UserId: 31, CreatedAt: 200, Type: LogTypeConsume, Content: "same-time-newer-id"},
	}
	require.NoError(t, db.Create(&logs).Error)

	listed, total, err := GetUserLogs(31, LogQueryParams{}, 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 3, total)

	exported, err := ExportUserLogs(31, LogQueryParams{})
	require.NoError(t, err)

	listedContents := make([]string, len(listed))
	for i, log := range listed {
		listedContents[i] = log.Content
	}
	exportedContents := make([]string, len(exported))
	for i, log := range exported {
		exportedContents[i] = log.Content
	}
	expected := []string{"newest-time", "same-time-newer-id", "same-time-older-id"}
	require.Equal(t, expected, listedContents)
	require.Equal(t, expected, exportedContents)
}

func TestExportUserLogsEnforcesLimit(t *testing.T) {
	db := setupLogExportTestDB(t)
	logs := make([]Log, LogExportLimit)
	for i := range logs {
		logs[i] = Log{UserId: 21, CreatedAt: int64(i + 1), Type: LogTypeConsume}
	}
	require.NoError(t, db.CreateInBatches(&logs, 250).Error)

	exported, err := ExportUserLogs(21, LogQueryParams{})
	require.NoError(t, err)
	require.Len(t, exported, LogExportLimit)

	require.NoError(t, db.Create(&Log{UserId: 21, CreatedAt: int64(LogExportLimit + 1), Type: LogTypeConsume}).Error)
	_, err = ExportUserLogs(21, LogQueryParams{})
	require.ErrorIs(t, err, ErrLogExportLimitExceeded)
	require.True(t, errors.Is(err, ErrLogExportLimitExceeded))
}
