package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/home-renovation/platform/internal/dto"
	apperrors "github.com/home-renovation/platform/internal/errors"
	"github.com/home-renovation/platform/internal/model"
	"github.com/home-renovation/platform/internal/service"
	"github.com/home-renovation/platform/internal/utils"
)

// MaterialUsageHandler 节点用料登记处理器。
type MaterialUsageHandler struct {
	usageSvc    service.MaterialUsageService
	materialSvc service.MaterialService
}

// NewMaterialUsageHandler 构造节点用料登记处理器。
func NewMaterialUsageHandler(usageSvc service.MaterialUsageService, materialSvc service.MaterialService) *MaterialUsageHandler {
	return &MaterialUsageHandler{usageSvc: usageSvc, materialSvc: materialSvc}
}

// Register 在指定施工节点登记本次用料。
func (h *MaterialUsageHandler) Register(c *gin.Context) {
	nodeID, err := parseID(c)
	if err != nil {
		c.Error(err)
		return
	}
	var req dto.RegisterMaterialUsageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(err)
		return
	}
	usage, err := h.usageSvc.Register(nodeID, &req)
	if err != nil {
		c.Error(err)
		return
	}
	material, err := h.materialSvc.GetByID(usage.MaterialID)
	if err != nil {
		c.Error(err)
		return
	}
	utils.Success(c, toMaterialUsageDTO(usage, material))
}

// ListByNode 查看指定节点的用料登记。
func (h *MaterialUsageHandler) ListByNode(c *gin.Context) {
	nodeID, err := parseID(c)
	if err != nil {
		c.Error(err)
		return
	}
	usages, err := h.usageSvc.ListByNode(nodeID)
	if err != nil {
		c.Error(err)
		return
	}
	materialMap, err := h.materialMapForUsages(usages)
	if err != nil {
		c.Error(err)
		return
	}
	utils.Success(c, toUsageDTOList(usages, materialMap))
}

// ListByProject 按项目查看全部节点用料登记，供施工页展开使用。
func (h *MaterialUsageHandler) ListByProject(c *gin.Context) {
	projectIDStr := c.Query("project_id")
	if projectIDStr == "" {
		c.Error(apperrors.NewBadRequest("project_id is required"))
		return
	}
	projectID, err := parseUintString(projectIDStr)
	if err != nil {
		c.Error(err)
		return
	}
	usages, err := h.usageSvc.ListByProject(projectID)
	if err != nil {
		c.Error(err)
		return
	}
	materials, err := h.materialSvc.ListByProjectID(projectID)
	if err != nil {
		c.Error(err)
		return
	}
	materialMap := make(map[uint]*model.MaterialItem, len(materials))
	for i := range materials {
		materialMap[materials[i].ID] = &materials[i]
	}
	utils.Success(c, toUsageDTOList(usages, materialMap))
}

// Get 获取单条用料登记。
func (h *MaterialUsageHandler) Get(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.Error(err)
		return
	}
	usage, err := h.usageSvc.GetByID(id)
	if err != nil {
		c.Error(err)
		return
	}
	material, err := h.materialSvc.GetByID(usage.MaterialID)
	if err != nil {
		c.Error(err)
		return
	}
	utils.Success(c, toMaterialUsageDTO(usage, material))
}

// Delete 删除未确认的用料登记。
func (h *MaterialUsageHandler) Delete(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.Error(err)
		return
	}
	if err := h.usageSvc.Delete(id); err != nil {
		c.Error(err)
		return
	}
	utils.SuccessMessage(c, "deleted", nil)
}

// materialMapForUsages 根据用料记录所属项目加载材料信息（名称、单位）。
func (h *MaterialUsageHandler) materialMapForUsages(usages []model.MaterialUsage) (map[uint]*model.MaterialItem, error) {
	materialMap := make(map[uint]*model.MaterialItem)
	if len(usages) == 0 {
		return materialMap, nil
	}
	projectID := usages[0].ProjectID
	materials, err := h.materialSvc.ListByProjectID(projectID)
	if err != nil {
		return nil, err
	}
	for i := range materials {
		materialMap[materials[i].ID] = &materials[i]
	}
	return materialMap, nil
}

func toUsageDTOList(usages []model.MaterialUsage, materialMap map[uint]*model.MaterialItem) []dto.MaterialUsageDTO {
	out := make([]dto.MaterialUsageDTO, 0, len(usages))
	for i := range usages {
		out = append(out, toMaterialUsageDTO(&usages[i], materialMap[usages[i].MaterialID]))
	}
	return out
}
