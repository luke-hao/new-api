package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
)

func TestRequestedPlaygroundGroupPrefersParsedRequestGroup(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
	common.SetContextKey(c, constant.ContextKeyTokenGroup, "video")
	if got := requestedPlaygroundGroup(c); got != "video" {
		t.Fatalf("requestedPlaygroundGroup() = %q, want video", got)
	}
}

func TestRequestedPlaygroundGroupFallsBackToUsingGroup(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
	if got := requestedPlaygroundGroup(c); got != "default" {
		t.Fatalf("requestedPlaygroundGroup() = %q, want default", got)
	}
}
