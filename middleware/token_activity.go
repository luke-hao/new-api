package middleware

import (
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

// Track one authenticated relay request, outside retry loops and until stream close.
func beginTokenActivity(c *gin.Context) func() {
	path := c.Request.URL.Path
	supported := false
	for _, prefix := range []string{"/v1/", "/v1beta/", "/mj/", "/suno/", "/kling/", "/jimeng/"} {
		if strings.HasPrefix(path, prefix) {
			supported = true
			break
		}
	}
	if !supported || strings.HasSuffix(path, "/count_tokens") {
		return func() {}
	}
	if c.Request.Method != http.MethodPost && path != "/v1/realtime" {
		return func() {}
	}
	// Task polling uses POST on some protocols, but is not a model invocation.
	if strings.Contains(path, "/fetch") || strings.Contains(path, "/task/") || strings.Contains(path, "/list-by-condition") {
		return func() {}
	}
	id := c.GetInt("token_id")
	if id <= 0 {
		return func() {}
	}
	return service.LiveTokenActivity.Begin(id)
}
