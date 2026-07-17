package db

import (
	"os"
	"path/filepath"
	"strings"

	"faka/server/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(databaseURL string) (*gorm.DB, error) {
	dsn := databaseURL
	if strings.HasPrefix(dsn, "sqlite:///") {
		dsn = strings.TrimPrefix(dsn, "sqlite:///")
	}
	if err := os.MkdirAll(filepath.Dir(dsn), 0o755); err != nil {
		return nil, err
	}
	// Enable SQLite WAL and busy timeout via DSN for better concurrent read/write.
	if !strings.Contains(dsn, "?") {
		dsn += "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)"
	}
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, err
	}
	// SQLite handles concurrency poorly with many writers; keep pool small.
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA temp_store=MEMORY",
	} {
		if _, err := sqlDB.Exec(pragma); err != nil {
			return nil, err
		}
	}
	return gdb, nil
}

func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&models.Space{},
		&models.ManagedFile{},
		&models.Card{},
		&models.Redemption{},
		&models.AuditLog{},
	); err != nil {
		return err
	}
	// Composite indexes for hot list/redeem paths.
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_files_space_status_uploaded ON files(space_id, status, uploaded_at)",
		"CREATE INDEX IF NOT EXISTS idx_files_space_status_id ON files(space_id, status, id)",
		"CREATE INDEX IF NOT EXISTS idx_cards_space_status_created ON cards(space_id, status, created_at)",
		"CREATE INDEX IF NOT EXISTS idx_files_sold_card_status ON files(sold_card_id, status)",
	}
	for _, stmt := range indexes {
		if err := db.Exec(stmt).Error; err != nil {
			return err
		}
	}
	return nil
}

func EnsureDefaultSpace(db *gorm.DB) (*models.Space, error) {
	var space models.Space
	err := db.Order("id asc").First(&space).Error
	if err == nil {
		return &space, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	space = models.Space{Name: "default", CardPrefix: "DEFAULT"}
	if err := db.Create(&space).Error; err != nil {
		return nil, err
	}
	return &space, nil
}
