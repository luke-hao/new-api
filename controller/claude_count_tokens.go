package controller

import (
	"github.com/QuantumNous/new-api/relay"
	"github.com/gin-gonic/gin"
)

// CountClaudeTokens uses the normal token/routing middleware, but deliberately
// bypasses generation precharge, settlement and generation retries.
func CountClaudeTokens(c *gin.Context) {
	if apiErr := relay.ClaudeCountTokens(c); apiErr != nil {
		c.JSON(apiErr.StatusCode, gin.H{"type": "error", "error": apiErr.ToClaudeError()})
	}
}
