package controller

import (
	"context"
	"errors"
	"fmt"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
	"time"
)

func init() { model.CancelChannelModelStability = cancelChannelModelRuns }

type channelStabilityGroupContextKey struct{}

func modelStabilityKey(group, name string) string { return group + "\x00" + name }

func cancelChannelModelRuns(group, name string) {
	channelGroupStabilityRuns.mu.Lock()
	defer channelGroupStabilityRuns.mu.Unlock()
	for key, run := range channelGroupStabilityRuns.runs {
		g, m, _ := strings.Cut(key, "\x00")
		if g == group && (name == "" || m == name) {
			run.cancel()
		}
	}
}

func channelModelStatus(parent model.ChannelGroupStabilityPolicy, child model.ChannelModelStabilityPolicy) channelGroupStabilityStatusResponse {
	effective := model.ResolveChannelModelPolicy(parent, child)
	status := channelGroupStabilityResponse(&effective)
	status.Model = child.Model
	status.GroupEnabled = parent.Enabled
	status.Paused = child.Paused
	status.Initialized = child.Initialized
	status.IntervalOverride = child.IntervalMinutes
	status.HealthyOverride = child.HealthyThresholdSeconds
	status.TimeoutOverride = child.ProbeTimeoutSeconds
	status.Running, status.RunningSince = channelGroupStabilityRuns.status(modelStabilityKey(parent.Group, child.Model))
	return status
}

func getChannelModelStability(c *gin.Context) {
	group, name := strings.TrimSpace(c.Query("group")), strings.TrimSpace(c.Query("model"))
	parent, err := model.GetOrCreateChannelGroupStabilityPolicy(group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	names, err := model.ListChannelRoutingModels(group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	result := channelGroupStabilityResponse(parent)
	result.GroupEnabled = parent.Enabled
	result.LastPrimaryChannelId = 0
	result.LastPrimaryLatencyMs = 0
	result.LastResult = model.ChannelGroupStabilityResultNever
	result.LastMessage = ""
	result.LastCheckAt = 0
	result.NextCheckAt = 0
	result.Running = false
	found := name == ""
	for _, candidate := range names {
		child, err := model.GetChannelModelPolicy(group, candidate)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		status := channelModelStatus(*parent, *child)
		if name == candidate {
			result = status
			found = true
			break
		}
		if name == "" {
			result.Models = append(result.Models, status)
			result.Running = result.Running || status.Running
			if status.LastCheckAt > result.LastCheckAt {
				result.LastCheckAt = status.LastCheckAt
			}
			if status.NextCheckAt > 0 && (result.NextCheckAt == 0 || status.NextCheckAt < result.NextCheckAt) {
				result.NextCheckAt = status.NextCheckAt
			}
		}
	}
	if !found {
		common.ApiErrorMsg(c, "当前分组没有此模型")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
}

func updateChannelModelStability(c *gin.Context, r channelGroupStabilityConfigRequest) {
	p := model.ChannelModelStabilityPolicy{Group: r.Group, Model: r.Model, Paused: r.Paused, IntervalMinutes: r.IntervalOverride, HealthyThresholdSeconds: r.HealthyOverride, ProbeTimeoutSeconds: r.TimeoutOverride}
	if err := model.SaveChannelModelPolicy(p); err != nil {
		common.ApiError(c, err)
		return
	}
	parent, err := model.GetOrCreateChannelGroupStabilityPolicy(r.Group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	saved, err := model.GetChannelModelPolicy(r.Group, r.Model)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "channel.model_stability.update", map[string]interface{}{"group": r.Group, "model": r.Model, "paused": r.Paused})
	signalChannelGroupStabilityScheduler()
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": channelModelStatus(*parent, *saved)})
}

func startChannelModelRuns(parent model.ChannelGroupStabilityPolicy, name string, automatic, full bool) int {
	names, err := model.ListChannelRoutingModels(parent.Group)
	if err != nil {
		common.SysError(err.Error())
		return 0
	}
	started := 0
	for _, candidate := range names {
		if name != "" && name != candidate {
			continue
		}
		child, err := model.GetChannelModelPolicy(parent.Group, candidate)
		if err != nil {
			common.SysError(err.Error())
			continue
		}
		effective := model.ResolveChannelModelPolicy(parent, *child)
		if automatic && (!effective.Enabled || effective.NextCheckAt > time.Now().UnixMilli()) {
			continue
		}
		if channelGroupStabilityRuns.tryStart(modelStabilityKey(parent.Group, candidate), automatic, func(ctx context.Context) {
			runChannelModelStability(ctx, effective, automatic, full || !child.Initialized)
		}) {
			started++
		}
	}
	return started
}

func supportsModelStabilityProbe(c *model.Channel, name string) bool {
	if _, known := common.GetVideoModelContract(name); known {
		return false
	}
	switch c.Type {
	case constant.ChannelTypeMidjourney, constant.ChannelTypeMidjourneyPlus, constant.ChannelTypeSunoAPI, constant.ChannelTypeKling, constant.ChannelTypeJimeng, constant.ChannelTypeDoubaoVideo, constant.ChannelTypeVidu:
		return false
	}
	return true
}

func runChannelModelStability(ctx context.Context, policy model.ChannelGroupStabilityPolicy, automatic, full bool) {
	runChannelModelStabilityWithProbe(ctx, policy, automatic, full, probeChannelForStability, probeChannelsForStability)
}

func runChannelModelStabilityWithProbe(ctx context.Context, policy model.ChannelGroupStabilityPolicy, automatic, full bool,
	probe func(context.Context, *model.Channel, int, int, ...string) channelStabilityProbeResult,
	probeMany func(context.Context, []*model.Channel, int, int, ...string) map[int]channelStabilityProbeResult) {
	ctx = context.WithValue(ctx, channelStabilityGroupContextKey{}, policy.Group)
	outcome := channelGroupStabilityOutcome{result: model.ChannelGroupStabilityResultError}
	var observations []model.ChannelModelRouting
	var patches []model.ChannelGroupRoutingPatch
	snapshot := ""
	initialized := false
	defer func() {
		if recovered := recover(); recovered != nil {
			outcome = channelGroupStabilityOutcome{result: model.ChannelGroupStabilityResultError, message: fmt.Sprintf("probe panic: %v", recovered)}
			patches = nil
			initialized = false
		}
		if ctx.Err() != nil {
			return
		}
		primaryID, latency := outcome.primaryChannelId, outcome.primaryLatencyMs
		if primaryID == 0 && !outcome.clearPrimary {
			primaryID = policy.LastPrimaryChannelId
			latency = policy.LastPrimaryLatencyMs
		}
		reorderedAt := outcome.reorderedAt
		if reorderedAt == 0 {
			reorderedAt = policy.LastReorderedAt
		}
		completed := time.Now().UnixMilli()
		saved, err := model.CommitChannelModelStabilityRun(policy, automatic, snapshot, observations, patches, model.ChannelGroupStabilityRunUpdate{
			LastCheckAt: completed, NextCheckAt: model.ChannelGroupStabilityNextCheckAt(policy, completed), LastResult: outcome.result, LastMessage: outcome.message,
			LastPrimaryChannelId: primaryID, LastPrimaryLatencyMs: latency, LastReorderedAt: reorderedAt,
		}, initialized)
		if errors.Is(err, model.ErrChannelGroupStabilityPolicyStale) {
			return
		}
		if err != nil {
			common.SysError("model stability commit: " + err.Error())
			return
		}
		if saved && len(patches) > 0 {
			model.InitChannelCache()
		}
		if saved && (outcome.result == model.ChannelGroupStabilityResultAllFailed || outcome.result == model.ChannelGroupStabilityResultError) && outcome.result != policy.LastResult {
			service.NotifyRootUser(dto.NotifyTypeChannelTest, fmt.Sprintf("稳定通道检测异常：%s / %s", policy.Group, policy.Model), outcome.message)
		}
	}()
	channels, currentSnapshot, err := model.GetChannelModelStabilityCandidates(policy.Group, policy.Model)
	if err != nil {
		outcome.message = err.Error()
		return
	}
	snapshot = currentSnapshot
	channels, skipped := channelGroupStabilityCandidates(channels)
	if len(channels) == 0 {
		outcome = channelGroupStabilityOutcome{result: model.ChannelGroupStabilityResultNoChannels, message: fmt.Sprintf("参与渠道 0，固定渠道 %d", skipped), clearPrimary: true}
		return
	}
	supported := make([]*model.Channel, 0, len(channels))
	for _, channel := range channels {
		if supportsModelStabilityProbe(channel, policy.Model) {
			supported = append(supported, channel)
		} else {
			observations = append(observations, model.ChannelModelRouting{ChannelId: channel.Id, LastCheckAt: time.Now().UnixMilli(), Result: "unsupported", Message: "此模型暂不支持自动探测"})
		}
	}
	channels = supported
	if len(channels) == 0 {
		outcome = channelGroupStabilityOutcome{result: "unsupported", message: "此模型暂不支持自动探测，保留原排名", clearPrimary: true}
		return
	}
	userID, err := resolveChannelTestUserID(nil)
	if err != nil {
		outcome.message = err.Error()
		return
	}
	primary, tied := selectChannelGroupStabilityPrimary(channels)
	results := map[int]channelStabilityProbeResult{}
	record := func(r channelStabilityProbeResult) {
		state, message := "success", ""
		if !r.success {
			state = "failed"
			message = "模型检测失败"
			if r.timedOut {
				state = "timeout"
				message = "模型检测超时"
			}
		}
		observations = append(observations, model.ChannelModelRouting{ChannelId: r.channel.Id, LastCheckAt: time.Now().UnixMilli(), LatencyMs: r.latencyMs, Result: state, Message: message})
		results[r.channel.Id] = r
	}
	if !full && !tied && primary != nil {
		r := probe(ctx, primary, userID, policy.ProbeTimeoutSeconds, policy.Model)
		if ctx.Err() != nil {
			return
		}
		record(r)
		if r.success && r.latencyMs <= int64(policy.HealthyThresholdSeconds)*1000 {
			outcome = channelGroupStabilityOutcome{result: model.ChannelGroupStabilityResultHealthy, message: fmt.Sprintf("主通道 #%d 响应 %dms，保持现有排名", primary.Id, r.latencyMs), primaryChannelId: primary.Id, primaryLatencyMs: r.latencyMs}
			return
		}
	}
	remaining := make([]*model.Channel, 0, len(channels))
	for _, c := range channels {
		if _, ok := results[c.Id]; !ok {
			remaining = append(remaining, c)
		}
	}
	for _, r := range probeMany(ctx, remaining, userID, policy.ProbeTimeoutSeconds, policy.Model) {
		record(r)
	}
	if ctx.Err() != nil {
		return
	}
	currentID := 0
	if !tied && primary != nil {
		currentID = primary.Id
	}
	ordered, newPatches := buildChannelGroupStabilityPriorities(channels, results, currentID)
	if len(ordered) == 0 {
		outcome = channelGroupStabilityOutcome{result: model.ChannelGroupStabilityResultAllFailed, message: "全部参与渠道检测失败，保留原排名"}
		return
	}
	// Even unchanged numbers become explicit model values on the first full run.
	patches = newPatches
	if !policy.Initialized {
		priorities := map[int]int64{}
		for _, c := range channels {
			priorities[c.Id] = effectiveChannelPriority(c)
		}
		for _, p := range patches {
			priorities[p.ChannelId] = *p.Priority
		}
		patches = nil
		for _, c := range channels {
			priority := priorities[c.Id]
			patches = append(patches, model.ChannelGroupRoutingPatch{ChannelId: c.Id, Priority: &priority})
		}
	}
	initialized = true
	outcome = channelGroupStabilityOutcome{result: model.ChannelGroupStabilityResultUnchanged, message: fmt.Sprintf("检测 %d 个渠道，成功 %d 个，固定跳过 %d 个", len(channels), len(ordered), skipped), primaryChannelId: ordered[0].channel.Id, primaryLatencyMs: ordered[0].latencyMs}
	if len(patches) > 0 {
		outcome.result = model.ChannelGroupStabilityResultReranked
		outcome.reorderedAt = time.Now().UnixMilli()
	}
}
