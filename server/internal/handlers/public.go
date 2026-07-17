package handlers

import (
	"net/http"
	"path/filepath"

	"faka/server/internal/config"
	"faka/server/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PublicHandler struct {
	DB  *gorm.DB
	Cfg config.Config
}

func (h *PublicHandler) Inventory(c *gin.Context) {
	spaces, total, err := services.InventoryBySpace(h.DB)
	if err != nil {
		serviceFail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"inventory": total,
		"spaces":    spaces,
	})
}

func (h *PublicHandler) Redeem(c *gin.Context) {
	cardCode := c.PostForm("card_code")
	if cardCode == "" {
		var body struct {
			CardCode     string `json:"card_code"`
			OutputFormat string `json:"output_format"`
		}
		_ = c.ShouldBindJSON(&body)
		cardCode = body.CardCode
		if c.PostForm("output_format") == "" && body.OutputFormat != "" {
			c.Request.PostForm = map[string][]string{"output_format": {body.OutputFormat}}
		}
	}
	format := c.DefaultPostForm("output_format", "cpa")
	if format == "sub" {
		path, name, err := services.RedeemCardsSub2API(h.DB, h.Cfg.DownloadDir, cardCode)
		if err != nil {
			serviceFail(c, err)
			return
		}
		c.FileAttachment(path, name)
		return
	}
	path, err := services.RedeemCardsCPA(h.DB, h.Cfg.DownloadDir, cardCode)
	if err != nil {
		serviceFail(c, err)
		return
	}
	c.FileAttachment(path, filepath.Base(path))
}
