package middleware

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/common/limiter"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type test40TrafficClass string

const (
	test40Username                 = "test40"
	test40UpstreamChannelID        = 111
	test40GlobalConcurrency        = 16
	test40QueueCapacity            = 50
	test40QueueMaxWait             = 30 * time.Second
	test40QueuePollInterval        = 250 * time.Millisecond
	test40TrafficClassStrict       = test40TrafficClass("strict")
	test40TrafficClassLoose        = test40TrafficClass("loose")
	test40TrafficBusyErrorCode     = types.ErrorCode("traffic_shaping_busy")
	test40RequestCanceledErrorCode = types.ErrorCode("request_canceled")
)

// test40TrafficConfig keeps high-cost traffic available while smoothing bursts
// before requests reach the selected upstream channel.
type test40TrafficConfig struct {
	ratePerSecond int
	burst         int
	requestCost   int
	maxConcurrent int
	retryAfter    int
}

type test40ModelTrafficConfig struct {
	strict test40TrafficConfig
	loose  test40TrafficConfig
}

var (
	test40Sonnet5Config = test40ModelTrafficConfig{
		strict: test40TrafficConfig{ratePerSecond: 2, burst: 8, requestCost: 4, maxConcurrent: 8, retryAfter: 2},
		loose:  test40TrafficConfig{ratePerSecond: 1, burst: 12, requestCost: 1, maxConcurrent: 12, retryAfter: 1},
	}
	test40Opus5Config = test40ModelTrafficConfig{
		strict: test40TrafficConfig{ratePerSecond: 1, burst: 5, requestCost: 5, maxConcurrent: 4, retryAfter: 5},
		loose:  test40TrafficConfig{ratePerSecond: 1, burst: 6, requestCost: 3, maxConcurrent: 6, retryAfter: 3},
	}
	test40OpusConfig = test40ModelTrafficConfig{
		strict: test40TrafficConfig{ratePerSecond: 1, burst: 4, requestCost: 4, maxConcurrent: 4, retryAfter: 4},
		loose:  test40TrafficConfig{ratePerSecond: 1, burst: 6, requestCost: 2, maxConcurrent: 6, retryAfter: 2},
	}
	test40SonnetConfig = test40ModelTrafficConfig{
		strict: test40TrafficConfig{ratePerSecond: 1, burst: 6, requestCost: 2, maxConcurrent: 8, retryAfter: 2},
		loose:  test40TrafficConfig{ratePerSecond: 1, burst: 12, requestCost: 1, maxConcurrent: 12, retryAfter: 1},
	}
	test40FableConfig = test40ModelTrafficConfig{
		strict: test40TrafficConfig{ratePerSecond: 1, burst: 6, requestCost: 3, maxConcurrent: 6, retryAfter: 3},
		loose:  test40TrafficConfig{ratePerSecond: 2, burst: 12, requestCost: 3, maxConcurrent: 8, retryAfter: 2},
	}
	test40HaikuConfig = test40ModelTrafficConfig{
		strict: test40TrafficConfig{ratePerSecond: 2, burst: 10, requestCost: 2, maxConcurrent: 10, retryAfter: 1},
		loose:  test40TrafficConfig{ratePerSecond: 2, burst: 12, requestCost: 1, maxConcurrent: 12, retryAfter: 1},
	}
	test40ClaudeDefault = test40ModelTrafficConfig{
		strict: test40TrafficConfig{ratePerSecond: 1, burst: 6, requestCost: 3, maxConcurrent: 6, retryAfter: 3},
		loose:  test40TrafficConfig{ratePerSecond: 2, burst: 12, requestCost: 3, maxConcurrent: 8, retryAfter: 2},
	}
)

type test40Semaphore struct {
	slots chan struct{}
}

func (s *test40Semaphore) tryAcquire() bool {
	select {
	case s.slots <- struct{}{}:
		return true
	default:
		return false
	}
}

func (s *test40Semaphore) release() {
	select {
	case <-s.slots:
	default:
	}
}

type test40WaitQueue struct {
	slots chan struct{}
}

func (q *test40WaitQueue) tryJoin() bool {
	select {
	case q.slots <- struct{}{}:
		return true
	default:
		return false
	}
}

func (q *test40WaitQueue) leave() {
	select {
	case <-q.slots:
	default:
	}
}

type test40MemoryBucket struct {
	mu         sync.Mutex
	tokens     float64
	lastRefill time.Time
}

type test40TrafficShaper struct {
	semaphores        sync.Map
	waitQueues        sync.Map
	memoryBuckets     sync.Map
	queueCapacity     int
	maxWait           time.Duration
	pollInterval      time.Duration
	globalConcurrent  int
	allowRateOverride func(context.Context, string, test40TrafficConfig) bool
}

func newTest40TrafficShaper(queueCapacity, globalConcurrent int, maxWait, pollInterval time.Duration) *test40TrafficShaper {
	return &test40TrafficShaper{
		queueCapacity:    queueCapacity,
		maxWait:          maxWait,
		pollInterval:     pollInterval,
		globalConcurrent: globalConcurrent,
	}
}

var defaultTest40TrafficShaper = newTest40TrafficShaper(
	test40QueueCapacity,
	test40GlobalConcurrency,
	test40QueueMaxWait,
	test40QueuePollInterval,
)

func test40TrafficRuleForModel(model string, trafficClass test40TrafficClass) (string, test40TrafficConfig, bool) {
	model = strings.ToLower(strings.TrimSpace(model))
	if !strings.HasPrefix(model, "claude-") {
		return "", test40TrafficConfig{}, false
	}

	var family string
	var configs test40ModelTrafficConfig
	switch {
	case model == "claude-sonnet-5" || strings.HasPrefix(model, "claude-sonnet-5-"):
		family, configs = "sonnet-5", test40Sonnet5Config
	case model == "claude-opus-5" || strings.HasPrefix(model, "claude-opus-5-"):
		family, configs = "opus-5", test40Opus5Config
	case strings.HasPrefix(model, "claude-opus-"):
		family, configs = "opus", test40OpusConfig
	case strings.HasPrefix(model, "claude-sonnet-"):
		family, configs = "sonnet", test40SonnetConfig
	case strings.HasPrefix(model, "claude-fable-"):
		family, configs = "fable", test40FableConfig
	case strings.HasPrefix(model, "claude-haiku-"):
		family, configs = "haiku", test40HaikuConfig
	default:
		family, configs = "claude-default", test40ClaudeDefault
	}

	if trafficClass == test40TrafficClassLoose {
		return family, configs.loose, true
	}
	return family, configs.strict, true
}

func test40RequestsPerMinute(config test40TrafficConfig) int {
	return config.ratePerSecond * 60 / config.requestCost
}

func test40HasMeaningfulCacheControl(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if strings.EqualFold(key, "cache_control") {
				cacheControl, ok := child.(map[string]any)
				if !ok {
					continue
				}
				cacheType, ok := cacheControl["type"].(string)
				if ok && strings.TrimSpace(cacheType) != "" {
					return true
				}
			}
			if test40HasMeaningfulCacheControl(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if test40HasMeaningfulCacheControl(child) {
				return true
			}
		}
	}
	return false
}

func test40RequestHasCacheIntent(request dto.Request) bool {
	if request == nil {
		return false
	}
	payload, err := common.Marshal(request)
	if err != nil {
		return false
	}
	var decoded any
	if err := common.Unmarshal(payload, &decoded); err != nil {
		return false
	}
	return test40HasMeaningfulCacheControl(decoded)
}

// IsTest40StrictTrafficRequest classifies only non-streaming requests without
// a meaningful cache_control object as strict traffic.
func IsTest40StrictTrafficRequest(request dto.Request, isStream bool) bool {
	return !isStream && !test40RequestHasCacheIntent(request)
}

func (s *test40TrafficShaper) semaphoreFor(key string, capacity int) *test40Semaphore {
	value, _ := s.semaphores.LoadOrStore(key, &test40Semaphore{slots: make(chan struct{}, capacity)})
	return value.(*test40Semaphore)
}

func (s *test40TrafficShaper) waitQueueFor(key string) *test40WaitQueue {
	value, _ := s.waitQueues.LoadOrStore(key, &test40WaitQueue{slots: make(chan struct{}, s.queueCapacity)})
	return value.(*test40WaitQueue)
}

func (s *test40TrafficShaper) allowMemoryRate(key string, config test40TrafficConfig) bool {
	now := time.Now()
	value, _ := s.memoryBuckets.LoadOrStore(key, &test40MemoryBucket{
		tokens:     float64(config.burst),
		lastRefill: now,
	})
	bucket := value.(*test40MemoryBucket)
	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	elapsed := now.Sub(bucket.lastRefill).Seconds()
	bucket.tokens = math.Min(float64(config.burst), bucket.tokens+elapsed*float64(config.ratePerSecond))
	bucket.lastRefill = now
	if bucket.tokens < float64(config.requestCost) {
		return false
	}
	bucket.tokens -= float64(config.requestCost)
	return true
}

func (s *test40TrafficShaper) allowRate(ctx context.Context, key string, config test40TrafficConfig) bool {
	if s.allowRateOverride != nil {
		return s.allowRateOverride(ctx, key, config)
	}
	if ctx.Err() != nil {
		return false
	}
	if common.RedisEnabled && common.RDB != nil {
		allowed, err := limiter.New(ctx, common.RDB).Allow(
			ctx,
			key,
			limiter.WithCapacity(int64(config.burst)),
			limiter.WithRate(int64(config.ratePerSecond)),
			limiter.WithRequested(int64(config.requestCost)),
		)
		if err == nil {
			return allowed
		}
		common.SysLog(fmt.Sprintf("test40 Redis traffic shaping fallback: %v", err))
	}
	return s.allowMemoryRate(key, config)
}

func (s *test40TrafficShaper) tryAcquire(
	ctx context.Context,
	userID string,
	family string,
	trafficClass test40TrafficClass,
	config test40TrafficConfig,
) (func(), bool) {
	globalGate := s.semaphoreFor("test40:concurrency:"+userID+":all", s.globalConcurrent)
	modelGate := s.semaphoreFor(
		"test40:concurrency:"+userID+":"+string(trafficClass)+":"+family,
		config.maxConcurrent,
	)
	if !globalGate.tryAcquire() {
		return nil, false
	}
	if !modelGate.tryAcquire() {
		globalGate.release()
		return nil, false
	}

	rateKey := "test40:rate:" + userID + ":111:" + string(trafficClass) + ":" + family
	if !s.allowRate(ctx, rateKey, config) {
		modelGate.release()
		globalGate.release()
		return nil, false
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			modelGate.release()
			globalGate.release()
		})
	}, true
}

func test40BusyError(c *gin.Context, retryAfter int) *types.NewAPIError {
	c.Header("Retry-After", strconv.Itoa(retryAfter))
	return types.NewErrorWithStatusCode(
		fmt.Errorf("request queue is busy; retry after %d seconds", retryAfter),
		test40TrafficBusyErrorCode,
		http.StatusTooManyRequests,
		types.ErrOptionWithSkipRetry(),
		types.ErrOptionWithNoRecordErrorLog(),
	)
}

func test40CanceledError(err error) *types.NewAPIError {
	if err == nil {
		err = errors.New("request canceled while waiting for an upstream slot")
	}
	return types.NewErrorWithStatusCode(
		err,
		test40RequestCanceledErrorCode,
		http.StatusRequestTimeout,
		types.ErrOptionWithSkipRetry(),
		types.ErrOptionWithNoRecordErrorLog(),
	)
}

func (s *test40TrafficShaper) acquire(
	c *gin.Context,
	modelName string,
	channelID int,
	strict bool,
) (func(), *types.NewAPIError) {
	if common.GetContextKeyString(c, constant.ContextKeyUserName) != test40Username || channelID != test40UpstreamChannelID {
		return func() {}, nil
	}

	trafficClass := test40TrafficClassLoose
	if strict {
		trafficClass = test40TrafficClassStrict
	}
	family, config, targeted := test40TrafficRuleForModel(modelName, trafficClass)
	if !targeted {
		return func() {}, nil
	}

	ctx := context.Background()
	if c.Request != nil {
		ctx = c.Request.Context()
	}
	userID := strconv.Itoa(common.GetContextKeyInt(c, constant.ContextKeyUserId))
	if release, ok := s.tryAcquire(ctx, userID, family, trafficClass, config); ok {
		return release, nil
	}

	queue := s.waitQueueFor("test40:queue:" + userID)
	if !queue.tryJoin() {
		return nil, test40BusyError(c, config.retryAfter)
	}
	defer queue.leave()

	timer := time.NewTimer(s.maxWait)
	ticker := time.NewTicker(s.pollInterval)
	defer timer.Stop()
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, test40CanceledError(ctx.Err())
		case <-timer.C:
			return nil, test40BusyError(c, config.retryAfter)
		case <-ticker.C:
			if release, ok := s.tryAcquire(ctx, userID, family, trafficClass, config); ok {
				return release, nil
			}
		}
	}
}

// AcquireTest40TrafficPermit applies channel-specific shaping after each relay
// attempt selects its actual upstream channel. The returned release function is
// safe to call more than once.
func AcquireTest40TrafficPermit(c *gin.Context, modelName string, channelID int, strict bool) (func(), *types.NewAPIError) {
	return defaultTest40TrafficShaper.acquire(c, modelName, channelID, strict)
}
