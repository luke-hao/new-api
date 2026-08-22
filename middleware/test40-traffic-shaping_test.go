package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func TestTest40TrafficConfigForModel(t *testing.T) {
	tests := []struct {
		model             string
		targeted          bool
		family            string
		strictRequestsMin int
		strictConcurrency int
		looseRequestsMin  int
		looseConcurrency  int
	}{
		{model: "claude-sonnet-5", targeted: true, family: "sonnet-5", strictRequestsMin: 30, strictConcurrency: 8, looseRequestsMin: 60, looseConcurrency: 12},
		{model: "claude-sonnet-5-20260820", targeted: true, family: "sonnet-5", strictRequestsMin: 30, strictConcurrency: 8, looseRequestsMin: 60, looseConcurrency: 12},
		{model: "claude-opus-5", targeted: true, family: "opus-5", strictRequestsMin: 12, strictConcurrency: 4, looseRequestsMin: 20, looseConcurrency: 6},
		{model: "claude-opus-4-8", targeted: true, family: "opus", strictRequestsMin: 15, strictConcurrency: 4, looseRequestsMin: 30, looseConcurrency: 6},
		{model: "claude-sonnet-4-6", targeted: true, family: "sonnet", strictRequestsMin: 30, strictConcurrency: 8, looseRequestsMin: 60, looseConcurrency: 12},
		{model: "claude-fable-5", targeted: true, family: "fable", strictRequestsMin: 20, strictConcurrency: 6, looseRequestsMin: 40, looseConcurrency: 8},
		{model: "claude-haiku-4-5-20251001", targeted: true, family: "haiku", strictRequestsMin: 60, strictConcurrency: 10, looseRequestsMin: 120, looseConcurrency: 12},
		{model: "claude-future-1", targeted: true, family: "claude-default", strictRequestsMin: 20, strictConcurrency: 6, looseRequestsMin: 40, looseConcurrency: 8},
		{model: "gpt-5", targeted: false},
	}

	for _, tt := range tests {
		for _, trafficClass := range []test40TrafficClass{test40TrafficClassStrict, test40TrafficClassLoose} {
			family, config, targeted := test40TrafficRuleForModel(tt.model, trafficClass)
			if targeted != tt.targeted {
				t.Fatalf("model %q class %q targeted=%v, want %v", tt.model, trafficClass, targeted, tt.targeted)
			}
			if !targeted {
				continue
			}
			if family != tt.family {
				t.Fatalf("model %q class %q family=%q, want %q", tt.model, trafficClass, family, tt.family)
			}
			requestsPerMin, concurrency := tt.strictRequestsMin, tt.strictConcurrency
			if trafficClass == test40TrafficClassLoose {
				requestsPerMin, concurrency = tt.looseRequestsMin, tt.looseConcurrency
			}
			if got := test40RequestsPerMinute(config); got != requestsPerMin {
				t.Fatalf("model %q class %q requests/minute=%d, want %d", tt.model, trafficClass, got, requestsPerMin)
			}
			if config.maxConcurrent != concurrency {
				t.Fatalf("model %q class %q concurrency=%d, want %d", tt.model, trafficClass, config.maxConcurrent, concurrency)
			}
		}
	}
}

func TestIsTest40StrictTrafficRequest(t *testing.T) {
	tests := []struct {
		name    string
		request dto.Request
		stream  bool
		strict  bool
	}{
		{
			name:    "non-streaming without cache control",
			request: &dto.ClaudeRequest{Model: "claude-sonnet-5"},
			strict:  true,
		},
		{
			name:    "streaming without cache control",
			request: &dto.ClaudeRequest{Model: "claude-sonnet-5"},
			stream:  true,
			strict:  false,
		},
		{
			name: "top-level cache control",
			request: &dto.ClaudeRequest{
				Model:        "claude-sonnet-5",
				CacheControl: []byte(`{"type":"ephemeral"}`),
			},
			strict: false,
		},
		{
			name: "nested Claude cache control",
			request: &dto.ClaudeRequest{
				Model: "claude-sonnet-5",
				Messages: []dto.ClaudeMessage{{
					Role: "user",
					Content: []any{map[string]any{
						"type":          "text",
						"text":          "hello",
						"cache_control": map[string]any{"type": "ephemeral"},
					}},
				}},
			},
			strict: false,
		},
		{
			name: "OpenAI-compatible nested cache control",
			request: &dto.GeneralOpenAIRequest{
				Model: "claude-sonnet-5",
				Messages: []dto.Message{{
					Role: "user",
					Content: []any{map[string]any{
						"type":          "text",
						"text":          "hello",
						"cache_control": map[string]any{"type": "ephemeral"},
					}},
				}},
			},
			strict: false,
		},
		{
			name: "empty cache control does not downgrade",
			request: &dto.ClaudeRequest{
				Model:        "claude-sonnet-5",
				CacheControl: []byte(`{}`),
			},
			strict: true,
		},
		{
			name: "blank cache type does not downgrade",
			request: &dto.ClaudeRequest{
				Model:        "claude-sonnet-5",
				CacheControl: []byte(`{"type":" "}`),
			},
			strict: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTest40StrictTrafficRequest(tt.request, tt.stream); got != tt.strict {
				t.Fatalf("strict=%v, want %v", got, tt.strict)
			}
		})
	}
}

func TestTest40SemaphoreCapacityAndIdempotentRelease(t *testing.T) {
	shaper := newTest40TrafficShaper(1, 1, time.Second, time.Millisecond)
	shaper.allowRateOverride = func(context.Context, string, test40TrafficConfig) bool { return true }
	config := test40TrafficConfig{maxConcurrent: 1}

	release, ok := shaper.tryAcquire(context.Background(), "40", "sonnet", test40TrafficClassStrict, config)
	if !ok {
		t.Fatal("first acquisition should succeed")
	}
	if _, ok := shaper.tryAcquire(context.Background(), "40", "sonnet", test40TrafficClassStrict, config); ok {
		t.Fatal("second acquisition should be rejected while the permit is held")
	}
	release()
	release()
	if _, ok := shaper.tryAcquire(context.Background(), "40", "sonnet", test40TrafficClassStrict, config); !ok {
		t.Fatal("acquisition after idempotent release should succeed")
	}
}

func TestTest40WaitQueueCapacity(t *testing.T) {
	queue := &test40WaitQueue{slots: make(chan struct{}, 2)}
	if !queue.tryJoin() || !queue.tryJoin() {
		t.Fatal("first two waiters should enter the queue")
	}
	if queue.tryJoin() {
		t.Fatal("third waiter should be rejected from a full queue")
	}
	queue.leave()
	if !queue.tryJoin() {
		t.Fatal("a waiter should enter after a queue slot is released")
	}
}

func TestAcquireTest40TrafficPermitQueueFullAndTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	newContext := func() (*gin.Context, *httptest.ResponseRecorder) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
		common.SetContextKey(c, constant.ContextKeyUserName, test40Username)
		common.SetContextKey(c, constant.ContextKeyUserId, 40)
		return c, recorder
	}

	shaper := newTest40TrafficShaper(1, 1, 30*time.Millisecond, 5*time.Millisecond)
	shaper.allowRateOverride = func(context.Context, string, test40TrafficConfig) bool { return true }
	config := test40TrafficConfig{maxConcurrent: 1}
	held, ok := shaper.tryAcquire(context.Background(), "40", "sonnet-5", test40TrafficClassStrict, config)
	if !ok {
		t.Fatal("failed to occupy the test shaper")
	}
	defer held()

	firstContext, firstRecorder := newContext()
	firstDone := make(chan *types.NewAPIError, 1)
	go func() {
		_, err := shaper.acquire(firstContext, "claude-sonnet-5", test40UpstreamChannelID, true)
		firstDone <- err
	}()

	deadline := time.Now().Add(time.Second)
	queue := shaper.waitQueueFor("test40:queue:40")
	for len(queue.slots) != 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if len(queue.slots) != 1 {
		t.Fatal("first waiter did not enter the queue")
	}

	secondContext, secondRecorder := newContext()
	_, queueFullErr := shaper.acquire(secondContext, "claude-sonnet-5", test40UpstreamChannelID, true)
	if queueFullErr == nil || queueFullErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("queue-full error=%v, want status 429", queueFullErr)
	}
	if got := secondRecorder.Header().Get("Retry-After"); got == "" {
		t.Fatal("queue-full response is missing Retry-After")
	}

	timeoutErr := <-firstDone
	if timeoutErr == nil || timeoutErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("timeout error=%v, want status 429", timeoutErr)
	}
	if got := firstRecorder.Header().Get("Retry-After"); got == "" {
		t.Fatal("timeout response is missing Retry-After")
	}
}

func TestAcquireTest40TrafficPermitSkipsOtherTraffic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	common.SetContextKey(c, constant.ContextKeyUserName, test40Username)
	common.SetContextKey(c, constant.ContextKeyUserId, 40)

	shaper := newTest40TrafficShaper(1, 1, time.Millisecond, time.Millisecond)
	for _, test := range []struct {
		model     string
		channelID int
	}{
		{model: "claude-sonnet-5", channelID: 137},
		{model: "gpt-5", channelID: test40UpstreamChannelID},
	} {
		release, err := shaper.acquire(c, test.model, test.channelID, true)
		if err != nil {
			t.Fatalf("model=%q channel=%d returned error: %v", test.model, test.channelID, err)
		}
		release()
	}
}
