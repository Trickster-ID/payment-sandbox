package handlers

import (
	"context"

	merchantEntity "payment-sandbox/app/modules/merchants/models/entity"
	merchantServices "payment-sandbox/app/modules/merchants/services"
	appErrors "payment-sandbox/app/shared/errors"
	"payment-sandbox/app/shared/pagination"
	"payment-sandbox/app/shared/response"

	"github.com/gin-gonic/gin"
)

type MerchantsHandler struct {
	service merchantServices.IMerchantsService
}

func NewMerchantsHandler(service merchantServices.IMerchantsService) *MerchantsHandler {
	return &MerchantsHandler{service: service}
}

type MerchantSummaryResponse = merchantEntity.MerchantSummary

type MerchantListResponse []merchantEntity.MerchantSummary

// ListMerchants godoc
// @Summary List merchants
// @Description Admin lists merchants with optional prefix search on name or email. Powers the admin merchant picker.
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param search query string false "Prefix search on name or email (case-insensitive)"
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Page size" default(20)
// @Success 200 {object} response.Envelope{data=handlers.MerchantListResponse,meta=response.PaginationMeta}
// @Failure 400 {object} response.Envelope{error=response.ErrorPayload}
// @Failure 401 {object} response.Envelope{error=response.ErrorPayload}
// @Failure 403 {object} response.Envelope{error=response.ErrorPayload}
// @Router /admin/merchants [get]
func (h *MerchantsHandler) ListMerchants(c *gin.Context) {
	search := c.Query("search")
	params := pagination.Parse(c.DefaultQuery("page", "1"), c.DefaultQuery("limit", "20"))

	merchants, total, err := h.service.ListMerchants(context.Background(), search, params.Page, params.Limit)
	if err != nil {
		response.Fail(c, appErrors.BadRequest("merchants_list_failed", err.Error(), nil))
		return
	}

	response.OKWithMeta(c, merchants, gin.H{
		"page":  params.Page,
		"limit": params.Limit,
		"total": total,
	})
}
