package config

import (
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Addr          string
	DatabaseURL   string
	StorageDir    string
	UploadDir     string
	DownloadDir   string
	AdminPassword string
	SessionSecret string
	StaticDir     string
	AuthRequired  bool
}

func Load() Config {
	base := env("TIKAWANG_BASE_DIR", ".")
	storage := env("TIKAWANG_STORAGE_DIR", filepath.Join(base, "storage"))
	dbURL := env("TIKAWANG_DATABASE_URL", "sqlite:///"+filepath.ToSlash(filepath.Join(base, "data", "tikawang.db")))
	environment := env("TIKAWANG_ENV", "production")
	return Config{
		Addr:          env("TIKAWANG_ADDR", "0.0.0.0:18743"),
		DatabaseURL:   dbURL,
		StorageDir:    storage,
		UploadDir:     filepath.Join(storage, "uploads"),
		DownloadDir:   filepath.Join(storage, "downloads"),
		AdminPassword: env("TIKAWANG_AUTH_PASSWORD", "Wgs0405java"),
		SessionSecret: env("TIKAWANG_SESSION_SECRET", "local-dev-secret-change-me"),
		StaticDir:     env("TIKAWANG_STATIC_DIR", filepath.Join(base, "web", "dist")),
		AuthRequired:  !strings.EqualFold(strings.TrimSpace(environment), "development"),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
