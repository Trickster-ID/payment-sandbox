package handlers

import (
	adminEntity "payment-sandbox/app/modules/admin/models/entity"
	adminServices "payment-sandbox/app/modules/admin/services"
	appErrors "payment-sandbox/app/shared/errors"
	"payment-sandbox/app/shared/response"

	"github.com/gin-gonic/gin"
)

type AdminHandler struct {
	service adminServices.IAdminService
}

type HealthResponse struct {
	Status string `json:"status" example:"ok"`
}

type DashboardStatsResponse = adminEntity.DashboardStats

func NewAdminHandler(service adminServices.IAdminService) *AdminHandler {
	return &AdminHandler{service: service}
}

// Healthz godoc
// @Summary Health check
// @Description Check API health status
// @Tags system
// @Produce json
// @Success 200 {object} response.Envelope{data=handlers.HealthResponse}
// @Router /ping [get]
func (h *AdminHandler) Healthz(c *gin.Context) {
	response.OK(c, gin.H{"status": "ok"})
}

// DashboardStats godoc
// @Summary Admin dashboard stats
// @Description Get aggregated invoice/payment/refund stats with optional filters
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param merchant_id query string false "Merchant ID"
// @Param start_date query string false "Start date (YYYY-MM-DD)"
// @Param end_date query string false "End date (YYYY-MM-DD)"
// @Success 200 {object} response.Envelope{data=handlers.DashboardStatsResponse}
// @Failure 400 {object} response.Envelope{error=response.ErrorPayload}
// @Failure 401 {object} response.Envelope{error=response.ErrorPayload}
// @Failure 403 {object} response.Envelope{error=response.ErrorPayload}
// @Router /admin/stats [get]
func (h *AdminHandler) DashboardStats(c *gin.Context) {
	stats, err := h.service.Stats(c.Query("merchant_id"), c.Query("start_date"), c.Query("end_date"))
	if err != nil {
		response.Fail(c, appErrors.BadRequest("stats_query_failed", err.Error(), nil))
		return
	}
	response.OK(c, stats)
}
