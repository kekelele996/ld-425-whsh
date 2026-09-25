package router

import (
	"github.com/gin-gonic/gin"
	"github.com/home-renovation/platform/internal/constants"
	"github.com/home-renovation/platform/internal/handler"
)

// registerMaterialUsageRoutes 注册节点用料路由。
func registerMaterialUsageRoutes(group *gin.RouterGroup, h *handler.MaterialUsageHandler, auth gin.HandlerFunc, rbac func(...string) gin.HandlerFunc) {
	group.GET("/material-usages/node", auth, h.ListByNode)
	group.POST("/material-usages", auth, rbac(constants.RoleAdmin, constants.RoleContractor, constants.RoleProjectManager), h.Register)
}
