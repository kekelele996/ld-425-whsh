package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/home-renovation/platform/internal/dto"
	"github.com/home-renovation/platform/internal/model"
	"github.com/home-renovation/platform/internal/service"
	"github.com/home-renovation/platform/internal/utils"
)

// MaterialHandler 材料处理器。
type MaterialHandler struct {
	service service.MaterialService
}

// NewMaterialHandler 构造材料处理器。
func NewMaterialHandler(service service.MaterialService) *MaterialHandler {
	return &MaterialHandler{service: service}
}

// Create 创建材料项。
func (h *MaterialHandler) Create(c *gin.Context) {
	var req dto.CreateMaterialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(err)
		return
	}
	item, err := h.service.Create(&req)
	if err != nil {
		c.Error(err)
		return
	}
	installed, err := h.service.InstalledQuantity(item.ID)
	if err != nil {
		c.Error(err)
		return
	}
	utils.Success(c, toMaterialDTO(item, installed))
}

// Get 获取材料项详情。
func (h *MaterialHandler) Get(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.Error(err)
		return
	}
	item, err := h.service.GetByID(id)
	if err != nil {
		c.Error(err)
		return
	}
	installed, err := h.service.InstalledQuantity(item.ID)
	if err != nil {
		c.Error(err)
		return
	}
	utils.Success(c, toMaterialDTO(item, installed))
}

// List 获取材料列表。
func (h *MaterialHandler) List(c *gin.Context) {
	page, pageSize := parsePage(c)
	projectIDStr := c.Query("project_id")
	category := c.Query("category")
	space := c.Query("space")
	if projectIDStr != "" {
		id, err := parseUintString(projectIDStr)
		if err != nil {
			c.Error(err)
			return
		}
		items, err := h.service.ListByProjectID(id)
		if err != nil {
			c.Error(err)
			return
		}
		dtos, err := h.toMaterialDTOs(items)
		if err != nil {
			c.Error(err)
			return
		}
		utils.Success(c, dtos)
		return
	}
	items, total, err := h.service.List(0, category, space, page, pageSize)
	if err != nil {
		c.Error(err)
		return
	}
	dtos, err := h.toMaterialDTOs(items)
	if err != nil {
		c.Error(err)
		return
	}
	utils.Success(c, utils.PageResult{List: dtos, Total: total, Page: page, PageSize: pageSize})
}

// toMaterialDTOs 批量附带已安装量/剩余量。
func (h *MaterialHandler) toMaterialDTOs(items []model.MaterialItem) ([]dto.MaterialDTO, error) {
	ids := make([]uint, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	installedMap, err := h.service.InstalledQuantities(ids)
	if err != nil {
		return nil, err
	}
	out := make([]dto.MaterialDTO, 0, len(items))
	for _, item := range items {
		out = append(out, toMaterialDTO(&item, installedMap[item.ID]))
	}
	return out, nil
}

// Update 更新材料项。
func (h *MaterialHandler) Update(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.Error(err)
		return
	}
	var req dto.UpdateMaterialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(err)
		return
	}
	item, err := h.service.Update(id, &req)
	if err != nil {
		c.Error(err)
		return
	}
	installed, err := h.service.InstalledQuantity(item.ID)
	if err != nil {
		c.Error(err)
		return
	}
	utils.Success(c, toMaterialDTO(item, installed))
}

// Delete 删除材料项。
func (h *MaterialHandler) Delete(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.Error(err)
		return
	}
	if err := h.service.Delete(id); err != nil {
		c.Error(err)
		return
	}
	utils.SuccessMessage(c, "deleted", nil)
}

// UpdateStatus 采购状态流转。
func (h *MaterialHandler) UpdateStatus(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.Error(err)
		return
	}
	var req dto.UpdateMaterialStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(err)
		return
	}
	item, err := h.service.UpdateStatus(id, req.Status)
	if err != nil {
		c.Error(err)
		return
	}
	installed, err := h.service.InstalledQuantity(item.ID)
	if err != nil {
		c.Error(err)
		return
	}
	utils.Success(c, toMaterialDTO(item, installed))
}
