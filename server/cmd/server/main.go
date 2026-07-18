package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"faka/server/internal/auth"
	"faka/server/internal/config"
	"faka/server/internal/db"
	"faka/server/internal/handlers"
	"faka/server/internal/middleware"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()
	if err := os.MkdirAll(cfg.UploadDir, 0o755); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(cfg.DownloadDir, 0o755); err != nil {
		log.Fatal(err)
	}

	database, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	if err := db.AutoMigrate(database); err != nil {
		log.Fatal(err)
	}
	if _, err := db.EnsureDefaultSpace(database); err != nil {
		log.Fatal(err)
	}

	sessions := auth.NewManager(cfg.SessionSecret)
	authH := &handlers.AuthHandler{Sessions: sessions, Password: cfg.AdminPassword}
	publicH := &handlers.PublicHandler{DB: database, Cfg: cfg}
	spaceH := &handlers.SpaceHandler{DB: database}
	cardH := &handlers.CardHandler{DB: database, Cfg: cfg}
	fileH := &handlers.FileHandler{DB: database, Cfg: cfg}
	dashH := &handlers.DashboardHandler{DB: database}

	r := gin.Default()
	// Keep most of large uploads on disk temp files; total size is enforced in handler (500MB).
	r.MaxMultipartMemory = 64 << 20
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://127.0.0.1:5173", "http://localhost:5173", "http://127.0.0.1:18743", "http://localhost:18743"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "X-Requested-With", "X-Admin-Password", "Authorization"},
		AllowCredentials: true,
	}))

	api := r.Group("/api")
	{
		api.GET("/inventory", publicH.Inventory)
		api.POST("/redeem", publicH.Redeem)
		api.POST("/auth/login", authH.Login)
		api.POST("/auth/logout", authH.Logout)
		api.GET("/auth/me", authH.Me)

		admin := api.Group("/admin")
		admin.Use(middleware.RequireAuth(sessions, cfg.AdminPassword))
		{
			admin.GET("/dashboard", dashH.Stats)
			admin.POST("/clear", spaceH.ClearCurrent)

			admin.GET("/spaces", spaceH.List)
			admin.POST("/spaces", spaceH.Create)
			admin.POST("/spaces/:id", spaceH.Update)
			admin.DELETE("/spaces/:id", spaceH.Delete)

			admin.GET("/cards", cardH.List)
			admin.POST("/cards", cardH.Create)
			admin.POST("/cards/status", cardH.UpdateStatus)
			admin.POST("/cards/download", cardH.Download)
			admin.GET("/cards/:id/redemptions", cardH.Redemptions)

			admin.GET("/files", fileH.List)
			admin.POST("/files/upload", fileH.Upload)
			admin.POST("/files/status", fileH.UpdateStatus)
			admin.POST("/files/download", fileH.Download)
		}
	}

	// SPA static
	staticDir := cfg.StaticDir
	if info, err := os.Stat(staticDir); err == nil && info.IsDir() {
		r.Static("/assets", filepath.Join(staticDir, "assets"))
		r.Static("/favicons", filepath.Join(staticDir, "favicons"))
		serveStaticFile := func(urlPath, filePath string) {
			r.GET(urlPath, func(c *gin.Context) {
				c.Header("Cache-Control", "public, max-age=86400")
				c.File(filePath)
			})
			r.HEAD(urlPath, func(c *gin.Context) {
				c.Header("Cache-Control", "public, max-age=86400")
				c.File(filePath)
			})
		}
		serveStaticFile("/favicon.svg", filepath.Join(staticDir, "favicon.svg"))
		r.NoRoute(func(c *gin.Context) {
			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "not found"})
				return
			}
			// Prefer real static files under dist before SPA fallback.
			rel := strings.TrimPrefix(c.Request.URL.Path, "/")
			if rel != "" && !strings.Contains(rel, "..") {
				candidate := filepath.Join(staticDir, rel)
				if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
					if strings.HasSuffix(strings.ToLower(rel), ".svg") {
						c.Header("Content-Type", "image/svg+xml")
						c.Header("Cache-Control", "public, max-age=86400")
					}
					c.File(candidate)
					return
				}
			}
			// Avoid caching SPA HTML as static assets at CDNs.
			c.Header("Cache-Control", "no-store")
			c.File(filepath.Join(staticDir, "index.html"))
		})
	}

	log.Printf("listening on %s", cfg.Addr)
	if err := r.Run(cfg.Addr); err != nil {
		log.Fatal(err)
	}
}
