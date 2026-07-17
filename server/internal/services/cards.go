package services

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"faka/server/internal/models"

	"gorm.io/gorm"
)

const (
	cardAlphabet       = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	cardRandomLength   = 32
	maxCardBatchQty    = 10000
	minCardFileCount   = 1
	maxRedeemFiles     = 10000
	// SQLite default SQLITE_MAX_VARIABLE_NUMBER is 999.
	sqlVariableChunk = 500
	cardInsertChunk  = 500
)

var cardPattern = regexp.MustCompile(`^[A-Z0-9]+-[A-Z0-9]{32}$`)

func existingCardCodes(db *gorm.DB, candidates []string) (map[string]struct{}, error) {
	existSet := make(map[string]struct{})
	if len(candidates) == 0 {
		return existSet, nil
	}
	for start := 0; start < len(candidates); start += sqlVariableChunk {
		end := start + sqlVariableChunk
		if end > len(candidates) {
			end = len(candidates)
		}
		chunk := candidates[start:end]
		var existing []string
		if err := db.Model(&models.Card{}).Where("code IN ?", chunk).Pluck("code", &existing).Error; err != nil {
			return nil, err
		}
		for _, c := range existing {
			existSet[c] = struct{}{}
		}
	}
	return existSet, nil
}

func GenerateCardCodes(db *gorm.DB, quantity int, prefix string) ([]string, error) {
	token := upper(trim(prefix))
	if token == "" {
		token = "DEFAULT"
	}
	if !strings.HasSuffix(token, "-") {
		token += "-"
	}
	codes := make([]string, 0, quantity)
	seen := map[string]struct{}{}
	for len(codes) < quantity {
		need := quantity - len(codes)
		// Generate in smaller chunks so uniqueness checks stay under SQLite variable limits.
		if need > sqlVariableChunk {
			need = sqlVariableChunk
		}
		candidates := make([]string, 0, need)
		for len(candidates) < need {
			suffix, err := randomSuffix(cardRandomLength)
			if err != nil {
				return nil, err
			}
			code := token + suffix
			if _, ok := seen[code]; ok {
				continue
			}
			seen[code] = struct{}{}
			candidates = append(candidates, code)
		}
		existSet, err := existingCardCodes(db, candidates)
		if err != nil {
			return nil, err
		}
		for _, c := range candidates {
			if _, ok := existSet[c]; !ok {
				codes = append(codes, c)
			}
		}
	}
	return codes, nil
}

func randomSuffix(n int) (string, error) {
	var b strings.Builder
	max := big.NewInt(int64(len(cardAlphabet)))
	for i := 0; i < n; i++ {
		v, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b.WriteByte(cardAlphabet[v.Int64()])
	}
	return b.String(), nil
}

func CreateCards(db *gorm.DB, space *models.Space, fileCount, quantity int) ([]models.Card, error) {
	if fileCount < minCardFileCount || fileCount > maxRedeemFiles {
		return nil, Err(fmt.Sprintf("卡密绑定文件数量必须在 %d 到 %d 之间", minCardFileCount, maxRedeemFiles))
	}
	if quantity < 1 || quantity > maxCardBatchQty {
		return nil, Err(fmt.Sprintf("单次生成数量必须在 1 到 %d 之间", maxCardBatchQty))
	}
	codes, err := GenerateCardCodes(db, quantity, space.CardPrefix)
	if err != nil {
		return nil, err
	}
	now := NowUTC()
	cards := make([]models.Card, 0, len(codes))
	for _, code := range codes {
		cards = append(cards, models.Card{
			Code:      code,
			SpaceID:   space.ID,
			FileCount: fileCount,
			Status:    "available",
			CreatedAt: now,
		})
	}
	// Insert in batches to avoid SQLite multi-row INSERT variable limits.
	if err := db.CreateInBatches(&cards, cardInsertChunk).Error; err != nil {
		return nil, err
	}
	// Keep audit light for large batches: one summary log instead of N rows.
	if len(cards) == 1 {
		id := cards[0].ID
		AddAudit(db, &space.ID, "create_card", "card", &id, fmt.Sprintf("%s:%d", cards[0].Code, fileCount))
	} else if len(cards) > 1 {
		AddAudit(db, &space.ID, "create_card_batch", "card", nil, fmt.Sprintf("count=%d file_count=%d prefix=%s", len(cards), fileCount, space.CardPrefix))
	}
	return cards, nil
}

func ApplyCardStatus(card *models.Card, target string, ts time.Time) {
	card.Status = target
	switch target {
	case "available", "pending":
		card.UsedAt = nil
		card.VoidedAt = nil
	case "sold":
		if card.UsedAt == nil {
			card.UsedAt = &ts
		}
		card.VoidedAt = nil
	case "voided":
		card.UsedAt = nil
		if card.VoidedAt == nil {
			card.VoidedAt = &ts
		}
	}
}

func VoidCards(db *gorm.DB, space *models.Space, ids []uint) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	var cards []models.Card
	if err := db.Where("id IN ? AND space_id = ? AND status = ?", ids, space.ID, "available").Find(&cards).Error; err != nil {
		return 0, err
	}
	ts := NowUTC()
	for i := range cards {
		cards[i].Status = "voided"
		cards[i].VoidedAt = &ts
		if err := db.Save(&cards[i]).Error; err != nil {
			return 0, err
		}
		id := cards[i].ID
		AddAudit(db, &space.ID, "void_card", "card", &id, cards[i].Code)
	}
	return len(cards), nil
}

func WriteCardCodesFile(downloadDir string, codes []string) (string, error) {
	if err := ensureDir(downloadDir); err != nil {
		return "", err
	}
	name := fmt.Sprintf("cards_batch_%s.txt", NowUTC().Format("20060102150405"))
	path := filepath.Join(downloadDir, name)
	content := strings.Join(codes, "\n")
	if err := writeFile(path, []byte(content)); err != nil {
		return "", err
	}
	return path, nil
}
