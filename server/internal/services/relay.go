package services

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"faka/server/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	RelayTypeSub2API = "sub2api"
	RelayTypeCPA     = "cpa"
)

var relayNamePattern = regexp.MustCompile(`^.{1,80}$`)

func NormalizeRelayType(t string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(t))
	switch v {
	case RelayTypeSub2API, RelayTypeCPA:
		return v, nil
	default:
		return "", Err("类型只能是 sub2api 或 cpa")
	}
}

func NormalizeRelayAddress(addr string) (string, error) {
	v := strings.TrimRight(strings.TrimSpace(addr), "/")
	if v == "" {
		return "", Err("地址不能为空")
	}
	u, err := url.Parse(v)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", Err("地址格式无效，需包含协议和主机，如 https://example.com")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", Err("地址协议只支持 http 或 https")
	}
	return v, nil
}

func CreateRelay(db *gorm.DB, name, typ, address, password string) (*models.Relay, error) {
	name = strings.TrimSpace(name)
	if name == "" || !relayNamePattern.MatchString(name) {
		return nil, Err("名称不能为空且不超过 80 个字符")
	}
	typ, err := NormalizeRelayType(typ)
	if err != nil {
		return nil, err
	}
	address, err = NormalizeRelayAddress(address)
	if err != nil {
		return nil, err
	}
	password = strings.TrimSpace(password)
	if password == "" {
		return nil, Err("密码不能为空")
	}
	relay := &models.Relay{
		Name:     name,
		Type:     typ,
		Address:  address,
		Password: password,
	}
	if err := db.Create(relay).Error; err != nil {
		return nil, err
	}
	AddAudit(db, nil, "create_relay", "relay", &relay.ID, name+":"+typ)
	return relay, nil
}

func UpdateRelay(db *gorm.DB, relay *models.Relay, name, typ, address, password string) error {
	name = strings.TrimSpace(name)
	if name == "" || !relayNamePattern.MatchString(name) {
		return Err("名称不能为空且不超过 80 个字符")
	}
	typ, err := NormalizeRelayType(typ)
	if err != nil {
		return err
	}
	address, err = NormalizeRelayAddress(address)
	if err != nil {
		return err
	}
	password = strings.TrimSpace(password)
	if password == "" {
		return Err("密码不能为空")
	}
	relay.Name = name
	relay.Type = typ
	relay.Address = address
	relay.Password = password
	if err := db.Save(relay).Error; err != nil {
		return err
	}
	AddAudit(db, nil, "update_relay", "relay", &relay.ID, name+":"+typ)
	return nil
}

func DeleteRelay(db *gorm.DB, relay *models.Relay) error {
	id := relay.ID
	name := relay.Name
	if err := db.Delete(relay).Error; err != nil {
		return err
	}
	AddAudit(db, nil, "delete_relay", "relay", &id, name)
	return nil
}

type RelayStats struct {
	Available *int64 `json:"available,omitempty"`
	Total     *int64 `json:"total,omitempty"`
	RateLimit *int64 `json:"rate_limit,omitempty"`
	Error     *int64 `json:"error,omitempty"`
	Queue     *int64 `json:"queue,omitempty"`
	Abnormal  *int64 `json:"abnormal,omitempty"`
	Message   string `json:"message,omitempty"`
}

func FetchRelayStats(relay *models.Relay) (*RelayStats, error) {
	switch relay.Type {
	case RelayTypeSub2API:
		return fetchSub2APIStats(relay)
	case RelayTypeCPA:
		return fetchCPAStats(relay)
	default:
		return nil, Err("未知中转类型")
	}
}

func fetchSub2APIStats(relay *models.Relay) (*RelayStats, error) {
	client := httpClient()
	availBody, err := sub2APIGet(client, relay, "/api/v1/admin/ops/account-availability")
	if err != nil {
		return &RelayStats{Message: err.Error()}, nil
	}
	concBody, err := sub2APIGet(client, relay, "/api/v1/admin/ops/concurrency")
	if err != nil {
		return &RelayStats{Message: err.Error()}, nil
	}

	var availResp struct {
		Code int `json:"code"`
		Data struct {
			Enabled  bool `json:"enabled"`
			Platform map[string]struct {
				TotalAccounts  int64 `json:"total_accounts"`
				AvailableCount int64 `json:"available_count"`
				RateLimitCount int64 `json:"rate_limit_count"`
				ErrorCount     int64 `json:"error_count"`
			} `json:"platform"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(availBody, &availResp); err != nil {
		return &RelayStats{Message: "解析可用性数据失败"}, nil
	}
	if availResp.Code != 0 {
		msg := availResp.Message
		if msg == "" {
			msg = "获取可用性失败"
		}
		return &RelayStats{Message: msg}, nil
	}

	var concResp struct {
		Code int `json:"code"`
		Data struct {
			Enabled  bool `json:"enabled"`
			Platform map[string]struct {
				WaitingInQueue int64 `json:"waiting_in_queue"`
			} `json:"platform"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(concBody, &concResp); err != nil {
		return &RelayStats{Message: "解析并发数据失败"}, nil
	}

	var total, available, rateLimit, errCount, queue int64
	for _, p := range availResp.Data.Platform {
		total += p.TotalAccounts
		available += p.AvailableCount
		rateLimit += p.RateLimitCount
		errCount += p.ErrorCount
	}
	for _, p := range concResp.Data.Platform {
		queue += p.WaitingInQueue
	}
	return &RelayStats{
		Available: &available,
		Total:     &total,
		RateLimit: &rateLimit,
		Error:     &errCount,
		Queue:     &queue,
	}, nil
}

func fetchCPAStats(relay *models.Relay) (*RelayStats, error) {
	client := httpClient()
	body, status, err := cpaRequest(client, relay, http.MethodGet, "/v0/management/auth-files", nil, "")
	if err != nil {
		return &RelayStats{Message: err.Error()}, nil
	}
	if status < 200 || status >= 300 {
		return &RelayStats{Message: fmt.Sprintf("CPA 返回 HTTP %d: %s", status, truncate(string(body), 200))}, nil
	}
	var resp struct {
		Files []struct {
			Status      string `json:"status"`
			Disabled    bool   `json:"disabled"`
			Unavailable bool   `json:"unavailable"`
			Failed      any    `json:"failed"`
		} `json:"files"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return &RelayStats{Message: "解析认证文件列表失败"}, nil
	}
	if resp.Error != "" {
		return &RelayStats{Message: resp.Error}, nil
	}
	total := int64(len(resp.Files))
	var abnormal int64
	for _, f := range resp.Files {
		if f.Disabled || f.Unavailable {
			abnormal++
			continue
		}
		st := strings.ToLower(strings.TrimSpace(f.Status))
		if st != "" && st != "active" && st != "ok" && st != "enabled" {
			abnormal++
			continue
		}
		switch v := f.Failed.(type) {
		case float64:
			if v > 0 {
				abnormal++
			}
		case int:
			if v > 0 {
				abnormal++
			}
		}
	}
	return &RelayStats{Total: &total, Abnormal: &abnormal}, nil
}

type SupplyResult struct {
	Supplied int      `json:"supplied"`
	Failed   int      `json:"failed"`
	Message  string   `json:"message"`
	Errors   []string `json:"errors,omitempty"`
}

type SupplyMeta struct {
	Mode         string
	SpaceID      *uint
	SpaceName    string
	GroupID      int64
	GroupName    string
	Concurrency  int
	CardCount    int
	RequestCount int
}

func SaveRelaySupplyRecord(db *gorm.DB, relay *models.Relay, result *SupplyResult, meta SupplyMeta) {
	if db == nil || relay == nil || result == nil {
		return
	}
	status := "success"
	if result.Supplied > 0 && result.Failed > 0 {
		status = "partial"
	} else if result.Supplied == 0 && result.Failed > 0 {
		status = "failed"
	}
	errText := ""
	if len(result.Errors) > 0 {
		// keep record compact
		parts := result.Errors
		if len(parts) > 10 {
			parts = parts[:10]
		}
		errText = strings.Join(parts, "；")
		if len(result.Errors) > 10 {
			errText += fmt.Sprintf("；...共 %d 条错误", len(result.Errors))
		}
	}
	var groupID *int64
	if meta.GroupID > 0 {
		gid := meta.GroupID
		groupID = &gid
	}
	rec := &models.RelaySupplyRecord{
		RelayID:      relay.ID,
		RelayName:    relay.Name,
		RelayType:    relay.Type,
		Mode:         meta.Mode,
		SpaceID:      meta.SpaceID,
		SpaceName:    meta.SpaceName,
		Supplied:     result.Supplied,
		Failed:       result.Failed,
		GroupID:      groupID,
		GroupName:    meta.GroupName,
		Concurrency:  meta.Concurrency,
		CardCount:    meta.CardCount,
		RequestCount: meta.RequestCount,
		Message:      truncate(result.Message, 500),
		Errors:       errText,
		Status:       status,
		CreatedAt:    NowUTC(),
	}
	_ = db.Create(rec).Error
}

func ListRelaySupplyRecords(db *gorm.DB, page, pageSize int, spaceID uint, relayID uint) ([]models.RelaySupplyRecord, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	q := db.Model(&models.RelaySupplyRecord{})
	if spaceID > 0 {
		q = q.Where("space_id = ?", spaceID)
	}
	if relayID > 0 {
		q = q.Where("relay_id = ?", relayID)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []models.RelaySupplyRecord
	if err := q.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

type Sub2APIGroup struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Status   string `json:"status"`
}

func ListSub2APIGroups(relay *models.Relay) ([]Sub2APIGroup, error) {
	if relay.Type != RelayTypeSub2API {
		return nil, Err("仅 sub2api 中转支持分组")
	}
	client := httpClient()
	// Prefer openai groups first for this product; fall back to all active groups.
	body, err := sub2APIGet(client, relay, "/api/v1/admin/groups/all?platform=openai")
	if err != nil {
		body, err = sub2APIGet(client, relay, "/api/v1/admin/groups/all")
		if err != nil {
			return nil, err
		}
	}
	var resp struct {
		Code    int            `json:"code"`
		Message string         `json:"message"`
		Data    []Sub2APIGroup `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, Err("解析分组列表失败")
	}
	if resp.Code != 0 {
		msg := resp.Message
		if msg == "" {
			msg = "获取分组失败"
		}
		return nil, Err(msg)
	}
	if resp.Data == nil {
		return []Sub2APIGroup{}, nil
	}
	return resp.Data, nil
}

func NormalizeAccountConcurrency(v int) (int, error) {
	if v < 0 {
		return 0, Err("账号并发不能为负数")
	}
	if v == 0 {
		return 10, nil
	}
	if v > 10000 {
		return 0, Err("账号并发不能超过 10000")
	}
	return v, nil
}

func SupplyRelayByCDKey(db *gorm.DB, downloadDir string, relay *models.Relay, cardCode string, groupID int64, concurrency int) (*SupplyResult, error) {
	codes, err := ParseCardCodes(cardCode)
	if err != nil {
		return nil, err
	}
	if len(codes) == 0 {
		return nil, Err("请输入卡密")
	}
	if len(codes) > 200 {
		return nil, Err("单次最多使用 200 个卡密")
	}
	// Keep original multi-code string for redeem helpers (they parse again).
	rawCodes := strings.Join(codes, "\n")

	switch relay.Type {
	case RelayTypeSub2API:
		if groupID <= 0 {
			return nil, Err("请选择要绑定的分组")
		}
		concurrency, err = NormalizeAccountConcurrency(concurrency)
		if err != nil {
			return nil, err
		}
		path, _, err := RedeemCardsSub2API(db, downloadDir, rawCodes)
		if err != nil {
			return nil, err
		}
		defer os.Remove(path)
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, Err("读取兑换结果失败")
		}
		n, failed, errs, err := pushSub2APIAccounts(relay, raw, groupID, concurrency)
		if err != nil {
			return nil, err
		}
		AddAudit(db, nil, "supply_relay_cdkey", "relay", &relay.ID, fmt.Sprintf("%s:cards=%d:group=%d:conc=%d:n=%d", relay.Name, len(codes), groupID, concurrency, n))
		msg := fmt.Sprintf("已用 %d 个卡密向中转补入 %d 个账号并绑定分组（并发 %d）", len(codes), n, concurrency)
		if failed > 0 {
			msg += fmt.Sprintf("，失败 %d 个", failed)
		}
		return &SupplyResult{Supplied: n, Failed: failed, Message: msg, Errors: errs}, nil

	case RelayTypeCPA:
		path, err := RedeemCardsCPA(db, downloadDir, rawCodes)
		if err != nil {
			return nil, err
		}
		defer os.Remove(path)
		files, err := extractJSONFromRedeemPath(path)
		if err != nil {
			return nil, err
		}
		uploaded, failed, errs := pushCPAAuthFiles(relay, files)
		AddAudit(db, nil, "supply_relay_cdkey", "relay", &relay.ID, fmt.Sprintf("%s:cards=%d:ok=%d,fail=%d", relay.Name, len(codes), uploaded, failed))
		msg := fmt.Sprintf("已用 %d 个卡密向中转补入 %d 个文件", len(codes), uploaded)
		if failed > 0 {
			msg += fmt.Sprintf("，失败 %d 个", failed)
		}
		return &SupplyResult{Supplied: uploaded, Failed: failed, Message: msg, Errors: errs}, nil
	default:
		return nil, Err("未知中转类型")
	}
}

func SupplyRelayByIdleFiles(db *gorm.DB, space *models.Space, relay *models.Relay, count int, groupID int64, concurrency int) (*SupplyResult, error) {
	if count <= 0 {
		return nil, Err("数量必须大于 0")
	}
	if count > 500 {
		return nil, Err("单次最多补 500 个文件")
	}

	var files []models.ManagedFile
	err := db.Transaction(func(tx *gorm.DB) error {
		var pool []models.ManagedFile
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("space_id = ? AND status = ?", space.ID, "available").
			Order("uploaded_at asc, id asc").
			Limit(count * 3).
			Find(&pool).Error; err != nil {
			return err
		}
		if len(pool) < count {
			return Err(fmt.Sprintf("空闲文件不足，当前可用 %d，需要 %d", len(pool), count))
		}
		picked := WeightedPick(pool, count)
		ids := make([]uint, 0, len(picked))
		for _, f := range picked {
			ids = append(ids, f.ID)
		}
		if err := claimAvailableFiles(tx, ids); err != nil {
			return err
		}
		if err := tx.Where("id IN ?", ids).Find(&files).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if se, ok := err.(*ServiceError); ok {
			return nil, se
		}
		if _, ok := err.(FileClaimConflict); ok {
			return nil, Err("文件抢占冲突，请重试")
		}
		return nil, err
	}

	successIDs := make([]uint, 0, len(files))
	failedIDs := make([]uint, 0)
	var pushErrs []string
	supplied := 0

	switch relay.Type {
	case RelayTypeSub2API:
		if groupID <= 0 {
			_ = releaseClaimedFiles(db, fileIDs(files))
			return nil, Err("请选择要绑定的分组")
		}
		concurrency, err = NormalizeAccountConcurrency(concurrency)
		if err != nil {
			_ = releaseClaimedFiles(db, fileIDs(files))
			return nil, err
		}
		cfg, err := BuildSub2APIConfig(files)
		if err != nil {
			_ = releaseClaimedFiles(db, fileIDs(files))
			return nil, err
		}
		raw, err := json.Marshal(cfg)
		if err != nil {
			_ = releaseClaimedFiles(db, fileIDs(files))
			return nil, err
		}
		n, failCount, errs, err := pushSub2APIAccounts(relay, raw, groupID, concurrency)
		if err != nil {
			_ = releaseClaimedFiles(db, fileIDs(files))
			return nil, err
		}
		pushErrs = append(pushErrs, errs...)
		// Files were converted as a batch export; mark inventory consumed when any account was created.
		if n > 0 {
			successIDs = fileIDs(files)
		} else {
			failedIDs = fileIDs(files)
		}
		supplied = n
		_ = failCount

	case RelayTypeCPA:
		payloads := make([]namedJSON, 0, len(files))
		for _, f := range files {
			raw, err := os.ReadFile(f.StoredPath)
			if err != nil {
				failedIDs = append(failedIDs, f.ID)
				pushErrs = append(pushErrs, f.OriginalName+": 读取失败")
				continue
			}
			payloads = append(payloads, namedJSON{Name: JSONDownloadName(f.OriginalName), Data: raw, FileID: f.ID})
		}
		if len(payloads) == 0 {
			_ = releaseClaimedFiles(db, failedIDs)
			return nil, Err("没有可读的空闲文件")
		}
		uploaded, _, errs := pushCPAAuthFiles(relay, payloads)
		pushErrs = append(pushErrs, errs...)
		failedNames := map[string]struct{}{}
		for _, e := range errs {
			for _, p := range payloads {
				if strings.Contains(e, p.Name) {
					failedNames[p.Name] = struct{}{}
				}
			}
		}
		if uploaded == 0 && len(errs) > 0 {
			for _, p := range payloads {
				failedIDs = append(failedIDs, p.FileID)
			}
		} else {
			for _, p := range payloads {
				if _, bad := failedNames[p.Name]; bad {
					failedIDs = append(failedIDs, p.FileID)
				} else {
					successIDs = append(successIDs, p.FileID)
				}
			}
		}
		supplied = len(successIDs)
		if supplied == 0 && uploaded > 0 {
			supplied = uploaded
		}
	}

	ts := NowUTC()
	if len(successIDs) > 0 {
		_ = updateFilesByIDs(db, successIDs, map[string]any{
			"status":             "relayed",
			"sold_at":            &ts,
			"latest_download_at": &ts,
		})
	}
	if len(failedIDs) > 0 {
		_ = releaseClaimedFiles(db, failedIDs)
	}

	detail := fmt.Sprintf("%s:count=%d,ok=%d,fail=%d", relay.Name, count, len(successIDs), len(failedIDs))
	if relay.Type == RelayTypeSub2API && groupID > 0 {
		detail = fmt.Sprintf("%s:group=%d:conc=%d:count=%d,ok=%d,fail=%d", relay.Name, groupID, concurrency, count, len(successIDs), len(failedIDs))
	}
	AddAudit(db, &space.ID, "supply_relay_idle", "relay", &relay.ID, detail)
	msg := fmt.Sprintf("已向中转补入 %d 个", supplied)
	if relay.Type == RelayTypeSub2API {
		msg = fmt.Sprintf("已向中转补入 %d 个账号并绑定分组（并发 %d）", supplied, concurrency)
	} else {
		msg = fmt.Sprintf("已向中转补入 %d 个文件", len(successIDs))
	}
	if len(failedIDs) > 0 || len(pushErrs) > 0 {
		msg += fmt.Sprintf("，失败 %d 个", maxInt(len(failedIDs), suppliedFailCount(pushErrs)))
	}
	return &SupplyResult{
		Supplied: supplied,
		Failed:   len(failedIDs),
		Message:  msg,
		Errors:   pushErrs,
	}, nil
}

type namedJSON struct {
	Name   string
	Data   []byte
	FileID uint
}

func fileIDs(files []models.ManagedFile) []uint {
	ids := make([]uint, 0, len(files))
	for _, f := range files {
		ids = append(ids, f.ID)
	}
	return ids
}

func releaseClaimedFiles(db *gorm.DB, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	return updateFilesByIDs(db, ids, map[string]any{
		"status":             "available",
		"sold_at":            nil,
		"latest_download_at": nil,
		"sold_card_id":       nil,
	})
}

func pushSub2APIAccounts(relay *models.Relay, raw []byte, groupID int64, concurrency int) (created, failed int, errs []string, err error) {
	var export struct {
		Accounts []map[string]any `json:"accounts"`
	}
	if err := json.Unmarshal(raw, &export); err != nil {
		return 0, 0, nil, Err("兑换结果 JSON 无效")
	}
	if len(export.Accounts) == 0 {
		return 0, 0, nil, Err("兑换结果中没有 accounts")
	}
	if groupID <= 0 {
		return 0, 0, nil, Err("请选择要绑定的分组")
	}
	concurrency, err = NormalizeAccountConcurrency(concurrency)
	if err != nil {
		return 0, 0, nil, err
	}

	// Use batch create so each account can bind group_ids at creation time.
	// data import always forces GroupIDs=nil and cannot select a target group.
	accounts := make([]map[string]any, 0, len(export.Accounts))
	for _, acc := range export.Accounts {
		item := map[string]any{
			"name":                       pickString(acc["name"]),
			"platform":                   pickString(acc["platform"], "openai"),
			"type":                       pickString(acc["type"], "oauth"),
			"credentials":                acc["credentials"],
			"extra":                      acc["extra"],
			"concurrency":                concurrency,
			"priority":                   acc["priority"],
			"rate_multiplier":            acc["rate_multiplier"],
			"auto_pause_on_expired":      acc["auto_pause_on_expired"],
			"group_ids":                  []int64{groupID},
			"confirm_mixed_channel_risk": true,
		}
		if item["name"] == "" {
			item["name"] = "imported"
		}
		if item["credentials"] == nil {
			failed++
			errs = append(errs, fmt.Sprintf("%v: 缺少 credentials", item["name"]))
			continue
		}
		accounts = append(accounts, item)
	}
	if len(accounts) == 0 {
		if len(errs) > 0 {
			return 0, failed, errs, Err(errs[0])
		}
		return 0, 0, nil, Err("没有可导入的账号")
	}

	payload := map[string]any{"accounts": accounts}
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, 0, nil, err
	}
	client := httpClient()
	respBody, status, err := sub2APIRequest(client, relay, http.MethodPost, "/api/v1/admin/accounts/batch", body, "application/json")
	if err != nil {
		return 0, 0, nil, err
	}
	if status < 200 || status >= 300 {
		return 0, 0, nil, Err(fmt.Sprintf("sub2api 批量创建失败 HTTP %d: %s", status, truncate(string(respBody), 300)))
	}
	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Success int `json:"success"`
			Failed  int `json:"failed"`
			Results []struct {
				Name    string `json:"name"`
				Success bool   `json:"success"`
				Error   string `json:"error"`
			} `json:"results"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return 0, 0, nil, Err("解析 sub2api 批量创建响应失败")
	}
	if resp.Code != 0 {
		msg := resp.Message
		if msg == "" {
			msg = "sub2api 批量创建失败"
		}
		return 0, 0, nil, Err(msg)
	}
	created = resp.Data.Success
	failed += resp.Data.Failed
	for _, r := range resp.Data.Results {
		if !r.Success && r.Error != "" {
			name := r.Name
			if name == "" {
				name = "account"
			}
			errs = append(errs, name+": "+r.Error)
		}
	}
	if created == 0 {
		msg := "sub2api 全部创建失败"
		if len(errs) > 0 {
			msg = errs[0]
		}
		return 0, failed, errs, Err(msg)
	}
	return created, failed, errs, nil
}

func pushCPAAuthFiles(relay *models.Relay, files []namedJSON) (uploaded, failed int, errs []string) {
	if len(files) == 0 {
		return 0, 0, nil
	}
	client := httpClient()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for _, f := range files {
		name := f.Name
		if !strings.HasSuffix(strings.ToLower(name), ".json") {
			name += ".json"
		}
		part, err := w.CreateFormFile("file", name)
		if err != nil {
			failed++
			errs = append(errs, name+": 构造上传失败")
			continue
		}
		if _, err := part.Write(f.Data); err != nil {
			failed++
			errs = append(errs, name+": 写入失败")
			continue
		}
	}
	contentType := w.FormDataContentType()
	if err := w.Close(); err != nil {
		return 0, len(files), []string{"构造 multipart 失败"}
	}
	body, status, err := cpaRequest(client, relay, http.MethodPost, "/v0/management/auth-files", buf.Bytes(), contentType)
	if err != nil {
		return 0, len(files), []string{err.Error()}
	}
	if status != 200 && status != 207 {
		return 0, len(files), []string{fmt.Sprintf("CPA 上传失败 HTTP %d: %s", status, truncate(string(body), 300))}
	}
	var resp struct {
		Status   string `json:"status"`
		Uploaded int    `json:"uploaded"`
		Failed   []struct {
			Name  string `json:"name"`
			Error string `json:"error"`
		} `json:"failed"`
		Error string `json:"error"`
	}
	_ = json.Unmarshal(body, &resp)
	if resp.Error != "" {
		return 0, len(files), []string{resp.Error}
	}
	if resp.Uploaded > 0 {
		uploaded = resp.Uploaded
	} else if status == 200 {
		uploaded = len(files)
	}
	for _, f := range resp.Failed {
		failed++
		errs = append(errs, f.Name+": "+f.Error)
	}
	if uploaded == 0 && failed == 0 {
		uploaded = len(files)
	}
	return uploaded, failed, errs
}

func extractJSONFromRedeemPath(path string) ([]namedJSON, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, Err("兑换文件不存在")
	}
	if info.IsDir() {
		return nil, Err("兑换路径无效")
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".json" {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		return []namedJSON{{Name: filepath.Base(path), Data: raw}}, nil
	}
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, Err("打开兑换 zip 失败")
	}
	defer r.Close()
	out := make([]namedJSON, 0)
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(f.Name), ".json") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		raw, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return nil, err
		}
		out = append(out, namedJSON{Name: filepath.Base(f.Name), Data: raw})
	}
	if len(out) == 0 {
		return nil, Err("兑换结果中没有 JSON 文件")
	}
	return out, nil
}

func httpClient() *http.Client {
	return &http.Client{Timeout: 120 * time.Second}
}

func sub2APIGet(client *http.Client, relay *models.Relay, path string) ([]byte, error) {
	body, status, err := sub2APIRequest(client, relay, http.MethodGet, path, nil, "")
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, Err(fmt.Sprintf("sub2api HTTP %d: %s", status, truncate(string(body), 200)))
	}
	return body, nil
}

func sub2APIRequest(client *http.Client, relay *models.Relay, method, path string, body []byte, contentType string) ([]byte, int, error) {
	u := relay.Address + path
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, u, rdr)
	if err != nil {
		return nil, 0, Err("构造请求失败")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-api-key", relay.Password)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, Err("请求中转失败: " + err.Error())
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, resp.StatusCode, Err("读取响应失败")
	}
	return raw, resp.StatusCode, nil
}

func cpaRequest(client *http.Client, relay *models.Relay, method, path string, body []byte, contentType string) ([]byte, int, error) {
	u := relay.Address + path
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, u, rdr)
	if err != nil {
		return nil, 0, Err("构造请求失败")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+relay.Password)
	req.Header.Set("X-Management-Key", relay.Password)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, Err("请求中转失败: " + err.Error())
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, resp.StatusCode, Err("读取响应失败")
	}
	return raw, resp.StatusCode, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func suppliedFailCount(errs []string) int {
	return len(errs)
}
