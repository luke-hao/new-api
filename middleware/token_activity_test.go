package middleware

import (
	"context"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestKeyConsoleActivityRoutes(t *testing.T) {
	for _, test := range []struct {
		method, path string
		tracked      bool
	}{
		{"POST", "/v1/responses", true}, {"POST", "/v1/messages", true}, {"POST", "/v1beta/models/x:generateContent", true},
		{"POST", "/v1/videos", true}, {"GET", "/v1/realtime", true}, {"GET", "/v1/models", false},
		{"POST", "/v1/messages/count_tokens", false}, {"GET", "/speedtest/ping", false}, {"POST", "/suno/fetch", false},
		{"POST", "/api/token/batch/status", false},
	} {
		t.Run(test.path, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(test.method, test.path, nil)
			c.Set("token_id", 10001)
			before := service.LiveTokenActivity.Snapshot([]int{10001})[10001]
			done := beginTokenActivity(c)
			during := service.LiveTokenActivity.Snapshot([]int{10001})[10001]
			if test.tracked && during.Active != before.Active+1 {
				t.Fatal(during)
			}
			if !test.tracked && during.Active != before.Active {
				t.Fatal(during)
			}
			done()
			after := service.LiveTokenActivity.Snapshot([]int{10001})[10001]
			if after.Active != before.Active {
				t.Fatal(after)
			}
		})
	}
}
func TestKeyConsoleActivityStreamAndPanic(t *testing.T) {
	started := make(chan struct{})
	finish := make(chan struct{})
	r := gin.New()
	r.Use(gin.Recovery())
	r.POST("/v1/responses", func(c *gin.Context) {
		c.Set("token_id", 10002)
		release := beginTokenActivity(c)
		defer release()
		close(started)
		<-finish
		panic("upstream disconnected")
	})
	ended := make(chan struct{})
	go func() {
		defer close(ended)
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/v1/responses", nil))
	}()
	<-started
	if m := service.LiveTokenActivity.Snapshot([]int{10002})[10002]; m.Active != 1 {
		t.Fatal(m)
	}
	close(finish)
	<-ended
	if m := service.LiveTokenActivity.Snapshot([]int{10002})[10002]; m.Active != 0 {
		t.Fatal(m)
	}
}

// Exercise an actual streaming HTTP connection and cancellation, with internal
// attempts inside a single tracked request.
func TestKeyConsoleActivityClientDisconnectAndRetry(t *testing.T) {
	const id = 10003
	finished := make(chan struct{})
	seen := make(chan service.TokenActivity, 3)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("token_id", id)
		release := beginTokenActivity(c)
		defer func() { release(); close(finished) }()
		c.Next()
	})
	r.POST("/v1/responses", func(c *gin.Context) {
		for attempt := 0; attempt < 3; attempt++ {
			seen <- service.LiveTokenActivity.Snapshot([]int{id})[id]
		}
		c.Header("Content-Type", "text/event-stream")
		c.Writer.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(c.Writer, "data: pending\n\n")
		c.Writer.Flush()
		<-c.Request.Context().Done()
	})
	server := httptest.NewServer(r)
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/v1/responses", nil)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	for attempt := 0; attempt < 3; attempt++ {
		m := <-seen
		if m.Active != 1 || m.RPM != 1 {
			t.Fatalf("attempt %d: %+v", attempt, m)
		}
	}
	cancel()
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("stream not released after client disconnect")
	}
	if m := service.LiveTokenActivity.Snapshot([]int{id})[id]; m.Active != 0 || m.RPM != 1 {
		t.Fatal(m)
	}
}
