package handlers

import (
	"net/http"
	"strconv"

	"faka/server/internal/models"
	"faka/server/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SpaceHandler struct {
	DB *gorm.DB
}

func (h *SpaceHandler) List(c *gin.Context) {
	var spaces []models.Space
	if err := h.DB.Order("id asc").Find(&spaces).Error; err != nil {
		serviceFail(c, err)
		return
	}
	current, err := resolveSpace(c, h.DB)
	if err != nil {
		serviceFail(c, err)
		return
	}
	ok(c, gin.H{"spaces": spaces, "current_space": current})
}

func (h *SpaceHandler) Create(c *gin.Context) {
	var body struct {
		Name       string `json:"name"`
		CardPrefix string `json:"card_prefix"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, "参数错误")
		return
	}
	space, err := services.CreateSpace(h.DB, body.Name, body.CardPrefix)
	if err != nil {
		serviceFail(c, err)
		return
	}
	ok(c, gin.H{"space": space, "message": "空间已创建"})
}

func (h *SpaceHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var space models.Space
	if err := h.DB.First(&space, id).Error; err != nil {
		fail(c, http.StatusNotFound, "空间不存在")
		return
	}
	var body struct {
		Name       string `json:"name"`
		CardPrefix string `json:"card_prefix"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, "参数错误")
		return
	}
	if err := services.UpdateSpace(h.DB, &space, body.Name, body.CardPrefix); err != nil {
		serviceFail(c, err)
		return
	}
	ok(c, gin.H{"space": space, "message": "已更新空间 " + space.Name})
}

func (h *SpaceHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var count int64
	if err := h.DB.Model(&models.Space{}).Count(&count).Error; err != nil {
		serviceFail(c, err)
		return
	}
	if count <= 1 {
		fail(c, http.StatusBadRequest, "至少保留一个空间")
		return
	}
	var space models.Space
	if err := h.DB.First(&space, id).Error; err != nil {
		fail(c, http.StatusNotFound, "空间不存在")
		return
	}
	name := space.Name
	if _, err := services.DeleteSpace(h.DB, &space); err != nil {
		serviceFail(c, err)
		return
	}
	ok(c, gin.H{"message": "已删除空间 " + name})
}

func (h *SpaceHandler) ClearCurrent(c *gin.Context) {
	space, err := resolveSpace(c, h.DB)
	if err != nil {
		serviceFail(c, err)
		return
	}
	counts, err := services.DeleteSpaceData(h.DB, space.ID)
	if err != nil {
		serviceFail(c, err)
		return
	}
	ok(c, gin.H{
		"message": "已清空空间 " + space.Name,
		"counts":  counts,
	})
}
