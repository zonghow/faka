package handlers

import (
	"time"

	"faka/server/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type DashboardHandler struct {
	DB *gorm.DB
}

func (h *DashboardHandler) Stats(c *gin.Context) {
	space, err := resolveSpace(c, h.DB)
	if err != nil {
		serviceFail(c, err)
		return
	}
	// Day buckets use UTC+8 calendar days.
	todayStart, tomorrowStart := dayBoundsUTC8(time.Now().UTC())
	yesterdayStart := todayStart.Add(-24 * time.Hour)

	countWhere := func(extra string, args ...any) int64 {
		q := h.DB.Model(&models.ManagedFile{}).Where("space_id = ?", space.ID)
		if extra != "" {
			q = q.Where(extra, args...)
		}
		var n int64
		_ = q.Count(&n).Error
		return n
	}

	ok(c, gin.H{
		"current_space": space,
		"stats": gin.H{
			"总上传文件数": countWhere(""),
			"昨日上传文件数": countWhere("uploaded_at >= ? AND uploaded_at < ?", yesterdayStart, todayStart),
			"今日上传文件数": countWhere("uploaded_at >= ? AND uploaded_at < ?", todayStart, tomorrowStart),
			"总售出文件数":  countWhere("status = ?", "sold"),
			"昨日售出文件数": countWhere("sold_at >= ? AND sold_at < ?", yesterdayStart, todayStart),
			"今日售出文件数": countWhere("sold_at >= ? AND sold_at < ?", todayStart, tomorrowStart),
		},
	})
}
