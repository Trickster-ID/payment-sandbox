package handlers

import (
	adminServices "payment-sandbox/app/modules/admin/services"
	appErrors "payment-sandbox/app/shared/errors"
	"payment-sandbox/app/shared/response"

	"github.com/gin-gonic/gin"
)

type AdminHandler struct {
	service adminServices.IAdminService
}

func NewAdminHandler(service adminServices.IAdminService) *AdminHandler {
	return &AdminHandler{service: service}
}

func (h *AdminHandler) Healthz(c *gin.Context) {
	response.OK(c, gin.H{"status": "ok"})
}

func (h *AdminHandler) DashboardStats(c *gin.Context) {
	stats, err := h.service.Stats(c.Query("merchant_id"), c.Query("start_date"), c.Query("end_date"))
	if err != nil {
		response.Fail(c, appErrors.BadRequest("stats_query_failed", err.Error(), nil))
		return
	}
	response.OK(c, stats)
}
