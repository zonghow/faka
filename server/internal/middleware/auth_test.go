package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"faka/server/internal/auth"

	"github.com/gin-gonic/gin"
)

func TestRequireAuthAllowsDevelopmentRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequireAuth(auth.NewManager("test-secret"), "test-password", false))
	router.GET("/admin", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/admin", nil))

	if response.Code != http.StatusNoContent {
		t.Fatalf("expected development request to pass, got %d", response.Code)
	}
}

func TestRequireAuthRejectsUnauthenticatedProductionRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequireAuth(auth.NewManager("test-secret"), "test-password", true))
	router.GET("/admin", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/admin", nil))

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected production request to be rejected, got %d", response.Code)
	}
}
