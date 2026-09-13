package middleware

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// AdminMediaAuth authenticates native media requests, which cannot supply the
// dashboard's New-Api-User header. Only a signed login session is accepted;
// current database permissions are checked on every request.
func AdminMediaAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := sessions.Default(c).Get("id").(int)
		if !ok || id <= 0 {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		user, err := model.GetUserById(id, false)
		if err != nil || user.Status != common.UserStatusEnabled || user.Role < common.RoleAdminUser {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Set("id", user.Id)
		c.Set("role", user.Role)
		c.Next()
	}
}
