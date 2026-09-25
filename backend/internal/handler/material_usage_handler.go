package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/home-renovation/platform/internal/dto"
	apperrors "github.com/home-renovation/platform/internal/errors"
	"github.com/home-renovation/platform/internal/service"
	"github.com/home-renovation/platform/internal/utils"
)

// MaterialUsageHandler 节点用料处理器。
type MaterialUsageHandler struct {
	usageService service.MaterialUsageService
}

// NewMaterialUsageHandler 构造节点用料处理器。
func NewMaterialUsageHandler(usageService service.MaterialUsageService) *MaterialUsageHandler {
	return &MaterialUsageHandler{usageService: usageService}
}

// Register 节点完工前登记用料。
func (h *MaterialUsageHandler) Register(c *gin.Context) {
	var req dto.RegisterMaterialUsageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(err)
		return
	}
	detail, err := h.usageService.Register(&req)
	if err != nil {
		c.Error(err)
		return
	}
	utils.Success(c, toUsageDetailDTO(*detail))
}

// ListByNode 查看某节点的用料明细。
func (h *MaterialUsageHandler) ListByNode(c *gin.Context) {
	id, err := parseUintString(c.Query("node_id"))
	if err != nil || id == 0 {
		c.Error(apperrors.NewBadRequest("node_id query parameter is required"))
		return
	}
	details, err := h.usageService.ListDetailsByNode(id)
	if err != nil {
		c.Error(err)
		return
	}
	utils.Success(c, toUsageDetailDTOList(details))
}
