package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCacheHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		path     string
		expected string
	}{
		{"/static/js/index.abc123.js", "public, max-age=31536000, immutable"},
		{"/kele-mascot-v2.webp", "public, max-age=31536000, immutable"},
		{"/kele-mascot-v2-64.png", "public, max-age=31536000, immutable"},
		{"/logo.png", "no-store, no-cache, must-revalidate, max-age=0"},
		{"/", "no-store, no-cache, must-revalidate, max-age=0"},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest("GET", test.path, nil)

			Cache()(context)

			require.Equal(t, test.expected, recorder.Header().Get("Cache-Control"))
		})
	}
}
