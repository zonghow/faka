package handlers

import (
	"net/http"
	"time"

	"faka/server/internal/auth"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	Sessions     *auth.Manager
	Password     string
	AuthRequired bool
}

func (h *AuthHandler) Login(c *gin.Context) {
	var body struct {
		Password string `json:"password"`
	}
	if h.AuthRequired {
		if err := c.ShouldBindJSON(&body); err != nil || body.Password == "" {
			// also accept form
			body.Password = c.PostForm("password")
		}
		if body.Password == "" || body.Password != h.Password {
			fail(c, http.StatusUnauthorized, "密码错误")
			return
		}
	}
	cookie, err := h.Sessions.CreateCookie(true, 30*24*time.Hour)
	if err != nil {
		fail(c, http.StatusInternalServerError, "登录失败")
		return
	}
	http.SetCookie(c.Writer, cookie)
	ok(c, gin.H{"message": "登录成功"})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	http.SetCookie(c.Writer, h.Sessions.ClearCookie())
	ok(c, gin.H{"message": "已退出"})
}

func (h *AuthHandler) Me(c *gin.Context) {
	ok(c, gin.H{"authenticated": !h.AuthRequired || h.Sessions.Parse(c.Request)})
}
