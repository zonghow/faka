package services

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"faka/server/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var chineseNamePattern = regexp.MustCompile(`[\x{3400}-\x{9fff}\x{f900}-\x{faff}]`)

func DisplayJSONName(filename string) string {
	base := filepath.Base(filename)
	if strings.EqualFold(filepath.Ext(base), ".json") {
		return strings.TrimSuffix(base, filepath.Ext(base))
	}
	return base
}

func JSONDownloadName(filename string) string {
	base := filepath.Base(filename)
	if strings.EqualFold(filepath.Ext(base), ".json") {
		return base
	}
	return base + ".json"
}

func ValidateUploadFilename(filename string) error {
	display := filepath.Base(filename)
	if chineseNamePattern.MatchString(display) {
		return Err("文件名不能包含中文：" + display)
	}
	return nil
}

func ValidateJSONPayload(raw []byte) error {
	if !utf8Valid(raw) {
		return Err("JSON 必须使用 UTF-8 编码")
	}
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return Err("JSON 格式错误")
	}
	switch payload.(type) {
	case map[string]any, []any:
		return nil
	default:
		return Err("JSON 顶层必须是对象或数组")
	}
}

func SaveJSONFile(db *gorm.DB, space *models.Space, uploadDir, filename string, raw []byte, batchName *string) (*models.ManagedFile, bool, error) {
	if err := ValidateUploadFilename(filename); err != nil {
		return nil, false, err
	}
	if err := ValidateJSONPayload(raw); err != nil {
		return nil, false, err
	}
	if err := ensureDir(uploadDir); err != nil {
		return nil, false, err
	}
	original := DisplayJSONName(filename)
	ts := NowUTC()
	var existing models.ManagedFile
	err := db.Where("space_id = ? AND original_name = ?", space.ID, original).Order("id desc").First(&existing).Error
	if err == nil {
		if err := writeFile(existing.StoredPath, raw); err != nil {
			return nil, false, err
		}
		existing.UploadedAt = ts
		existing.BatchName = batchName
		if err := db.Save(&existing).Error; err != nil {
			return nil, false, err
		}
		id := existing.ID
		AddAudit(db, &space.ID, "replace_file", "file", &id, existing.OriginalName)
		return &existing, true, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, false, err
	}
	storedName := uuid.NewString() + "_" + original
	target := filepath.Join(uploadDir, storedName)
	if err := writeFile(target, raw); err != nil {
		return nil, false, err
	}
	item := &models.ManagedFile{
		OriginalName: original,
		StoredPath:   target,
		GeneratedAt:  ts,
		UploadedAt:   ts,
		SpaceID:      space.ID,
		Status:       "available",
		BatchName:    batchName,
	}
	if err := db.Create(item).Error; err != nil {
		return nil, false, err
	}
	id := item.ID
	AddAudit(db, &space.ID, "upload_file", "file", &id, item.OriginalName)
	return item, false, nil
}

func ImportUpload(db *gorm.DB, space *models.Space, uploadDir, filename string, raw []byte) (int, int, []string, error) {
	if err := ValidateUploadFilename(filename); err != nil {
		return 0, 0, nil, err
	}
	suffix := strings.ToLower(filepath.Ext(filename))
	errors := []string{}
	created := 0
	duplicated := 0
	switch suffix {
	case ".json":
		_, isDup, err := SaveJSONFile(db, space, uploadDir, filepath.Base(filename), raw, nil)
		if err != nil {
			return 0, 0, nil, err
		}
		if isDup {
			duplicated = 1
		} else {
			created = 1
		}
	case ".zip":
		batch := filepath.Base(filename)
		zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
		if err != nil {
			return 0, 0, nil, Err("ZIP 文件无法读取")
		}
		names := 0
		for _, f := range zr.File {
			if strings.HasSuffix(f.Name, "/") {
				continue
			}
			names++
			inner := filepath.Base(f.Name)
			if !strings.EqualFold(filepath.Ext(inner), ".json") {
				errors = append(errors, inner+": ZIP 内只允许 JSON 文件")
				continue
			}
			rc, err := f.Open()
			if err != nil {
				errors = append(errors, inner+": 无法读取")
				continue
			}
			content, err := io.ReadAll(rc)
			_ = rc.Close()
			if err != nil {
				errors = append(errors, inner+": 无法读取")
				continue
			}
			_, isDup, err := SaveJSONFile(db, space, uploadDir, inner, content, &batch)
			if err != nil {
				if se, ok := err.(*ServiceError); ok {
					errors = append(errors, inner+": "+se.Message)
				} else {
					errors = append(errors, inner+": "+err.Error())
				}
				continue
			}
			if isDup {
				duplicated++
			} else {
				created++
			}
		}
		if names == 0 {
			return 0, 0, nil, Err("ZIP 不能为空")
		}
	default:
		return 0, 0, nil, Err("只支持上传 .json 或 .zip 文件")
	}
	if created+duplicated == 0 && len(errors) > 0 {
		return 0, 0, nil, Err(strings.Join(errors, "；"))
	}
	return created, duplicated, errors, nil
}

func ApplyFileStatus(item *models.ManagedFile, target string, ts time.Time) {
	item.Status = target
	switch target {
	case "available", "locked":
		item.SoldAt = nil
		item.VoidedAt = nil
		item.SoldCardID = nil
	case "sold":
		if item.SoldAt == nil {
			item.SoldAt = &ts
		}
		item.VoidedAt = nil
	case "voided":
		item.SoldAt = nil
		item.SoldCardID = nil
		if item.VoidedAt == nil {
			item.VoidedAt = &ts
		}
	}
}

func VoidFiles(db *gorm.DB, space *models.Space, ids []uint) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	var files []models.ManagedFile
	if err := db.Where("id IN ? AND space_id = ? AND status IN ?", ids, space.ID, []string{"available", "locked"}).Find(&files).Error; err != nil {
		return 0, err
	}
	ts := NowUTC()
	for i := range files {
		files[i].Status = "voided"
		files[i].VoidedAt = &ts
		if err := db.Save(&files[i]).Error; err != nil {
			return 0, err
		}
		id := files[i].ID
		AddAudit(db, &space.ID, "void_file", "file", &id, files[i].OriginalName)
	}
	return len(files), nil
}

func InventoryCount(db *gorm.DB, spaceID *uint) (int64, error) {
	q := db.Model(&models.ManagedFile{}).Where("status = ?", "available")
	if spaceID != nil {
		q = q.Where("space_id = ?", *spaceID)
	}
	var count int64
	err := q.Count(&count).Error
	return count, err
}

type SpaceInventory struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	CardPrefix string `json:"card_prefix"`
	Inventory  int64  `json:"inventory"`
}

func InventoryBySpace(db *gorm.DB) ([]SpaceInventory, int64, error) {
	var spaces []models.Space
	if err := db.Order("id asc").Find(&spaces).Error; err != nil {
		return nil, 0, err
	}

	type countRow struct {
		SpaceID uint
		Count   int64
	}
	var rows []countRow
	if err := db.Model(&models.ManagedFile{}).
		Select("space_id, COUNT(*) as count").
		Where("status = ?", "available").
		Group("space_id").
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	countMap := make(map[uint]int64, len(rows))
	var total int64
	for _, row := range rows {
		countMap[row.SpaceID] = row.Count
		total += row.Count
	}

	result := make([]SpaceInventory, 0, len(spaces))
	for _, space := range spaces {
		result = append(result, SpaceInventory{
			ID:         space.ID,
			Name:       space.Name,
			CardPrefix: space.CardPrefix,
			Inventory:  countMap[space.ID],
		})
	}
	return result, total, nil
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

func writeFile(path string, raw []byte) error {
	if err := ensureDir(filepath.Dir(path)); err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func utf8Valid(b []byte) bool {
	return json.Valid(append(append([]byte{}, b...), []byte(`null`)...)) || json.Valid(b) || isUTF8(b)
}

func isUTF8(b []byte) bool {
	return strings.ToValidUTF8(string(b), "") == string(b)
}

func ZipFiles(downloadDir string, files []models.ManagedFile) (string, error) {
	if err := ensureDir(downloadDir); err != nil {
		return "", err
	}
	name := fmt.Sprintf("files_batch_%s.zip", NowUTC().Format("20060102150405"))
	path := filepath.Join(downloadDir, name)
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	used := map[string]struct{}{}
	for _, item := range files {
		arc := JSONDownloadName(item.OriginalName)
		if _, ok := used[arc]; ok {
			arc = fmt.Sprintf("%d_%s", item.ID, arc)
		}
		used[arc] = struct{}{}
		w, err := zw.Create(arc)
		if err != nil {
			_ = zw.Close()
			return "", err
		}
		raw, err := os.ReadFile(item.StoredPath)
		if err != nil {
			_ = zw.Close()
			return "", err
		}
		if _, err := w.Write(raw); err != nil {
			_ = zw.Close()
			return "", err
		}
	}
	if err := zw.Close(); err != nil {
		return "", err
	}
	return path, nil
}
