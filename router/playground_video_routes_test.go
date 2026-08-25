package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPlaygroundVideoRoutesRequireUserSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("playground-video-route-test"))))
	SetRelayRouter(engine)

	tests := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPost, path: "/pg/videos", body: `{"model":"sora-2","prompt":"test"}`},
		{method: http.MethodGet, path: "/pg/videos/task_example"},
	}
	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")

			engine.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusUnauthorized, recorder.Code)
			require.Contains(t, recorder.Body.String(), `"success":false`)
		})
	}
}
