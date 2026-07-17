package handlers

import (
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"faka/server/internal/config"
	"faka/server/internal/models"
	"faka/server/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CardHandler struct {
	DB  *gorm.DB
	Cfg config.Config
}

func (h *CardHandler) List(c *gin.Context) {
	space, err := resolveSpace(c, h.DB)
	if err != nil {
		serviceFail(c, err)
		return
	}
	page, size := parsePage(c, 200, []int{50, 100, 200, 500, 1000, 2000, 5000, 10000})
	q := h.DB.Model(&models.Card{}).Where("space_id = ?", space.ID)
	if s := strings.TrimSpace(c.Query("q")); s != "" {
		q = q.Where("code LIKE ?", "%"+strings.ToUpper(s)+"%")
	}
	if s := c.Query("status"); s != "" {
		q = q.Where("status = ?", s)
	}
	if start := c.Query("start"); start != "" {
		if t, err := time.Parse("2006-01-02", start); err == nil {
			q = q.Where("used_at >= ?", t)
		}
	}
	if end := c.Query("end"); end != "" {
		if t, err := time.Parse("2006-01-02", end); err == nil {
			q = q.Where("used_at < ?", t.Add(24*time.Hour))
		}
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		serviceFail(c, err)
		return
	}
	var cards []models.Card
	if err := q.Order("created_at desc, id desc").Offset((page - 1) * size).Limit(size).Find(&cards).Error; err != nil {
		serviceFail(c, err)
		return
	}
	items := make([]gin.H, 0, len(cards))
	for _, card := range cards {
		items = append(items, gin.H{
			"id":         card.ID,
			"code":       card.Code,
			"file_count": card.FileCount,
			"status":     card.Status,
			"created_at": formatTimeVal(card.CreatedAt),
			"used_at":    formatTime(card.UsedAt),
			"voided_at":  formatTime(card.VoidedAt),
		})
	}
	ok(c, gin.H{"cards": items, "pagination": pagination(total, page, size), "current_space": space})
}

func (h *CardHandler) Create(c *gin.Context) {
	space, err := resolveSpace(c, h.DB)
	if err != nil {
		serviceFail(c, err)
		return
	}
	var body struct {
		FileCount int `json:"file_count"`
		Quantity  int `json:"quantity"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, "参数错误")
		return
	}
	if body.Quantity <= 0 {
		body.Quantity = 1
	}
	cards, err := services.CreateCards(h.DB, space, body.FileCount, body.Quantity)
	if err != nil {
		serviceFail(c, err)
		return
	}
	ok(c, gin.H{"message": "已生成 " + strconv.Itoa(len(cards)) + " 张卡密", "count": len(cards)})
}

func (h *CardHandler) UpdateStatus(c *gin.Context) {
	space, err := resolveSpace(c, h.DB)
	if err != nil {
		serviceFail(c, err)
		return
	}
	var body struct {
		IDs          []any  `json:"ids"`
		TargetStatus string `json:"target_status"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, "参数错误")
		return
	}
	ids := parseIDs(body.IDs)
	if len(ids) == 0 {
		fail(c, http.StatusBadRequest, "请选择要修改的卡密")
		return
	}
	if len(ids) > 10000 {
		fail(c, http.StatusBadRequest, "单次最多操作 10000 张卡密")
		return
	}
	valid := map[string]bool{"available": true, "pending": true, "sold": true, "voided": true}
	if !valid[body.TargetStatus] {
		fail(c, http.StatusBadRequest, "目标状态无效")
		return
	}
	var cards []models.Card
	if err := h.DB.Where("id IN ? AND space_id = ?", ids, space.ID).Find(&cards).Error; err != nil {
		serviceFail(c, err)
		return
	}
	if len(cards) != len(ids) {
		fail(c, http.StatusForbidden, "部分卡密不存在或不在当前空间")
		return
	}
	ts := time.Now().UTC()
	values := map[string]any{"status": body.TargetStatus}
	switch body.TargetStatus {
	case "available", "pending":
		values["used_at"] = nil
		values["voided_at"] = nil
	case "sold":
		values["used_at"] = ts
		values["voided_at"] = nil
	case "voided":
		values["used_at"] = nil
		values["voided_at"] = ts
	}
	for _, chunk := range chunkUint(ids, 500) {
		if err := h.DB.Model(&models.Card{}).Where("id IN ? AND space_id = ?", chunk, space.ID).Updates(values).Error; err != nil {
			serviceFail(c, err)
			return
		}
	}
	ok(c, gin.H{"message": "已修改 " + strconv.Itoa(len(cards)) + " 张卡密状态"})
}

func (h *CardHandler) Download(c *gin.Context) {
	space, err := resolveSpace(c, h.DB)
	if err != nil {
		serviceFail(c, err)
		return
	}
	var body struct {
		IDs         []any `json:"ids"`
		MarkPending bool  `json:"mark_pending"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, "参数错误")
		return
	}
	ids := parseIDs(body.IDs)
	if len(ids) == 0 {
		fail(c, http.StatusBadRequest, "请选择要下载的卡密")
		return
	}
	var cards []models.Card
	if err := h.DB.Where("id IN ? AND space_id = ? AND status = ?", ids, space.ID, "available").Order("id asc").Find(&cards).Error; err != nil {
		serviceFail(c, err)
		return
	}
	if len(cards) == 0 {
		fail(c, http.StatusBadRequest, "没有可下载的卡密")
		return
	}
	codes := make([]string, 0, len(cards))
	idsToPending := make([]uint, 0, len(cards))
	for i := range cards {
		codes = append(codes, cards[i].Code)
		if body.MarkPending {
			idsToPending = append(idsToPending, cards[i].ID)
		}
	}
	if len(idsToPending) > 0 {
		for _, chunk := range chunkUint(idsToPending, 500) {
			if err := h.DB.Model(&models.Card{}).Where("id IN ?", chunk).Updates(map[string]any{
				"status":    "pending",
				"used_at":   nil,
				"voided_at": nil,
			}).Error; err != nil {
				serviceFail(c, err)
				return
			}
		}
	}
	path, err := services.WriteCardCodesFile(h.Cfg.DownloadDir, codes)
	if err != nil {
		serviceFail(c, err)
		return
	}
	c.FileAttachment(path, filepath.Base(path))
}

func (h *CardHandler) Redemptions(c *gin.Context) {
	space, err := resolveSpace(c, h.DB)
	if err != nil {
		serviceFail(c, err)
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var card models.Card
	if err := h.DB.Where("id = ? AND space_id = ?", id, space.ID).First(&card).Error; err != nil {
		fail(c, http.StatusNotFound, "卡密不存在或不在当前空间")
		return
	}
	var redemptions []models.Redemption
	if err := h.DB.Where("card_id = ?", card.ID).Order("redeemed_at asc, id asc").Find(&redemptions).Error; err != nil {
		serviceFail(c, err)
		return
	}
	items := make([]gin.H, 0, len(redemptions))
	for _, r := range redemptions {
		fileCount := 0
		if strings.TrimSpace(r.FileIDs) != "" {
			fileCount = len(strings.Split(r.FileIDs, ","))
		}
		format := "CPA 文件"
		if r.OutputFormat == "sub" || strings.Contains(strings.ToLower(r.DownloadPath), "sub2api") {
			format = "SUB 文件"
		}
		items = append(items, gin.H{
			"id":            r.ID,
			"redeemed_at":   formatTimeVal(r.RedeemedAt),
			"output_format": format,
			"file_count":    fileCount,
			"status":        r.Status,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"card_code":     card.Code,
		"first_used_at": formatTime(card.UsedAt),
		"redemptions":   items,
	})
}
