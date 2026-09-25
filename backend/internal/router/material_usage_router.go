package router

import (
	"github.com/gin-gonic/gin"
	"github.com/home-renovation/platform/internal/constants"
	"github.com/home-renovation/platform/internal/handler"
)

// registerMaterialUsageRoutes 注册节点用料登记路由。
func registerMaterialUsageRoutes(group *gin.RouterGroup, h *handler.MaterialUsageHandler, auth gin.HandlerFunc, rbac func(...string) gin.HandlerFunc) {
	// 项目维度查看用料（施工页展开）。
	group.GET("/material-usages", auth, h.ListByProject)
	group.GET("/material-usages/:id", auth, h.Get)
	// 单条用料登记删除（仅未确认且节点未通过验收）。
	group.DELETE("/material-usages/:id", auth, rbac(constants.RoleAdmin, constants.RoleProjectManager, constants.RoleContractor), h.Delete)
	// 节点维度：节点完工前登记本次用料、查看节点用料。
	group.POST("/constructions/:id/usages", auth, rbac(constants.RoleAdmin, constants.RoleProjectManager, constants.RoleContractor), h.Register)
	group.GET("/constructions/:id/usages", auth, h.ListByNode)
}
