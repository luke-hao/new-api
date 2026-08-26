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
	SetVideoRouter(engine)

	tests := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPost, path: "/pg/videos", body: `{"model":"sora-2","prompt":"test"}`},
		{method: http.MethodGet, path: "/pg/videos/task_example"},
		{method: http.MethodPost, path: "/v1/videos", body: `{"model":"sora-2","prompt":"test"}`},
		{method: http.MethodGet, path: "/v1/videos/task_example"},
	}
	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")

			engine.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusUnauthorized, recorder.Code)
			if strings.HasPrefix(test.path, "/v1/") {
				require.Contains(t, recorder.Body.String(), `"error"`)
			} else {
				require.Contains(t, recorder.Body.String(), `"success":false`)
			}
		})
	}
}
