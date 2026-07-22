package services

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"faka/server/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	fileClaimRetryLimit   = 5
	sub2apiDownloadPrefix = "sub2api"
)

var splitCodesPattern = regexp.MustCompile(`[\s,，;；]+`)

func ParseCardCodes(raw string) ([]string, error) {
	parts := splitCodesPattern.Split(raw, -1)
	codes := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, p := range parts {
		code := upper(trim(p))
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			return nil, Err("卡密不能重复输入")
		}
		if !cardPattern.MatchString(code) {
			return nil, Err("卡密格式错误：" + code)
		}
		seen[code] = struct{}{}
		codes = append(codes, code)
	}
	if len(codes) == 0 {
		return nil, Err("请输入卡密")
	}
	return codes, nil
}

func AssertCardCanRedeem(card *models.Card, code string, ts time.Time) error {
	if card == nil {
		return Err("卡密不存在：" + code)
	}
	switch card.Status {
	case "available":
		return Err("卡密需先修改为待使用后才能使用：" + code)
	case "voided":
		return Err("卡密已作废：" + code)
	case "pending", "sold":
	default:
		return Err("卡密状态不可使用：" + code)
	}
	if card.UsedAt != nil && ts.After(card.UsedAt.Add(24*time.Hour)) {
		return Err("卡密自首次使用后24小时内有效")
	}
	return nil
}

func WeightedPick(files []models.ManagedFile, count int) []models.ManagedFile {
	if count <= 0 || len(files) == 0 {
		return nil
	}
	if count >= len(files) {
		out := make([]models.ManagedFile, len(files))
		copy(out, files)
		return out
	}
	pool := make([]models.ManagedFile, len(files))
	copy(pool, files)
	// Prefer older uploads (already ordered by uploaded_at asc). Use fenwick-like
	// prefix sums so each draw is O(log n) instead of rebuilding weights.
	n := len(pool)
	// weight[i] = n-i  => older items get higher weight
	bit := make([]int, n+1)
	add := func(i, delta int) {
		for i <= n {
			bit[i] += delta
			i += i & -i
		}
	}
	sum := func(i int) int {
		s := 0
		for i > 0 {
			s += bit[i]
			i -= i & -i
		}
		return s
	}
	for i := 0; i < n; i++ {
		add(i+1, n-i)
	}
	alive := make([]bool, n)
	for i := range alive {
		alive[i] = true
	}
	picked := make([]models.ManagedFile, 0, count)
	for len(picked) < count {
		total := sum(n)
		if total <= 0 {
			break
		}
		r := rand.Intn(total) + 1
		// binary search first index with prefix >= r
		lo, hi := 1, n
		for lo < hi {
			mid := (lo + hi) / 2
			if sum(mid) >= r {
				hi = mid
			} else {
				lo = mid + 1
			}
		}
		idx := lo - 1
		if idx < 0 || idx >= n || !alive[idx] {
			// fallback linear scan for rare bit inconsistencies
			acc := 0
			idx = -1
			for i := 0; i < n; i++ {
				if !alive[i] {
					continue
				}
				acc += n - i
				if acc >= r {
					idx = i
					break
				}
			}
			if idx < 0 {
				break
			}
		}
		picked = append(picked, pool[idx])
		alive[idx] = false
		add(idx+1, -(n - idx))
	}
	return picked
}

func claimAvailableFiles(tx *gorm.DB, fileIDs []uint) error {
	if len(fileIDs) == 0 {
		return nil
	}
	var affected int64
	for _, chunk := range chunkIDs(fileIDs, sqlVariableChunk) {
		res := tx.Model(&models.ManagedFile{}).
			Where("id IN ? AND status = ?", chunk, "available").
			Update("status", "locked")
		if res.Error != nil {
			return res.Error
		}
		affected += res.RowsAffected
	}
	if affected != int64(len(fileIDs)) {
		return FileClaimConflict{}
	}
	return nil
}

func pickAndClaimFilesForCards(tx *gorm.DB, cards []models.Card) (map[uint][]models.ManagedFile, error) {
	// Precompute how many free files each space needs in this redeem request.
	needBySpace := map[uint]int{}
	for _, card := range cards {
		needBySpace[card.SpaceID] += card.FileCount
	}

	availableBySpace := map[uint][]models.ManagedFile{}
	picks := map[uint][]models.ManagedFile{}
	for _, card := range cards {
		var bound []models.ManagedFile
		if err := tx.Where("sold_card_id = ? AND status = ?", card.ID, "sold").
			Order("sold_at asc, id asc").Find(&bound).Error; err != nil {
			return nil, err
		}
		if len(bound) > 0 {
			picks[card.ID] = bound
			// Already bound cards do not consume free inventory for this pass.
			needBySpace[card.SpaceID] -= card.FileCount
			continue
		}
		if _, ok := availableBySpace[card.SpaceID]; !ok {
			// Load a bounded pool: required count * 3 (min 200) to keep weighted
			// preference for older files without reading the entire inventory.
			limit := needBySpace[card.SpaceID] * 3
			if limit < 200 {
				limit = 200
			}
			if limit < needBySpace[card.SpaceID] {
				limit = needBySpace[card.SpaceID]
			}
			var pool []models.ManagedFile
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("space_id = ? AND status = ?", card.SpaceID, "available").
				Order("uploaded_at asc, id asc").
				Limit(limit).
				Find(&pool).Error; err != nil {
				return nil, err
			}
			availableBySpace[card.SpaceID] = pool
		}
		pool := availableBySpace[card.SpaceID]
		if len(pool) < card.FileCount {
			return nil, Err("库存不足，无法兑换：" + card.Code)
		}
		picked := WeightedPick(pool, card.FileCount)
		ids := make([]uint, 0, len(picked))
		pickedIDs := map[uint]struct{}{}
		for _, p := range picked {
			ids = append(ids, p.ID)
			pickedIDs[p.ID] = struct{}{}
		}
		if err := claimAvailableFiles(tx, ids); err != nil {
			return nil, err
		}
		next := make([]models.ManagedFile, 0, len(pool)-len(picked))
		for _, item := range pool {
			if _, ok := pickedIDs[item.ID]; !ok {
				next = append(next, item)
			}
		}
		availableBySpace[card.SpaceID] = next
		picks[card.ID] = picked
	}
	return picks, nil
}

func pickFilesForCards(db *gorm.DB, cards []models.Card) (map[uint][]models.ManagedFile, error) {
	var last error
	for i := 0; i < fileClaimRetryLimit; i++ {
		var result map[uint][]models.ManagedFile
		err := db.Transaction(func(tx *gorm.DB) error {
			picks, err := pickAndClaimFilesForCards(tx, cards)
			if err != nil {
				return err
			}
			result = picks
			return nil
		})
		if err == nil {
			return result, nil
		}
		if _, ok := err.(FileClaimConflict); ok {
			last = err
			continue
		}
		return nil, err
	}
	if last != nil {
		return nil, Err("库存状态变化，请重试")
	}
	return nil, Err("库存状态变化，请重试")
}

func loadCardsByCodes(db *gorm.DB, codes []string) (map[string]models.Card, error) {
	var cards []models.Card
	if err := db.Where("code IN ?", codes).Find(&cards).Error; err != nil {
		return nil, err
	}
	m := map[string]models.Card{}
	for _, c := range cards {
		m[c.Code] = c
	}
	return m, nil
}

func RedeemCardsCPA(db *gorm.DB, downloadDir, rawCodes string) (string, error) {
	codes, err := ParseCardCodes(rawCodes)
	if err != nil {
		return "", err
	}
	ts := NowUTC()
	byCode, err := loadCardsByCodes(db, codes)
	if err != nil {
		return "", err
	}
	cards := make([]models.Card, 0, len(codes))
	for _, code := range codes {
		card, ok := byCode[code]
		var ptr *models.Card
		if ok {
			ptr = &card
		}
		if err := AssertCardCanRedeem(ptr, code, ts); err != nil {
			return "", err
		}
		cards = append(cards, card)
	}
	total := 0
	for _, c := range cards {
		total += c.FileCount
	}
	if total > maxRedeemFiles {
		return "", Err("单次最多支持打包 10000 个文件")
	}
	picks, err := pickFilesForCards(db, cards)
	if err != nil {
		return "", err
	}
	// reload picked files after claim transaction
	for cardID, items := range picks {
		ids := make([]uint, 0, len(items))
		for _, it := range items {
			ids = append(ids, it.ID)
		}
		var fresh []models.ManagedFile
		if err := db.Where("id IN ?", ids).Find(&fresh).Error; err != nil {
			return "", err
		}
		// preserve order by ids
		byID := map[uint]models.ManagedFile{}
		for _, f := range fresh {
			byID[f.ID] = f
		}
		ordered := make([]models.ManagedFile, 0, len(ids))
		for _, id := range ids {
			ordered = append(ordered, byID[id])
		}
		picks[cardID] = ordered
	}

	newly := make([]models.ManagedFile, 0)
	for _, items := range picks {
		for _, item := range items {
			if item.SoldCardID == nil {
				newly = append(newly, item)
			}
		}
	}
	archiveStem := cards[0].Code
	if len(cards) > 1 {
		archiveStem = fmt.Sprintf("BATCH_%d_CARDS", len(cards))
	}
	archivePath := filepath.Join(downloadDir, fmt.Sprintf("%s_%s.zip", archiveStem, ts.Format("20060102150405")))
	if err := ensureDir(downloadDir); err != nil {
		return "", err
	}
	if err := writeZipFromPicks(archivePath, cards, picks); err != nil {
		return "", err
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		if len(newly) > 0 {
			ids := make([]uint, 0, len(newly))
			for _, item := range newly {
				ids = append(ids, item.ID)
			}
			now := ts
			if err := updateFilesByIDs(tx, ids, map[string]any{
				"status":             "sold",
				"sold_at":            &now,
				"latest_download_at": &now,
			}); err != nil {
				return err
			}
		}
		for _, card := range cards {
			usedAt := card.UsedAt
			if usedAt == nil {
				usedAt = &ts
			}
			if err := tx.Model(&models.Card{}).Where("id = ?", card.ID).Updates(map[string]any{
				"status":  "sold",
				"used_at": usedAt,
			}).Error; err != nil {
				return err
			}
			fileIDs := make([]string, 0, len(picks[card.ID]))
			pickedIDs := make([]uint, 0, len(picks[card.ID]))
			for _, item := range picks[card.ID] {
				fileIDs = append(fileIDs, strconv.FormatUint(uint64(item.ID), 10))
				pickedIDs = append(pickedIDs, item.ID)
			}
			if err := updateFilesByIDs(tx, pickedIDs, map[string]any{
				"sold_card_id": card.ID,
			}); err != nil {
				return err
			}
			if err := tx.Create(&models.Redemption{
				CardID:       card.ID,
				RedeemedAt:   ts,
				OutputFormat: "cpa",
				DownloadPath: archivePath,
				FileIDs:      strings.Join(fileIDs, ","),
				Status:       "completed",
			}).Error; err != nil {
				return err
			}
			id := card.ID
			sid := card.SpaceID
			AddAudit(tx, &sid, "redeem_card", "card", &id, card.Code)
		}
		return nil
	})
	if err != nil {
		_ = os.Remove(archivePath)
		_ = releaseNewly(db, newly)
		return "", err
	}
	return archivePath, nil
}

func releaseNewly(db *gorm.DB, newly []models.ManagedFile) error {
	if len(newly) == 0 {
		return nil
	}
	ids := make([]uint, 0, len(newly))
	for _, item := range newly {
		ids = append(ids, item.ID)
	}
	return updateFilesByIDs(db, ids, map[string]any{
		"status":             "available",
		"sold_at":            nil,
		"latest_download_at": nil,
		"sold_card_id":       nil,
	})
}

func writeZipFromPicks(path string, cards []models.Card, picks map[uint][]models.ManagedFile) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	used := map[string]struct{}{}
	for _, card := range cards {
		for _, item := range picks[card.ID] {
			downloadName := JSONDownloadName(item.OriginalName)
			arcname := downloadName
			if _, ok := used[arcname]; ok {
				dir := filepath.Dir(arcname)
				base := filepath.Base(arcname)
				if dir == "." {
					arcname = fmt.Sprintf("%d_%s", item.ID, base)
				} else {
					arcname = fmt.Sprintf("%s/%d_%s", dir, item.ID, base)
				}
			}
			used[arcname] = struct{}{}
			w, err := zw.Create(arcname)
			if err != nil {
				_ = zw.Close()
				return err
			}
			raw, err := os.ReadFile(item.StoredPath)
			if err != nil {
				_ = zw.Close()
				return err
			}
			if _, err := w.Write(raw); err != nil {
				_ = zw.Close()
				return err
			}
		}
	}
	return zw.Close()
}

func RedeemCardsSub2API(db *gorm.DB, downloadDir, rawCodes string) (string, string, error) {
	codes, err := ParseCardCodes(rawCodes)
	if err != nil {
		return "", "", err
	}
	ts := NowUTC()
	byCode, err := loadCardsByCodes(db, codes)
	if err != nil {
		return "", "", err
	}
	cards := make([]models.Card, 0, len(codes))
	for _, code := range codes {
		card, ok := byCode[code]
		var ptr *models.Card
		if ok {
			ptr = &card
		}
		if err := AssertCardCanRedeem(ptr, code, ts); err != nil {
			return "", "", err
		}
		cards = append(cards, card)
	}
	total := 0
	for _, c := range cards {
		total += c.FileCount
	}
	if total > maxRedeemFiles {
		return "", "", Err("单次最多支持打包 10000 个文件")
	}
	picks, err := pickFilesForCards(db, cards)
	if err != nil {
		return "", "", err
	}
	var all []models.ManagedFile
	newly := make([]models.ManagedFile, 0)
	for cardID, items := range picks {
		ids := make([]uint, 0, len(items))
		for _, it := range items {
			ids = append(ids, it.ID)
		}
		var fresh []models.ManagedFile
		if err := db.Where("id IN ?", ids).Find(&fresh).Error; err != nil {
			return "", "", err
		}
		byID := map[uint]models.ManagedFile{}
		for _, f := range fresh {
			byID[f.ID] = f
		}
		ordered := make([]models.ManagedFile, 0, len(ids))
		for _, id := range ids {
			ordered = append(ordered, byID[id])
		}
		picks[cardID] = ordered
		for _, item := range ordered {
			all = append(all, item)
			if item.SoldCardID == nil {
				newly = append(newly, item)
			}
		}
	}
	cfg, err := BuildSub2APIConfig(all)
	if err != nil {
		_ = releaseNewly(db, newly)
		return "", "", err
	}
	if err := ensureDir(downloadDir); err != nil {
		return "", "", err
	}
	outPath := filepath.Join(downloadDir, fmt.Sprintf("%s-%s-%s.json", sub2apiDownloadPrefix, ts.Format("20060102150405"), uuid.NewString()))
	raw, _ := json.MarshalIndent(cfg, "", "  ")
	raw = append(raw, '\n')
	if err := writeFile(outPath, raw); err != nil {
		_ = releaseNewly(db, newly)
		return "", "", err
	}
	downloadName := fmt.Sprintf("%s-%s-plus.json", sub2apiDownloadPrefix, ts.Format("20060102150405"))
	err = db.Transaction(func(tx *gorm.DB) error {
		if len(newly) > 0 {
			ids := make([]uint, 0, len(newly))
			for _, item := range newly {
				ids = append(ids, item.ID)
			}
			now := ts
			if err := updateFilesByIDs(tx, ids, map[string]any{
				"status":             "sold",
				"sold_at":            &now,
				"latest_download_at": &now,
			}); err != nil {
				return err
			}
		}
		for _, card := range cards {
			usedAt := card.UsedAt
			if usedAt == nil {
				usedAt = &ts
			}
			if err := tx.Model(&models.Card{}).Where("id = ?", card.ID).Updates(map[string]any{
				"status":  "sold",
				"used_at": usedAt,
			}).Error; err != nil {
				return err
			}
			fileIDs := make([]string, 0, len(picks[card.ID]))
			pickedIDs := make([]uint, 0, len(picks[card.ID]))
			for _, item := range picks[card.ID] {
				fileIDs = append(fileIDs, strconv.FormatUint(uint64(item.ID), 10))
				pickedIDs = append(pickedIDs, item.ID)
			}
			if err := updateFilesByIDs(tx, pickedIDs, map[string]any{
				"sold_card_id": card.ID,
			}); err != nil {
				return err
			}
			if err := tx.Create(&models.Redemption{
				CardID:       card.ID,
				RedeemedAt:   ts,
				OutputFormat: "sub",
				DownloadPath: outPath,
				FileIDs:      strings.Join(fileIDs, ","),
				Status:       "completed",
			}).Error; err != nil {
				return err
			}
			id := card.ID
			sid := card.SpaceID
			AddAudit(tx, &sid, "redeem_card_sub2api", "card", &id, card.Code)
		}
		return nil
	})
	if err != nil {
		_ = os.Remove(outPath)
		_ = releaseNewly(db, newly)
		return "", "", err
	}
	return outPath, downloadName, nil
}

func BuildSub2APIConfig(files []models.ManagedFile) (map[string]any, error) {
	accounts := make([]map[string]any, 0)
	seen := map[string]struct{}{}
	for _, item := range files {
		raw, err := os.ReadFile(item.StoredPath)
		if err != nil {
			return nil, Err("JSON file cannot be read: " + item.OriginalName)
		}
		var data map[string]any
		if err := json.Unmarshal(raw, &data); err != nil {
			return nil, Err("JSON format error: " + item.OriginalName)
		}
		account, err := buildSub2APIAccountEntry(data, item.StoredPath)
		if err != nil {
			return nil, err
		}
		account["concurrency"] = 10
		account["priority"] = 1
		account["rate_multiplier"] = 1
		account["auto_pause_on_expired"] = true
		key := buildSub2APIDedupeKey(account)
		if key != "" {
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
		}
		accounts = append(accounts, account)
	}
	if len(accounts) == 0 {
		return nil, Err("没有成功转换任何账号")
	}
	return map[string]any{"accounts": accounts, "proxies": []any{}}, nil
}

func extractTokenValue(data map[string]any, name string) string {
	if tokens, ok := data["tokens"].(map[string]any); ok {
		if v, ok := tokens[name].(string); ok {
			return v
		}
	}
	if v, ok := data[name].(string); ok {
		return v
	}
	return ""
}

func decodeJWTPayload(token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil, Err("token 不是合法 JWT")
	}
	seg := parts[1]
	if m := len(seg) % 4; m != 0 {
		seg += strings.Repeat("=", 4-m)
	}
	seg = strings.ReplaceAll(strings.ReplaceAll(seg, "-", "+"), "_", "/")
	raw, err := base64.StdEncoding.DecodeString(seg)
	if err != nil {
		return nil, Err("无法解析 token payload")
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, Err("无法解析 token payload")
	}
	return payload, nil
}

func detectAuthFormat(data map[string]any) (string, error) {
	if data["auth_mode"] == "chatgpt" {
		if _, ok := data["tokens"].(map[string]any); ok {
			return "chatgpt", nil
		}
	}
	if data["type"] == "codex" {
		return "codex", nil
	}
	return "", Err("无法识别格式，需要 CPA/codex 或 ChatGPT auth JSON")
}

func extractOpenAIAuth(payload map[string]any) map[string]any {
	if v, ok := payload["https://api.openai.com/auth"].(map[string]any); ok {
		return v
	}
	return map[string]any{}
}

func pickString(values ...any) string {
	for _, v := range values {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func resolveTier(payload map[string]any) string {
	auth := extractOpenAIAuth(payload)
	candidate := strings.ToLower(pickString(payload["tier"], payload["plan"], auth["chatgpt_plan_type"], "unknown"))
	switch {
	case strings.Contains(candidate, "team"):
		return "team"
	case strings.Contains(candidate, "pro"):
		return "pro"
	case strings.Contains(candidate, "plus"):
		return "plus"
	default:
		if candidate == "" {
			return "unknown"
		}
		return candidate
	}
}

func buildSub2APIAccountEntry(data map[string]any, sourcePath string) (map[string]any, error) {
	accessToken := extractTokenValue(data, "access_token")
	if accessToken == "" {
		return nil, Err("缺少 access_token，无法导出 sub2api")
	}
	accessPayload, err := decodeJWTPayload(accessToken)
	if err != nil {
		return nil, err
	}
	accessAuth := extractOpenAIAuth(accessPayload)
	idToken := extractTokenValue(data, "id_token")
	idPayload := map[string]any{}
	if idToken != "" {
		idPayload, _ = decodeJWTPayload(idToken)
	}
	idAuth := extractOpenAIAuth(idPayload)
	orgID := ""
	if orgs, ok := idAuth["organizations"].([]any); ok && len(orgs) > 0 {
		if first, ok := orgs[0].(map[string]any); ok {
			orgID = pickString(first["id"])
		}
	}
	fmtName, err := detectAuthFormat(data)
	if err != nil {
		return nil, err
	}
	var accountID any
	if fmtName == "chatgpt" {
		if tokens, ok := data["tokens"].(map[string]any); ok {
			accountID = tokens["account_id"]
		}
	} else {
		accountID = data["account_id"]
	}
	accountIDStr := pickString(accountID, accessAuth["chatgpt_account_id"], idPayload["sub"])
	if accountIDStr == "" {
		return nil, Err("无法从 JSON 中解析 account_id")
	}
	email := strings.ToLower(pickString(data["email"], idPayload["email"]))
	if email == "" {
		return nil, Err("无法从 JSON 中解析 email")
	}
	tier := resolveTier(idPayload)
	if tier == "unknown" {
		tier = resolveTier(accessPayload)
	}
	exp, _ := accessPayload["exp"].(float64)
	iat, _ := accessPayload["iat"].(float64)
	expiresIn := 864000
	if exp > 0 && iat > 0 {
		expiresIn = int(exp - iat)
	}
	name := strings.Split(email, "@")[0]
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
	}
	lastRefresh := pickString(data["last_refresh"])
	clientID := pickString(accessPayload["client_id"])
	return map[string]any{
		"name":     name,
		"platform": "openai",
		"type":     "oauth",
		"credentials": map[string]any{
			"access_token":       accessToken,
			"chatgpt_account_id": pickString(accessAuth["chatgpt_account_id"], accountIDStr),
			"chatgpt_user_id":    pickString(accessAuth["chatgpt_user_id"]),
			"client_id":          clientID,
			"email":              email,
			"expires_at":         int64(exp),
			"expires_in":         expiresIn,
			"id_token":           idToken,
			"organization_id":    orgID,
			"plan_type":          tier,
			"refresh_token":      extractTokenValue(data, "refresh_token"),
		},
		"extra": map[string]any{
			"email":        email,
			"last_refresh": lastRefresh,
		},
	}, nil
}

func buildSub2APIDedupeKey(account map[string]any) string {
	credentials, _ := account["credentials"].(map[string]any)
	extra, _ := account["extra"].(map[string]any)
	if credentials == nil {
		credentials = map[string]any{}
	}
	if extra == nil {
		extra = map[string]any{}
	}
	userID := pickString(credentials["chatgpt_user_id"], extra["chatgpt_user_id"])
	accID := pickString(credentials["chatgpt_account_id"], extra["chatgpt_account_id"])
	if userID != "" && accID != "" {
		return "account-user:" + userID + "|" + accID
	}
	if rt := pickString(credentials["refresh_token"]); rt != "" {
		return "refresh:" + rt
	}
	if at := pickString(credentials["access_token"]); at != "" {
		sum := sha256.Sum256([]byte(at))
		return "access:" + fmt.Sprintf("%x", sum)
	}
	email := pickString(credentials["email"], extra["email"])
	if email != "" {
		return "account:" + email + "|" + accID + "|" + pickString(credentials["organization_id"]) + "|" + pickString(credentials["plan_type"])
	}
	return ""
}
