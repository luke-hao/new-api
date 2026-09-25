package middleware

import (
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"testing"
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
