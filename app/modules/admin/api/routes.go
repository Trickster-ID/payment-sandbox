package api

import (
	adminHandlers "payment-sandbox/app/modules/admin/handlers"

	"github.com/gin-gonic/gin"
)

func RegisterAdminRoutes(admin *gin.RouterGroup, handler *adminHandlers.AdminHandler) {
	admin.GET("/stats", handler.DashboardStats)
}
