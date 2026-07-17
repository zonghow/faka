package middleware

import (
	"net/http"

	"faka/server/internal/auth"

	"github.com/gin-gonic/gin"
)

func RequireAuth(m *auth.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.Parse(c.Request) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"ok": false, "error": "未登录"})
			return
		}
		c.Next()
	}
}
