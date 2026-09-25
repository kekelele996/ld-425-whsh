package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/home-renovation/platform/internal/dto"
	"github.com/home-renovation/platform/internal/model"
	"github.com/home-renovation/platform/internal/service"
	"github.com/home-renovation/platform/internal/utils"
)

// ConstructionHandler 施工节点处理器。
type ConstructionHandler struct {
	service    service.ConstructionService
	usageSvc   service.MaterialUsageService
	materialSvc service.MaterialService
}

// NewConstructionHandler 构造施工处理器。
func NewConstructionHandler(service service.ConstructionService, usageSvc service.MaterialUsageService, materialSvc service.MaterialService) *ConstructionHandler {
	return &ConstructionHandler{service: service, usageSvc: usageSvc, materialSvc: materialSvc}
}

// Create 创建施工节点。
func (h *ConstructionHandler) Create(c *gin.Context) {
	var req dto.CreateConstructionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(err)
		return
	}
	node, err := h.service.Create(&req)
	if err != nil {
		c.Error(err)
		return
	}
	utils.Success(c, toConstructionDTO(node))
}

// Get 获取施工节点详情。
func (h *ConstructionHandler) Get(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.Error(err)
		return
	}
	node, err := h.service.GetByID(id)
	if err != nil {
		c.Error(err)
		return
	}
	nodeDTO := toConstructionDTO(node)
	if err := h.attachUsages(&nodeDTO, node.ProjectID); err != nil {
		c.Error(err)
		return
	}
	utils.Success(c, nodeDTO)
}

// List 获取施工节点列表。
func (h *ConstructionHandler) List(c *gin.Context) {
	page, pageSize := parsePage(c)
	if projectIDStr := c.Query("project_id"); projectIDStr != "" {
		id, err := parseUintString(projectIDStr)
		if err != nil {
			c.Error(err)
			return
		}
		nodes, err := h.service.ListByProjectID(id)
		if err != nil {
			c.Error(err)
			return
		}
		usageMap, err := h.usageMapByProject(id)
		if err != nil {
			c.Error(err)
			return
		}
		utils.Success(c, toConstructionDTOList(nodes, usageMap))
		return
	}
	status := c.Query("status")
	nodes, total, err := h.service.List(0, status, page, pageSize)
	if err != nil {
		c.Error(err)
		return
	}
	usageMap, err := h.usageMapForNodes(nodes)
	if err != nil {
		c.Error(err)
		return
	}
	utils.Success(c, utils.PageResult{List: toConstructionDTOList(nodes, usageMap), Total: total, Page: page, PageSize: pageSize})
}

// Update 更新施工节点。
func (h *ConstructionHandler) Update(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.Error(err)
		return
	}
	var req dto.UpdateConstructionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(err)
		return
	}
	node, err := h.service.Update(id, &req)
	if err != nil {
		c.Error(err)
		return
	}
	utils.Success(c, toConstructionDTO(node))
}

// Delete 删除施工节点。
func (h *ConstructionHandler) Delete(c *gin.Context) {
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

// UpdateStatus 施工状态流转。
func (h *ConstructionHandler) UpdateStatus(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.Error(err)
		return
	}
	var req dto.UpdateConstructionStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(err)
		return
	}
	node, err := h.service.UpdateStatus(id, req.Status)
	if err != nil {
		c.Error(err)
		return
	}
	utils.Success(c, toConstructionDTO(node))
}

// Accept 施工验收。
func (h *ConstructionHandler) Accept(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.Error(err)
		return
	}
	var req dto.AcceptConstructionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(err)
		return
	}
	node, err := h.service.Accept(id, &req)
	if err != nil {
		c.Error(err)
		return
	}
	nodeDTO := toConstructionDTO(node)
	if err := h.attachUsages(&nodeDTO, node.ProjectID); err != nil {
		c.Error(err)
		return
	}
	utils.Success(c, nodeDTO)
}

// attachUsages 为单个节点 DTO 填充该节点的用料登记明细。
func (h *ConstructionHandler) attachUsages(nodeDTO *dto.ConstructionDTO, projectID uint) error {
	materials, err := h.materialSvc.ListByProjectID(projectID)
	if err != nil {
		return err
	}
	materialMap := make(map[uint]*model.MaterialItem, len(materials))
	for i := range materials {
		materialMap[materials[i].ID] = &materials[i]
	}
	usages, err := h.usageSvc.ListByNode(nodeDTO.ID)
	if err != nil {
		return err
	}
	usageMap := groupUsagesByNode(usages, materialMap)
	nodeDTO.Usages = usageMap[nodeDTO.ID]
	if nodeDTO.Usages == nil {
		nodeDTO.Usages = []dto.MaterialUsageDTO{}
	}
	return nil
}

// usageMapByProject 按项目一次性加载全部用料明细并按节点分组。
func (h *ConstructionHandler) usageMapByProject(projectID uint) (map[uint][]dto.MaterialUsageDTO, error) {
	usages, err := h.usageSvc.ListByProject(projectID)
	if err != nil {
		return nil, err
	}
	materials, err := h.materialSvc.ListByProjectID(projectID)
	if err != nil {
		return nil, err
	}
	materialMap := make(map[uint]*model.MaterialItem, len(materials))
	for i := range materials {
		materialMap[materials[i].ID] = &materials[i]
	}
	return groupUsagesByNode(usages, materialMap), nil
}

// usageMapForNodes 跨项目节点列表（分页）按所属项目分组加载用料明细。
func (h *ConstructionHandler) usageMapForNodes(nodes []model.ConstructionNode) (map[uint][]dto.MaterialUsageDTO, error) {
	out := make(map[uint][]dto.MaterialUsageDTO)
	projectSet := map[uint]struct{}{}
	for _, node := range nodes {
		projectSet[node.ProjectID] = struct{}{}
	}
	for projectID := range projectSet {
		m, err := h.usageMapByProject(projectID)
		if err != nil {
			return nil, err
		}
		for nodeID, usages := range m {
			out[nodeID] = usages
		}
	}
	return out, nil
}
