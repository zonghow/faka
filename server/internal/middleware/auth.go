package middleware

import (
	"net/http"
	"strings"

	"faka/server/internal/auth"

	"github.com/gin-gonic/gin"
)

// RequireAuth allows either a valid session cookie or a matching admin password
// supplied via headers for programmatic API access:
//   - X-Admin-Password: <password>
//   - Authorization: Bearer <password>
//   - Authorization: Password <password>
func RequireAuth(m *auth.Manager, adminPassword string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if m.Parse(c.Request) {
			c.Next()
			return
		}
		if adminPassword != "" && passwordMatches(c, adminPassword) {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"ok": false, "error": "未登录"})
	}
}

func passwordMatches(c *gin.Context, adminPassword string) bool {
	if p := strings.TrimSpace(c.GetHeader("X-Admin-Password")); p != "" && p == adminPassword {
		return true
	}
	authz := strings.TrimSpace(c.GetHeader("Authorization"))
	if authz == "" {
		return false
	}
	const bearer = "Bearer "
	const passwordPrefix = "Password "
	switch {
	case strings.HasPrefix(authz, bearer):
		return strings.TrimSpace(authz[len(bearer):]) == adminPassword
	case strings.HasPrefix(authz, passwordPrefix):
		return strings.TrimSpace(authz[len(passwordPrefix):]) == adminPassword
	case strings.EqualFold(authz, "password "+adminPassword):
		return true
	default:
		// raw password in Authorization (discouraged but convenient)
		return authz == adminPassword
	}
}
