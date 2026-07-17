package services

import (
	"os"
	"strings"
	"time"
	"unicode"

	"faka/server/internal/models"

	"gorm.io/gorm"
)

func NowUTC() time.Time { return time.Now().UTC() }

func trim(s string) string { return strings.TrimSpace(s) }

func upper(s string) string { return strings.ToUpper(s) }

func AddAudit(db *gorm.DB, spaceID *uint, action, targetType string, targetID *uint, detail string) {
	var d *string
	if detail != "" {
		d = &detail
	}
	_ = db.Create(&models.AuditLog{
		SpaceID:    spaceID,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Detail:     d,
		CreatedAt:  NowUTC(),
	}).Error
}

func removePath(path string) error {
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	return os.Remove(path)
}

func onlyASCIILetters(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			b.WriteRune(unicode.ToUpper(r))
		}
	}
	return b.String()
}
