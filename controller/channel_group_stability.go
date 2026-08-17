package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
)

const (
	channelGroupStabilitySchedulerTick = 15 * time.Second
	channelGroupStabilityConcurrency   = 4
	channelGroupStabilityGracePeriod   = 10 * time.Second
	channelGroupStabilityMinInterval   = 1
	channelGroupStabilityMaxInterval   = 1440
	channelGroupStabilityMinHealthy    = 1
	channelGroupStabilityMaxHealthy    = 300
	channelGroupStabilityMinTimeout    = 2
	channelGroupStabilityMaxTimeout    = 600
)

type channelGroupStabilityRun struct {
	cancel    context.CancelFunc
	automatic bool
	startedAt int64
}

type channelGroupStabilityRunRegistry struct {
	mu   sync.Mutex
	runs map[string]channelGroupStabilityRun
}

var (
	channelGroupStabilityTaskOnce sync.Once
	channelGroupStabilityWake     = make(chan struct{}, 1)
	channelGroupStabilityRuns     = channelGroupStabilityRunRegistry{runs: make(map[string]channelGroupStabilityRun)}
)

type channelGroupStabilityConfigRequest struct {
	Group                   string `json:"group"`
	Enabled                 bool   `json:"enabled"`
	IntervalMinutes         int    `json:"interval_minutes"`
	HealthyThresholdSeconds int    `json:"healthy_threshold_seconds"`
	ProbeTimeoutSeconds     int    `json:"probe_timeout_seconds"`
}

type channelGroupStabilityRunRequest struct {
	Group string `json:"group"`
	Mode  string `json:"mode"`
}

type channelGroupStabilityStatusResponse struct {
	Group                   string `json:"group"`
	Enabled                 bool   `json:"enabled"`
	IntervalMinutes         int    `json:"interval_minutes"`
	HealthyThresholdSeconds int    `json:"healthy_threshold_seconds"`
	ProbeTimeoutSeconds     int    `json:"probe_timeout_seconds"`
	Running                 bool   `json:"running"`
	RunningSince            int64  `json:"running_since"`
	LastCheckAt             int64  `json:"last_check_at"`
	NextCheckAt             int64  `json:"next_check_at"`
	LastResult              string `json:"last_result"`
	LastMessage             string `json:"last_message"`
	LastPrimaryChannelId    int    `json:"last_primary_channel_id"`
	LastPrimaryLatencyMs    int64  `json:"last_primary_latency_ms"`
	LastReorderedAt         int64  `json:"last_reordered_at"`
}

type channelStabilityProbeResult struct {
	channel   *model.Channel
	success   bool
	latencyMs int64
	timedOut  bool
	err       error
}

type channelGroupStabilityOutcome struct {
	result           string
	message          string
	primaryChannelId int
	primaryLatencyMs int64
	reorderedAt      int64
}

func (registry *channelGroupStabilityRunRegistry) tryStart(group string, automatic bool, runner func(context.Context)) bool {
	registry.mu.Lock()
	if _, exists := registry.runs[group]; exists {
		registry.mu.Unlock()
		return false
	}
	ctx, cancel := context.WithCancel(context.Background())
	registry.runs[group] = channelGroupStabilityRun{cancel: cancel, automatic: automatic, startedAt: time.Now().UnixMilli()}
	registry.mu.Unlock()

	gopool.Go(func() {
		defer func() {
			cancel()
			registry.mu.Lock()
			delete(registry.runs, group)
			registry.mu.Unlock()
		}()
		runner(ctx)
	})
	return true
}

func (registry *channelGroupStabilityRunRegistry) cancelAutomatic(group string) {
	registry.mu.Lock()
	run, exists := registry.runs[group]
	registry.mu.Unlock()
	if exists && run.automatic {
		run.cancel()
	}
}

func (registry *channelGroupStabilityRunRegistry) status(group string) (bool, int64) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	run, exists := registry.runs[group]
	if !exists {
		return false, 0
	}
	return true, run.startedAt
}

func signalChannelGroupStabilityScheduler() {
	select {
	case channelGroupStabilityWake <- struct{}{}:
	default:
	}
}

func validateChannelGroupStabilityConfig(request channelGroupStabilityConfigRequest) error {
	if strings.TrimSpace(request.Group) == "" {
		return errors.New("group cannot be empty")
	}
	if request.IntervalMinutes < channelGroupStabilityMinInterval || request.IntervalMinutes > channelGroupStabilityMaxInterval {
		return fmt.Errorf("interval_minutes must be between %d and %d", channelGroupStabilityMinInterval, channelGroupStabilityMaxInterval)
	}
	if request.HealthyThresholdSeconds < channelGroupStabilityMinHealthy || request.HealthyThresholdSeconds > channelGroupStabilityMaxHealthy {
		return fmt.Errorf("healthy_threshold_seconds must be between %d and %d", channelGroupStabilityMinHealthy, channelGroupStabilityMaxHealthy)
	}
	if request.ProbeTimeoutSeconds < channelGroupStabilityMinTimeout || request.ProbeTimeoutSeconds > channelGroupStabilityMaxTimeout {
		return fmt.Errorf("probe_timeout_seconds must be between %d and %d", channelGroupStabilityMinTimeout, channelGroupStabilityMaxTimeout)
	}
	if request.ProbeTimeoutSeconds <= request.HealthyThresholdSeconds {
		return errors.New("probe_timeout_seconds must be greater than healthy_threshold_seconds")
	}
	return nil
}

func channelGroupStabilityResponse(policy *model.ChannelGroupStabilityPolicy) channelGroupStabilityStatusResponse {
	running, runningSince := channelGroupStabilityRuns.status(policy.Group)
	return channelGroupStabilityStatusResponse{
		Group: policy.Group, Enabled: policy.Enabled, IntervalMinutes: policy.IntervalMinutes,
		HealthyThresholdSeconds: policy.HealthyThresholdSeconds, ProbeTimeoutSeconds: policy.ProbeTimeoutSeconds,
		Running: running, RunningSince: runningSince, LastCheckAt: policy.LastCheckAt, NextCheckAt: policy.NextCheckAt,
		LastResult: policy.LastResult, LastMessage: policy.LastMessage, LastPrimaryChannelId: policy.LastPrimaryChannelId,
		LastPrimaryLatencyMs: policy.LastPrimaryLatencyMs, LastReorderedAt: policy.LastReorderedAt,
	}
}

func GetChannelGroupStability(c *gin.Context) {
	policy, err := model.GetOrCreateChannelGroupStabilityPolicy(strings.TrimSpace(c.Query("group")))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": channelGroupStabilityResponse(policy)})
}

func UpdateChannelGroupStability(c *gin.Context) {
	var request channelGroupStabilityConfigRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	request.Group = strings.TrimSpace(request.Group)
	if err := validateChannelGroupStabilityConfig(request); err != nil {
		common.ApiError(c, err)
		return
	}
	channelGroupStabilityRuns.cancelAutomatic(request.Group)
	policy, err := model.SaveChannelGroupStabilityConfig(model.ChannelGroupStabilityConfig{
		Group: request.Group, Enabled: request.Enabled, IntervalMinutes: request.IntervalMinutes,
		HealthyThresholdSeconds: request.HealthyThresholdSeconds, ProbeTimeoutSeconds: request.ProbeTimeoutSeconds,
	}, time.Now().UnixMilli())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "channel.group_stability.update", map[string]interface{}{
		"group": request.Group, "enabled": request.Enabled, "interval_minutes": request.IntervalMinutes,
		"healthy_threshold_seconds": request.HealthyThresholdSeconds, "probe_timeout_seconds": request.ProbeTimeoutSeconds,
	})
	if policy.Enabled {
		signalChannelGroupStabilityScheduler()
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": channelGroupStabilityResponse(policy)})
}

func RunChannelGroupStability(c *gin.Context) {
	var request channelGroupStabilityRunRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	request.Group = strings.TrimSpace(request.Group)
	if request.Group == "" || request.Mode != "full" {
		common.ApiErrorMsg(c, "invalid group stability run request")
		return
	}
	policy, err := model.GetOrCreateChannelGroupStabilityPolicy(request.Group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !startChannelGroupStabilityRun(*policy, false, true) {
		common.ApiErrorMsg(c, "该分组的稳定通道检测已在运行中")
		return
	}
	recordManageAudit(c, "channel.group_stability.run", map[string]interface{}{"group": request.Group, "mode": request.Mode})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"accepted": true}})
}

func StartChannelGroupStabilityTask() {
	channelGroupStabilityTaskOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			common.SysLog("channel group stability task started")
			ticker := time.NewTicker(channelGroupStabilitySchedulerTick)
			defer ticker.Stop()
			for {
				runDueChannelGroupStabilityPolicies()
				select {
				case <-ticker.C:
				case <-channelGroupStabilityWake:
				}
			}
		})
	})
}

func runDueChannelGroupStabilityPolicies() {
	policies, err := model.ListDueChannelGroupStabilityPolicies(time.Now().UnixMilli())
	if err != nil {
		common.SysError("failed to list due channel group stability policies: " + err.Error())
		return
	}
	for _, policy := range policies {
		startChannelGroupStabilityRun(policy, true, false)
	}
}

func startChannelGroupStabilityRun(policy model.ChannelGroupStabilityPolicy, automatic bool, forceFull bool) bool {
	return channelGroupStabilityRuns.tryStart(policy.Group, automatic, func(ctx context.Context) {
		runChannelGroupStabilitySafely(ctx, policy, automatic, forceFull)
	})
}

func runChannelGroupStabilitySafely(ctx context.Context, policy model.ChannelGroupStabilityPolicy, automatic bool, forceFull bool) {
	outcome := channelGroupStabilityOutcome{result: model.ChannelGroupStabilityResultError}
	defer func() {
		if recovered := recover(); recovered != nil {
			outcome = channelGroupStabilityOutcome{result: model.ChannelGroupStabilityResultError, message: fmt.Sprintf("stability task panic: %v", recovered)}
		}
		persistChannelGroupStabilityOutcome(policy, automatic, outcome)
	}()
	outcome = executeChannelGroupStability(ctx, policy, automatic, forceFull)
}

func executeChannelGroupStability(parent context.Context, policy model.ChannelGroupStabilityPolicy, automatic bool, forceFull bool) channelGroupStabilityOutcome {
	channels, err := model.GetEnabledChannelsForGroupStability(policy.Group)
	if err != nil {
		return channelGroupStabilityOutcome{result: model.ChannelGroupStabilityResultError, message: err.Error()}
	}
	if len(channels) == 0 {
		return channelGroupStabilityOutcome{result: model.ChannelGroupStabilityResultNoChannels, message: "当前分组没有启用渠道"}
	}
	phaseCount := 1 + (len(channels)+channelGroupStabilityConcurrency-1)/channelGroupStabilityConcurrency
	runTimeout := time.Duration(phaseCount*policy.ProbeTimeoutSeconds)*time.Second + channelGroupStabilityGracePeriod
	ctx, cancel := context.WithTimeout(parent, runTimeout)
	defer cancel()
	testUserID, err := resolveChannelTestUserID(nil)
	if err != nil {
		return channelGroupStabilityOutcome{result: model.ChannelGroupStabilityResultError, message: err.Error()}
	}
	currentPrimary, tied := selectChannelGroupStabilityPrimary(channels)
	results := make(map[int]channelStabilityProbeResult, len(channels))
	if !forceFull && !tied && currentPrimary != nil {
		primaryResult := probeChannelForStability(ctx, currentPrimary, testUserID, policy.ProbeTimeoutSeconds)
		if ctx.Err() != nil {
			return channelGroupStabilityContextOutcome(ctx)
		}
		results[currentPrimary.Id] = primaryResult
		if primaryResult.success && primaryResult.latencyMs <= int64(policy.HealthyThresholdSeconds)*1000 {
			return channelGroupStabilityOutcome{
				result:           model.ChannelGroupStabilityResultHealthy,
				message:          fmt.Sprintf("主通道 #%d 响应 %dms，保持现有优先级", currentPrimary.Id, primaryResult.latencyMs),
				primaryChannelId: currentPrimary.Id, primaryLatencyMs: primaryResult.latencyMs, reorderedAt: policy.LastReorderedAt,
			}
		}
	}
	remaining := make([]*model.Channel, 0, len(channels))
	for _, channel := range channels {
		if _, exists := results[channel.Id]; !exists {
			remaining = append(remaining, channel)
		}
	}
	for channelID, result := range probeChannelsForStability(ctx, remaining, testUserID, policy.ProbeTimeoutSeconds) {
		results[channelID] = result
	}
	if ctx.Err() != nil {
		return channelGroupStabilityContextOutcome(ctx)
	}
	currentPrimaryID := 0
	if !tied && currentPrimary != nil {
		currentPrimaryID = currentPrimary.Id
	}
	ordered, patches := buildChannelGroupStabilityPriorities(channels, results, currentPrimaryID)
	if len(ordered) == 0 {
		return channelGroupStabilityOutcome{result: model.ChannelGroupStabilityResultAllFailed, message: fmt.Sprintf("%s 分组的 %d 个启用渠道全部检测失败，保留原优先级", policy.Group, len(channels))}
	}
	updated := 0
	if len(patches) > 0 {
		updated, err = model.ApplyChannelGroupStabilityPriorities(policy, automatic, patches)
		if err != nil {
			if errors.Is(err, model.ErrChannelGroupStabilityPolicyStale) {
				return channelGroupStabilityOutcome{result: model.ChannelGroupStabilityResultCancelled, message: err.Error()}
			}
			return channelGroupStabilityOutcome{result: model.ChannelGroupStabilityResultError, message: err.Error()}
		}
		model.InitChannelCache()
	}
	resultName := model.ChannelGroupStabilityResultUnchanged
	reorderedAt := policy.LastReorderedAt
	if updated > 0 {
		resultName = model.ChannelGroupStabilityResultReranked
		reorderedAt = time.Now().UnixMilli()
	}
	primary := ordered[0]
	common.SysLog(fmt.Sprintf("channel group stability done: group=%s channels=%d success=%d updated=%d primary=%d latency_ms=%d", policy.Group, len(channels), len(ordered), updated, primary.channel.Id, primary.latencyMs))
	return channelGroupStabilityOutcome{
		result: resultName, message: fmt.Sprintf("检测 %d 个渠道，成功 %d 个，更新 %d 个优先级", len(channels), len(ordered), updated),
		primaryChannelId: primary.channel.Id, primaryLatencyMs: primary.latencyMs, reorderedAt: reorderedAt,
	}
}

func persistChannelGroupStabilityOutcome(policy model.ChannelGroupStabilityPolicy, automatic bool, outcome channelGroupStabilityOutcome) {
	if outcome.primaryChannelId == 0 {
		outcome.primaryChannelId = policy.LastPrimaryChannelId
		outcome.primaryLatencyMs = policy.LastPrimaryLatencyMs
	}
	if outcome.reorderedAt == 0 {
		outcome.reorderedAt = policy.LastReorderedAt
	}
	completedAt := time.Now().UnixMilli()
	nextCheckAt := int64(0)
	if policy.Enabled {
		nextCheckAt = model.ChannelGroupStabilityNextCheckAt(policy, completedAt)
	}
	persisted, err := model.UpdateChannelGroupStabilityRun(policy, automatic, model.ChannelGroupStabilityRunUpdate{
		LastCheckAt: completedAt, NextCheckAt: nextCheckAt, LastResult: outcome.result, LastMessage: outcome.message,
		LastPrimaryChannelId: outcome.primaryChannelId, LastPrimaryLatencyMs: outcome.primaryLatencyMs, LastReorderedAt: outcome.reorderedAt,
	})
	if err != nil {
		common.SysError("failed to persist channel group stability result: " + err.Error())
		return
	}
	if !persisted {
		return
	}
	previousAnomaly := policy.LastResult == model.ChannelGroupStabilityResultAllFailed || policy.LastResult == model.ChannelGroupStabilityResultError
	currentAnomaly := outcome.result == model.ChannelGroupStabilityResultAllFailed || outcome.result == model.ChannelGroupStabilityResultError
	if currentAnomaly && !previousAnomaly {
		gopool.Go(func() {
			service.NotifyRootUser(dto.NotifyTypeChannelTest, fmt.Sprintf("稳定通道检测异常：%s", policy.Group), outcome.message)
		})
	}
}

func channelGroupStabilityContextOutcome(ctx context.Context) channelGroupStabilityOutcome {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return channelGroupStabilityOutcome{result: model.ChannelGroupStabilityResultError, message: "稳定通道整轮检测超时"}
	}
	return channelGroupStabilityOutcome{result: model.ChannelGroupStabilityResultCancelled, message: ctx.Err().Error()}
}

func selectChannelGroupStabilityPrimary(channels []*model.Channel) (*model.Channel, bool) {
	if len(channels) == 0 {
		return nil, false
	}
	maxPriority := effectiveChannelPriority(channels[0])
	top := make([]*model.Channel, 0, 1)
	for _, channel := range channels {
		priority := effectiveChannelPriority(channel)
		if priority > maxPriority {
			maxPriority = priority
			top = top[:0]
		}
		if priority == maxPriority {
			top = append(top, channel)
		}
	}
	if len(top) != 1 {
		return nil, true
	}
	return top[0], false
}

func effectiveChannelPriority(channel *model.Channel) int64 {
	if channel.EffectivePriority != nil {
		return *channel.EffectivePriority
	}
	return channel.GetPriority()
}

func probeChannelsForStability(ctx context.Context, channels []*model.Channel, testUserID int, timeoutSeconds int) map[int]channelStabilityProbeResult {
	results := make(map[int]channelStabilityProbeResult, len(channels))
	if len(channels) == 0 {
		return results
	}
	jobs := make(chan *model.Channel)
	resultCh := make(chan channelStabilityProbeResult, len(channels))
	workerCount := channelGroupStabilityConcurrency
	if len(channels) < workerCount {
		workerCount = len(channels)
	}
	var workers sync.WaitGroup
	workers.Add(workerCount)
	for i := 0; i < workerCount; i++ {
		go func() {
			defer workers.Done()
			for channel := range jobs {
				if ctx.Err() != nil {
					return
				}
				resultCh <- probeChannelForStability(ctx, channel, testUserID, timeoutSeconds)
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, channel := range channels {
			select {
			case jobs <- channel:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		workers.Wait()
		close(resultCh)
	}()
	for result := range resultCh {
		results[result.channel.Id] = result
	}
	return results
}

func probeChannelForStability(parent context.Context, channel *model.Channel, testUserID int, timeoutSeconds int) channelStabilityProbeResult {
	ctx, cancel := context.WithTimeout(parent, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()
	startedAt := time.Now()
	testResultCh := make(chan testResult, 1)
	panicCh := make(chan interface{}, 1)
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				panicCh <- recovered
			}
		}()
		testResultCh <- testChannelWithContext(ctx, channel, testUserID, "", "", true)
	}()
	result := channelStabilityProbeResult{channel: channel}
	select {
	case testResult := <-testResultCh:
		result.latencyMs = time.Since(startedAt).Milliseconds()
		if testResult.localErr == nil && testResult.newAPIError == nil {
			result.success = true
			channel.UpdateResponseTime(result.latencyMs)
			return result
		}
		result.err = testResult.localErr
		if result.err == nil && testResult.newAPIError != nil {
			result.err = testResult.newAPIError
		}
		channel.UpdateTestFailure(channelTestErrorCode(testResult), channelTestErrorMessage(testResult))
		return result
	case recovered := <-panicCh:
		result.latencyMs = time.Since(startedAt).Milliseconds()
		result.err = fmt.Errorf("channel test panic: %v", recovered)
		channel.UpdateTestFailure("channel_test_panic", result.err.Error())
		return result
	case <-ctx.Done():
		result.latencyMs = time.Since(startedAt).Milliseconds()
		result.err = ctx.Err()
		result.timedOut = errors.Is(ctx.Err(), context.DeadlineExceeded) && parent.Err() == nil
		if result.timedOut {
			channel.UpdateTestFailure("channel_test_timeout", fmt.Sprintf("稳定通道检测超过 %d 秒", timeoutSeconds))
		}
		return result
	}
}

func buildChannelGroupStabilityPriorities(channels []*model.Channel, results map[int]channelStabilityProbeResult, currentPrimaryID int) ([]channelStabilityProbeResult, []model.ChannelGroupRoutingPatch) {
	successful := make([]channelStabilityProbeResult, 0, len(channels))
	for _, channel := range channels {
		if result, exists := results[channel.Id]; exists && result.success {
			successful = append(successful, result)
		}
	}
	if len(successful) == 0 {
		return nil, nil
	}
	sort.Slice(successful, func(i, j int) bool {
		if successful[i].latencyMs == successful[j].latencyMs {
			return successful[i].channel.Id < successful[j].channel.Id
		}
		return successful[i].latencyMs < successful[j].latencyMs
	})
	if currentPrimaryID > 0 && successful[0].channel.Id != currentPrimaryID {
		currentIndex := -1
		for i := range successful {
			if successful[i].channel.Id == currentPrimaryID {
				currentIndex = i
				break
			}
		}
		if currentIndex >= 0 && successful[0].latencyMs*100 > successful[currentIndex].latencyMs*85 {
			current := successful[currentIndex]
			copy(successful[1:currentIndex+1], successful[0:currentIndex])
			successful[0] = current
		}
	}
	priorities := make(map[int]int64, len(channels))
	for _, channel := range channels {
		priorities[channel.Id] = 0
	}
	for index, result := range successful {
		priorities[result.channel.Id] = int64(len(successful) - index)
	}
	patches := make([]model.ChannelGroupRoutingPatch, 0, len(channels))
	for _, channel := range channels {
		priority := priorities[channel.Id]
		if effectiveChannelPriority(channel) == priority {
			continue
		}
		priorityCopy := priority
		patches = append(patches, model.ChannelGroupRoutingPatch{ChannelId: channel.Id, Priority: &priorityCopy})
	}
	return successful, patches
}
