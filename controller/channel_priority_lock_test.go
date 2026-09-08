package controller

import (
	"context"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestPriorityLockFiltersPrimaryProbeCandidatesAndRanking(t *testing.T) {
	fixed := stabilityTestChannel(1, 7)
	fixed.PriorityLocked = true
	normal := stabilityTestChannel(2, 1)
	channels := []*model.Channel{fixed, normal}
	eligible, skipped := channelGroupStabilityCandidates(channels)
	require.Equal(t, []*model.Channel{normal}, eligible)
	require.Equal(t, 1, skipped)
	primary, tied := selectChannelGroupStabilityPrimary(channels)
	require.False(t, tied)
	require.Equal(t, normal, primary)
	results := map[int]channelStabilityProbeResult{1: stabilityTestResult(fixed, true, 1), 2: stabilityTestResult(normal, true, 200)}
	ordered, patches := buildChannelGroupStabilityPriorities(channels, results, 1)
	require.Len(t, ordered, 1)
	require.Equal(t, normal, ordered[0].channel)
	for _, patch := range patches {
		require.NotEqual(t, fixed.Id, patch.ChannelId)
	}
}
func TestPriorityLockAllFixedSkipsProbesAndClearsOldPrimary(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.ChannelGroupRouting{}, &model.ChannelGroupStabilityPolicy{}))
	priority := int64(7)
	a := &model.Channel{Id: 8201, Type: 1, Key: "fixture", Name: "fixture", Status: common.ChannelStatusEnabled, Models: "fixture-model", Group: "fixed-only", Priority: &priority}
	require.NoError(t, a.Insert())
	locked := true
	_, err := model.UpdateChannelGroupRoutings("fixed-only", []model.ChannelGroupRoutingPatch{{ChannelId: a.Id, PriorityLocked: &locked}})
	require.NoError(t, err)
	policy, err := model.GetOrCreateChannelGroupStabilityPolicy("fixed-only")
	require.NoError(t, err)
	policy.Enabled = true
	policy.LastPrimaryChannelId = a.Id
	policy.LastPrimaryLatencyMs = 100
	require.NoError(t, db.Save(policy).Error)
	// No test user or upstream is configured: exclusion must run before either is accessed.
	for _, automatic := range []bool{true, false} {
		outcome := executeChannelGroupStability(context.Background(), *policy, automatic, !automatic)
		require.Equal(t, model.ChannelGroupStabilityResultNoChannels, outcome.result)
		require.True(t, outcome.clearPrimary)
		require.Contains(t, outcome.message, "跳过固定渠道 1 个")
		persistChannelGroupStabilityOutcome(*policy, automatic, outcome)
		current, err := model.GetOrCreateChannelGroupStabilityPolicy(policy.Group)
		require.NoError(t, err)
		require.Zero(t, current.LastPrimaryChannelId)
		require.Zero(t, current.LastPrimaryLatencyMs)
		require.Greater(t, current.NextCheckAt, time.Now().UnixMilli())
	}
}
func TestPriorityLockAPIExplicitFalseAndRerankMode(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.ChannelGroupRouting{}, &model.ChannelGroupStabilityPolicy{}, &model.Log{}))
	priority := int64(7)
	a := &model.Channel{Id: 8202, Type: 1, Key: "fixture", Name: "fixture", Status: common.ChannelStatusEnabled, Models: "fixture-model", Group: "api-lock", Priority: &priority}
	require.NoError(t, a.Insert())
	route := gin.New()
	route.PUT("/group-routing", UpdateChannelGroupRouting)
	send := func(body string) map[string]interface{} {
		req := httptest.NewRequest("PUT", "/group-routing", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		route.ServeHTTP(w, req)
		var result map[string]interface{}
		require.NoError(t, common.Unmarshal(w.Body.Bytes(), &result))
		return result
	}
	r := send(`{"group":"api-lock","updates":[{"channel_id":8202,"priority_locked":true}]}`)
	require.Equal(t, true, r["success"])
	r = send(`{"group":"api-lock","mode":"rerank","updates":[{"channel_id":8202,"priority":0}]}`)
	require.Equal(t, true, r["success"])
	require.Equal(t, float64(1), r["data"].(map[string]interface{})["skipped_locked"])
	r = send(`{"group":"api-lock","updates":[{"channel_id":8202,"priority":9}]}`)
	require.Equal(t, true, r["success"])
	require.NoError(t, model.PopulateEffectiveChannelRoutings([]*model.Channel{a}, "api-lock"))
	require.True(t, a.PriorityLocked)
	require.Equal(t, int64(9), *a.EffectivePriority)
	r = send(`{"group":"api-lock","updates":[{"channel_id":8202,"priority_locked":false}]}`)
	require.Equal(t, true, r["success"])
	require.NoError(t, model.PopulateEffectiveChannelRoutings([]*model.Channel{a}, "api-lock"))
	require.False(t, a.PriorityLocked)
	require.Equal(t, int64(9), *a.EffectivePriority)
}
func TestPriorityLockCancelsBothAutomaticAndManualRuns(t *testing.T) {
	for _, automatic := range []bool{true, false} {
		registry := channelGroupStabilityRunRegistry{runs: make(map[string]channelGroupStabilityRun)}
		var cancelled atomic.Bool
		registry.runs["group"] = channelGroupStabilityRun{automatic: automatic, cancel: func() { cancelled.Store(true) }}
		registry.cancel("group")
		require.True(t, cancelled.Load())
	}
}
