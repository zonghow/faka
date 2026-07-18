package handlers

import (
	"net/http"
	"strconv"
	"time"

	"faka/server/internal/models"
	"faka/server/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const spaceCookie = "tikawang_space_id"

// displayLocation is fixed UTC+8 for all user-facing timestamps.
var displayLocation = time.FixedZone("UTC+8", 8*60*60)

func ok(c *gin.Context, data gin.H) {
	if data == nil {
		data = gin.H{}
	}
	data["ok"] = true
	c.JSON(http.StatusOK, data)
}

func fail(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"ok": false, "error": msg})
}

func serviceFail(c *gin.Context, err error) {
	if se, ok := err.(*services.ServiceError); ok {
		fail(c, http.StatusBadRequest, se.Message)
		return
	}
	fail(c, http.StatusInternalServerError, err.Error())
}

func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.In(displayLocation).Format("2006-01-02 15:04:05")
}

func formatTimeVal(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(displayLocation).Format("2006-01-02 15:04:05")
}

// parseDateStart parses YYYY-MM-DD as 00:00:00 in UTC+8 and returns the UTC instant.
func parseDateStart(s string) (time.Time, bool) {
	t, err := time.ParseInLocation("2006-01-02", s, displayLocation)
	if err != nil {
		return time.Time{}, false
	}
	return t.UTC(), true
}

// dayBoundsUTC8 returns [todayStart, tomorrowStart) as UTC instants for the current UTC+8 calendar day.
func dayBoundsUTC8(now time.Time) (todayStart, tomorrowStart time.Time) {
	local := now.In(displayLocation)
	startLocal := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, displayLocation)
	return startLocal.UTC(), startLocal.Add(24 * time.Hour).UTC()
}

func resolveSpace(c *gin.Context, db *gorm.DB) (*models.Space, error) {
	var spaces []models.Space
	if err := db.Order("id asc").Find(&spaces).Error; err != nil {
		return nil, err
	}
	if len(spaces) == 0 {
		return nil, services.Err("没有可用空间")
	}
	cookie, _ := c.Cookie(spaceCookie)
	if cookie != "" {
		if id, err := strconv.ParseUint(cookie, 10, 64); err == nil {
			for i := range spaces {
				if spaces[i].ID == uint(id) {
					return &spaces[i], nil
				}
			}
		}
	}
	return &spaces[0], nil
}

func parseIDs(raw []any) []uint {
	ids := make([]uint, 0, len(raw))
	for _, v := range raw {
		switch t := v.(type) {
		case float64:
			ids = append(ids, uint(t))
		case int:
			ids = append(ids, uint(t))
		case string:
			if n, err := strconv.ParseUint(t, 10, 64); err == nil {
				ids = append(ids, uint(n))
			}
		}
	}
	return ids
}

func chunkUint(ids []uint, size int) [][]uint {
	if size <= 0 {
		size = 500
	}
	if len(ids) == 0 {
		return nil
	}
	out := make([][]uint, 0, (len(ids)+size-1)/size)
	for start := 0; start < len(ids); start += size {
		end := start + size
		if end > len(ids) {
			end = len(ids)
		}
		out = append(out, ids[start:end])
	}
	return out
}

func parsePage(c *gin.Context, defaultSize int, allowed []int) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", strconv.Itoa(defaultSize)))
	okSize := false
	for _, a := range allowed {
		if size == a {
			okSize = true
			break
		}
	}
	if !okSize {
		size = defaultSize
	}
	return page, size
}

func pagination(total int64, page, size int) gin.H {
	totalPages := int((total + int64(size) - 1) / int64(size))
	if totalPages < 1 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}
	return gin.H{
		"page":        page,
		"page_size":   size,
		"total":       total,
		"total_pages": totalPages,
		"has_prev":    page > 1,
		"has_next":    page < totalPages,
		"prev_page":   max(1, page-1),
		"next_page":   min(totalPages, page+1),
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
