package handlers

import (
	"net/http"
	"strconv"

	"faka/server/internal/config"
	"faka/server/internal/models"
	"faka/server/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type RelayHandler struct {
	DB  *gorm.DB
	Cfg config.Config
}

func (h *RelayHandler) List(c *gin.Context) {
	var relays []models.Relay
	if err := h.DB.Order("id asc").Find(&relays).Error; err != nil {
		serviceFail(c, err)
		return
	}
	items := make([]gin.H, 0, len(relays))
	for _, r := range relays {
		items = append(items, gin.H{
			"id":         r.ID,
			"name":       r.Name,
			"type":       r.Type,
			"address":    r.Address,
			"password":   r.Password,
			"created_at": formatTimeVal(r.CreatedAt),
			"updated_at": formatTimeVal(r.UpdatedAt),
		})
	}
	ok(c, gin.H{"relays": items})
}

func (h *RelayHandler) Create(c *gin.Context) {
	var body struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		Address  string `json:"address"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, "参数错误")
		return
	}
	relay, err := services.CreateRelay(h.DB, body.Name, body.Type, body.Address, body.Password)
	if err != nil {
		serviceFail(c, err)
		return
	}
	ok(c, gin.H{
		"relay": gin.H{
			"id":       relay.ID,
			"name":     relay.Name,
			"type":     relay.Type,
			"address":  relay.Address,
			"password": relay.Password,
		},
		"message": "中转已创建",
	})
}

func (h *RelayHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var relay models.Relay
	if err := h.DB.First(&relay, id).Error; err != nil {
		fail(c, http.StatusNotFound, "中转不存在")
		return
	}
	var body struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		Address  string `json:"address"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, "参数错误")
		return
	}
	if err := services.UpdateRelay(h.DB, &relay, body.Name, body.Type, body.Address, body.Password); err != nil {
		serviceFail(c, err)
		return
	}
	ok(c, gin.H{
		"relay": gin.H{
			"id":       relay.ID,
			"name":     relay.Name,
			"type":     relay.Type,
			"address":  relay.Address,
			"password": relay.Password,
		},
		"message": "已更新中转 " + relay.Name,
	})
}

func (h *RelayHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var relay models.Relay
	if err := h.DB.First(&relay, id).Error; err != nil {
		fail(c, http.StatusNotFound, "中转不存在")
		return
	}
	name := relay.Name
	if err := services.DeleteRelay(h.DB, &relay); err != nil {
		serviceFail(c, err)
		return
	}
	ok(c, gin.H{"message": "已删除中转 " + name})
}

func (h *RelayHandler) Stats(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var relay models.Relay
	if err := h.DB.First(&relay, id).Error; err != nil {
		fail(c, http.StatusNotFound, "中转不存在")
		return
	}
	stats, err := services.FetchRelayStats(&relay)
	if err != nil {
		serviceFail(c, err)
		return
	}
	ok(c, gin.H{"stats": stats, "type": relay.Type})
}

func (h *RelayHandler) Groups(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var relay models.Relay
	if err := h.DB.First(&relay, id).Error; err != nil {
		fail(c, http.StatusNotFound, "中转不存在")
		return
	}
	if relay.Type != services.RelayTypeSub2API {
		fail(c, http.StatusBadRequest, "仅 sub2api 中转支持分组")
		return
	}
	groups, err := services.ListSub2APIGroups(&relay)
	if err != nil {
		serviceFail(c, err)
		return
	}
	ok(c, gin.H{"groups": groups})
}

func (h *RelayHandler) resolveGroupName(relay *models.Relay, groupID int64) string {
	if groupID <= 0 || relay == nil || relay.Type != services.RelayTypeSub2API {
		return ""
	}
	groups, err := services.ListSub2APIGroups(relay)
	if err != nil {
		return ""
	}
	for _, g := range groups {
		if g.ID == groupID {
			return g.Name
		}
	}
	return ""
}

func (h *RelayHandler) SupplyRecords(c *gin.Context) {
	page, size := parsePage(c, 50, []int{20, 50, 100, 200})
	var relayID uint
	if raw := c.Query("relay_id"); raw != "" {
		if n, err := strconv.ParseUint(raw, 10, 64); err == nil {
			relayID = uint(n)
		}
	}
	rows, total, err := services.ListRelaySupplyRecords(h.DB, page, size, relayID)
	if err != nil {
		serviceFail(c, err)
		return
	}
	items := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		items = append(items, gin.H{
			"id":            r.ID,
			"relay_id":      r.RelayID,
			"relay_name":    r.RelayName,
			"relay_type":    r.RelayType,
			"mode":          r.Mode,
			"space_id":      r.SpaceID,
			"space_name":    r.SpaceName,
			"supplied":      r.Supplied,
			"failed":        r.Failed,
			"group_id":      r.GroupID,
			"group_name":    r.GroupName,
			"concurrency":   r.Concurrency,
			"card_count":    r.CardCount,
			"request_count": r.RequestCount,
			"message":       r.Message,
			"errors":        r.Errors,
			"status":        r.Status,
			"created_at":    formatTimeVal(r.CreatedAt),
		})
	}
	ok(c, gin.H{"records": items, "pagination": pagination(total, page, size)})
}

func (h *RelayHandler) Supply(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var relay models.Relay
	if err := h.DB.First(&relay, id).Error; err != nil {
		fail(c, http.StatusNotFound, "中转不存在")
		return
	}
	var body struct {
		Mode        string `json:"mode"` // cdkey | idle
		CardCode    string `json:"card_code"`
		Count       int    `json:"count"`
		GroupID     int64  `json:"group_id"`
		Concurrency int    `json:"concurrency"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, "参数错误")
		return
	}
	mode := body.Mode
	if mode == "" {
		if body.CardCode != "" {
			mode = "cdkey"
		} else {
			mode = "idle"
		}
	}
	groupName := h.resolveGroupName(&relay, body.GroupID)
	switch mode {
	case "cdkey":
		result, err := services.SupplyRelayByCDKey(h.DB, h.Cfg.DownloadDir, &relay, body.CardCode, body.GroupID, body.Concurrency)
		if err != nil {
			serviceFail(c, err)
			return
		}
		cardCount := 0
		if codes, parseErr := services.ParseCardCodes(body.CardCode); parseErr == nil {
			cardCount = len(codes)
		}
		meta := services.SupplyMeta{
			Mode:         "cdkey",
			GroupID:      body.GroupID,
			GroupName:    groupName,
			Concurrency:  body.Concurrency,
			CardCount:    cardCount,
			RequestCount: cardCount,
		}
		if relay.Type == services.RelayTypeSub2API {
			if conc, nerr := services.NormalizeAccountConcurrency(body.Concurrency); nerr == nil {
				meta.Concurrency = conc
			}
		} else {
			meta.Concurrency = 0
			meta.GroupID = 0
			meta.GroupName = ""
		}
		services.SaveRelaySupplyRecord(h.DB, &relay, result, meta)
		ok(c, gin.H{
			"message":  result.Message,
			"supplied": result.Supplied,
			"failed":   result.Failed,
			"errors":   result.Errors,
		})
	case "idle":
		space, err := resolveSpace(c, h.DB)
		if err != nil {
			serviceFail(c, err)
			return
		}
		result, err := services.SupplyRelayByIdleFiles(h.DB, space, &relay, body.Count, body.GroupID, body.Concurrency)
		if err != nil {
			serviceFail(c, err)
			return
		}
		sid := space.ID
		meta := services.SupplyMeta{
			Mode:         "idle",
			SpaceID:      &sid,
			SpaceName:    space.Name,
			GroupID:      body.GroupID,
			GroupName:    groupName,
			Concurrency:  body.Concurrency,
			RequestCount: body.Count,
		}
		if relay.Type == services.RelayTypeSub2API {
			if conc, nerr := services.NormalizeAccountConcurrency(body.Concurrency); nerr == nil {
				meta.Concurrency = conc
			}
		} else {
			meta.Concurrency = 0
			meta.GroupID = 0
			meta.GroupName = ""
		}
		services.SaveRelaySupplyRecord(h.DB, &relay, result, meta)
		ok(c, gin.H{
			"message":  result.Message,
			"supplied": result.Supplied,
			"failed":   result.Failed,
			"errors":   result.Errors,
		})
	default:
		fail(c, http.StatusBadRequest, "补号方式无效，请使用 cdkey 或 idle")
	}
}
