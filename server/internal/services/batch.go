package services

import (
	"faka/server/internal/models"

	"gorm.io/gorm"
)

// chunkIDs splits IDs into pieces that stay under SQLite variable limits.
func chunkIDs(ids []uint, size int) [][]uint {
	if size <= 0 {
		size = sqlVariableChunk
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

func updateFilesByIDs(db *gorm.DB, ids []uint, values map[string]any) error {
	for _, chunk := range chunkIDs(ids, sqlVariableChunk) {
		if err := db.Model(&models.ManagedFile{}).Where("id IN ?", chunk).Updates(values).Error; err != nil {
			return err
		}
	}
	return nil
}

func updateCardsByIDs(db *gorm.DB, ids []uint, values map[string]any) error {
	for _, chunk := range chunkIDs(ids, sqlVariableChunk) {
		if err := db.Model(&models.Card{}).Where("id IN ?", chunk).Updates(values).Error; err != nil {
			return err
		}
	}
	return nil
}
