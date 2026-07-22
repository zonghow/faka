package handlers

import (
	"io"
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

type FileHandler struct {
	DB  *gorm.DB
	Cfg config.Config
}

func (h *FileHandler) List(c *gin.Context) {
	space, err := resolveSpace(c, h.DB)
	if err != nil {
		serviceFail(c, err)
		return
	}
	page, size := parsePage(c, 200, []int{50, 100, 200, 500, 1000, 2000, 5000, 10000})
	q := h.DB.Model(&models.ManagedFile{}).Where("space_id = ?", space.ID)
	if s := strings.TrimSpace(c.Query("q")); s != "" {
		q = q.Where("original_name LIKE ?", "%"+s+"%")
	}
	if code := strings.TrimSpace(c.Query("card_code")); code != "" {
		var card models.Card
		err := h.DB.Where("code = ? AND space_id = ?", strings.ToUpper(code), space.ID).First(&card).Error
		if err != nil {
			q = q.Where("sold_card_id = ?", -1)
		} else {
			q = q.Where("sold_card_id = ?", card.ID)
		}
	}
	if s := c.Query("status"); s != "" {
		q = q.Where("status = ?", s)
	}
	if start := c.Query("start"); start != "" {
		if t, ok := parseDateStart(start); ok {
			q = q.Where("uploaded_at >= ?", t)
		}
	}
	if end := c.Query("end"); end != "" {
		if t, ok := parseDateStart(end); ok {
			q = q.Where("uploaded_at < ?", t.Add(24*time.Hour))
		}
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		serviceFail(c, err)
		return
	}
	var files []models.ManagedFile
	if err := q.Preload("SoldCard").Order("uploaded_at desc, id desc").Offset((page - 1) * size).Limit(size).Find(&files).Error; err != nil {
		serviceFail(c, err)
		return
	}
	items := make([]gin.H, 0, len(files))
	for _, f := range files {
		soldCode := ""
		soldUsed := ""
		if f.SoldCard != nil {
			soldCode = f.SoldCard.Code
			soldUsed = formatTime(f.SoldCard.UsedAt)
		}
		if soldUsed == "" {
			soldUsed = formatTime(f.SoldAt)
		}
		items = append(items, gin.H{
			"id":            f.ID,
			"original_name": f.OriginalName,
			"status":        f.Status,
			"sold_at":       soldUsed,
			"sold_card":     soldCode,
			"voided_at":     formatTime(f.VoidedAt),
			"uploaded_at":   formatTimeVal(f.UploadedAt),
		})
	}
	ok(c, gin.H{"files": items, "pagination": pagination(total, page, size), "current_space": space})
}

func (h *FileHandler) UploadRecords(c *gin.Context) {
	space, err := resolveSpace(c, h.DB)
	if err != nil {
		serviceFail(c, err)
		return
	}
	page, size := parsePage(c, 50, []int{20, 50, 100, 200})
	q := h.DB.Model(&models.UploadRecord{}).Where("space_id = ?", space.ID)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		serviceFail(c, err)
		return
	}
	var records []models.UploadRecord
	if err := q.Order("created_at desc, id desc").Offset((page - 1) * size).Limit(size).Find(&records).Error; err != nil {
		serviceFail(c, err)
		return
	}
	items := make([]gin.H, 0, len(records))
	for _, record := range records {
		items = append(items, gin.H{
			"id":                record.ID,
			"filename":          record.Filename,
			"created_count":     record.CreatedCount,
			"overwritten_count": record.OverwrittenCount,
			"created_at":        formatTimeVal(record.CreatedAt),
		})
	}
	ok(c, gin.H{"records": items, "pagination": pagination(total, page, size)})
}

func (h *FileHandler) Upload(c *gin.Context) {
	space, err := resolveSpace(c, h.DB)
	if err != nil {
		serviceFail(c, err)
		return
	}
	form, err := c.MultipartForm()
	if err != nil {
		fail(c, http.StatusBadRequest, "请选择要上传的文件")
		return
	}
	files := form.File["file"]
	if len(files) == 0 {
		fail(c, http.StatusBadRequest, "请选择要上传的文件")
		return
	}
	const maxBytes = 500 * 1024 * 1024
	totalBytes := 0
	totalCreated := 0
	totalDuplicated := 0
	allErrors := []string{}
	for _, fh := range files {
		f, err := fh.Open()
		if err != nil {
			allErrors = append(allErrors, fh.Filename+": 读取失败")
			continue
		}
		raw, err := io.ReadAll(f)
		_ = f.Close()
		if err != nil {
			allErrors = append(allErrors, fh.Filename+": 读取失败")
			continue
		}
		totalBytes += len(raw)
		if totalBytes > maxBytes {
			fail(c, http.StatusBadRequest, "单批上传文件总大小不能超过 500MB")
			return
		}
		created, duplicated, errs, err := services.ImportUpload(h.DB, space, h.Cfg.UploadDir, fh.Filename, raw)
		if err != nil {
			if se, ok := err.(*services.ServiceError); ok {
				allErrors = append(allErrors, fh.Filename+": "+se.Message)
			} else {
				allErrors = append(allErrors, fh.Filename+": "+err.Error())
			}
			continue
		}
		totalCreated += created
		totalDuplicated += duplicated
		allErrors = append(allErrors, errs...)
		if created+duplicated > 0 {
			record := models.UploadRecord{
				SpaceID:          space.ID,
				Filename:         filepath.Base(fh.Filename),
				CreatedCount:     created,
				OverwrittenCount: duplicated,
				CreatedAt:        services.NowUTC(),
			}
			if err := h.DB.Create(&record).Error; err != nil {
				serviceFail(c, err)
				return
			}
		}
	}
	totalProcessed := totalCreated + totalDuplicated
	if totalProcessed == 0 && len(allErrors) > 0 {
		msg := allErrors[0]
		if len(allErrors) > 1 {
			msg = strings.Join(allErrors[:min(3, len(allErrors))], "；")
		}
		fail(c, http.StatusBadRequest, msg)
		return
	}
	msg := "新增 " + strconv.Itoa(totalCreated) + " 个，覆盖 " + strconv.Itoa(totalDuplicated) + " 个"
	if len(allErrors) > 0 {
		msg += "，部分失败：" + strings.Join(allErrors[:min(3, len(allErrors))], "；")
	}
	ok(c, gin.H{
		"message":    msg,
		"imported":   totalCreated,
		"created":    totalCreated,
		"duplicated": totalDuplicated,
	})
}

func (h *FileHandler) UpdateStatus(c *gin.Context) {
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
		fail(c, http.StatusBadRequest, "请选择要修改的文件")
		return
	}
	if len(ids) > 10000 {
		fail(c, http.StatusBadRequest, "单次最多操作 10000 个文件")
		return
	}
	valid := map[string]bool{"available": true, "locked": true, "sold": true, "voided": true}
	if !valid[body.TargetStatus] {
		fail(c, http.StatusBadRequest, "目标状态无效")
		return
	}
	var files []models.ManagedFile
	if err := h.DB.Where("id IN ? AND space_id = ?", ids, space.ID).Find(&files).Error; err != nil {
		serviceFail(c, err)
		return
	}
	if len(files) != len(ids) {
		fail(c, http.StatusForbidden, "部分文件不存在或不在当前空间")
		return
	}
	ts := time.Now().UTC()
	values := map[string]any{"status": body.TargetStatus}
	switch body.TargetStatus {
	case "available", "locked":
		values["sold_at"] = nil
		values["voided_at"] = nil
		values["sold_card_id"] = nil
	case "sold":
		values["sold_at"] = ts
		values["voided_at"] = nil
	case "voided":
		values["sold_at"] = nil
		values["sold_card_id"] = nil
		values["voided_at"] = ts
	}
	for _, chunk := range chunkUint(ids, 500) {
		if err := h.DB.Model(&models.ManagedFile{}).Where("id IN ? AND space_id = ?", chunk, space.ID).Updates(values).Error; err != nil {
			serviceFail(c, err)
			return
		}
	}
	ok(c, gin.H{"message": "已修改 " + strconv.Itoa(len(files)) + " 个文件状态"})
}

func (h *FileHandler) Download(c *gin.Context) {
	space, err := resolveSpace(c, h.DB)
	if err != nil {
		serviceFail(c, err)
		return
	}
	var body struct {
		IDs      []any `json:"ids"`
		MarkSold bool  `json:"mark_sold"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, "参数错误")
		return
	}
	ids := parseIDs(body.IDs)
	if len(ids) == 0 {
		fail(c, http.StatusBadRequest, "请选择要下载的文件")
		return
	}
	var files []models.ManagedFile
	if err := h.DB.Where("id IN ? AND space_id = ? AND status IN ?", ids, space.ID, []string{"available", "locked"}).Order("id asc").Find(&files).Error; err != nil {
		serviceFail(c, err)
		return
	}
	if len(files) == 0 {
		fail(c, http.StatusBadRequest, "没有可下载的文件")
		return
	}
	ts := time.Now().UTC()
	fileIDs := make([]uint, 0, len(files))
	for _, f := range files {
		fileIDs = append(fileIDs, f.ID)
	}
	values := map[string]any{"latest_download_at": ts}
	if body.MarkSold {
		values["status"] = "sold"
		values["sold_at"] = ts
	}
	for _, chunk := range chunkUint(fileIDs, 500) {
		if err := h.DB.Model(&models.ManagedFile{}).Where("id IN ?", chunk).Updates(values).Error; err != nil {
			serviceFail(c, err)
			return
		}
	}
	if len(files) == 1 {
		item := files[0]
		c.FileAttachment(item.StoredPath, services.JSONDownloadName(item.OriginalName))
		return
	}
	path, err := services.ZipFiles(h.Cfg.DownloadDir, files)
	if err != nil {
		serviceFail(c, err)
		return
	}
	c.FileAttachment(path, filepath.Base(path))
}
