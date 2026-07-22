package services

import (
	"path/filepath"
	"regexp"
	"time"

	"faka/server/internal/models"

	"gorm.io/gorm"
)

var (
	spaceNamePattern   = regexp.MustCompile(`^[A-Za-z0-9_]{1,20}$`)
	spacePrefixPattern = regexp.MustCompile(`^[A-Z0-9]{1,10}$`)
)

func NormalizeSpaceName(name string) (string, error) {
	value := trim(name)
	if !spaceNamePattern.MatchString(value) {
		return "", Err("空间名只能是英文、数字、下划线，且不超过 20 个字符")
	}
	return value, nil
}

func NormalizeSpacePrefix(prefix string) (string, error) {
	value := upper(trim(prefix))
	if !spacePrefixPattern.MatchString(value) {
		return "", Err("卡密前缀只能是大写英文和数字，且不超过 10 个字符")
	}
	return value, nil
}

func CreateSpace(db *gorm.DB, name, cardPrefix string) (*models.Space, error) {
	name, err := NormalizeSpaceName(name)
	if err != nil {
		return nil, err
	}
	cardPrefix, err = NormalizeSpacePrefix(cardPrefix)
	if err != nil {
		return nil, err
	}
	var count int64
	if err := db.Model(&models.Space{}).Where("name = ?", name).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, Err("空间名已存在")
	}
	if err := db.Model(&models.Space{}).Where("card_prefix = ?", cardPrefix).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, Err("卡密前缀已存在")
	}
	space := &models.Space{Name: name, CardPrefix: cardPrefix}
	if err := db.Create(space).Error; err != nil {
		return nil, err
	}
	AddAudit(db, &space.ID, "create_space", "space", &space.ID, name+":"+cardPrefix)
	return space, nil
}

func UpdateSpace(db *gorm.DB, space *models.Space, name, cardPrefix string) error {
	name, err := NormalizeSpaceName(name)
	if err != nil {
		return err
	}
	cardPrefix, err = NormalizeSpacePrefix(cardPrefix)
	if err != nil {
		return err
	}
	var count int64
	if err := db.Model(&models.Space{}).Where("name = ? AND id <> ?", name, space.ID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return Err("空间名已存在")
	}
	if err := db.Model(&models.Space{}).Where("card_prefix = ? AND id <> ?", cardPrefix, space.ID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return Err("卡密前缀已存在")
	}
	space.Name = name
	space.CardPrefix = cardPrefix
	space.UpdatedAt = time.Now().UTC()
	if err := db.Save(space).Error; err != nil {
		return err
	}
	AddAudit(db, &space.ID, "update_space", "space", &space.ID, name+":"+cardPrefix)
	return nil
}

func DeleteSpaceData(db *gorm.DB, spaceID uint) (map[string]int, error) {
	var cards []models.Card
	if err := db.Where("space_id = ?", spaceID).Find(&cards).Error; err != nil {
		return nil, err
	}
	cardIDs := make([]uint, 0, len(cards))
	for _, c := range cards {
		cardIDs = append(cardIDs, c.ID)
	}
	var files []models.ManagedFile
	if err := db.Where("space_id = ?", spaceID).Find(&files).Error; err != nil {
		return nil, err
	}
	var redemptions []models.Redemption
	if len(cardIDs) > 0 {
		if err := db.Where("card_id IN ?", cardIDs).Find(&redemptions).Error; err != nil {
			return nil, err
		}
	}
	var audits []models.AuditLog
	if err := db.Where("space_id = ?", spaceID).Find(&audits).Error; err != nil {
		return nil, err
	}
	var uploadRecordCount int64
	if err := db.Model(&models.UploadRecord{}).Where("space_id = ?", spaceID).Count(&uploadRecordCount).Error; err != nil {
		return nil, err
	}
	for _, item := range files {
		_ = removeFile(item.StoredPath)
		if err := db.Delete(&item).Error; err != nil {
			return nil, err
		}
	}
	for _, r := range redemptions {
		_ = removeFile(r.DownloadPath)
		if err := db.Delete(&r).Error; err != nil {
			return nil, err
		}
	}
	for _, c := range cards {
		if err := db.Delete(&c).Error; err != nil {
			return nil, err
		}
	}
	for _, a := range audits {
		if err := db.Delete(&a).Error; err != nil {
			return nil, err
		}
	}
	if err := db.Where("space_id = ?", spaceID).Delete(&models.UploadRecord{}).Error; err != nil {
		return nil, err
	}
	return map[string]int{
		"cards":          len(cards),
		"files":          len(files),
		"redemptions":    len(redemptions),
		"audits":         len(audits),
		"upload_records": int(uploadRecordCount),
	}, nil
}

func DeleteSpace(db *gorm.DB, space *models.Space) (map[string]int, error) {
	counts, err := DeleteSpaceData(db, space.ID)
	if err != nil {
		return nil, err
	}
	if err := db.Delete(space).Error; err != nil {
		return nil, err
	}
	return counts, nil
}

func removeFile(path string) error {
	if path == "" {
		return nil
	}
	return removePath(filepath.Clean(path))
}
