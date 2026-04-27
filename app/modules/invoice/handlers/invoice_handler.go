package handlers

import (
	"payment-sandbox/app/middleware"
	invoiceServices "payment-sandbox/app/modules/invoice/services"
	appErrors "payment-sandbox/app/shared/errors"
	"payment-sandbox/app/shared/journeylog"
	"payment-sandbox/app/shared/pagination"
	"payment-sandbox/app/shared/response"

	"github.com/gin-gonic/gin"
)

type InvoiceHandler struct {
	service       invoiceServices.IInvoiceService
	journeyLogger journeylog.IJourneyLogger
}

func NewInvoiceHandler(service invoiceServices.IInvoiceService, journeyLogger journeylog.IJourneyLogger) *InvoiceHandler {
	return &InvoiceHandler{service: service, journeyLogger: journeyLogger}
}

type CreateInvoiceRequest struct {
	CustomerName  string  `json:"customer_name" binding:"required"`
	CustomerEmail string  `json:"customer_email" binding:"required,email"`
	Amount        float64 `json:"amount" binding:"required,gt=0"`
	Description   string  `json:"description"`
	DueDate       string  `json:"due_date" binding:"required"`
}

// CreateInvoice godoc
// @Summary Create invoice
// @Description Merchant creates a new invoice
// @Tags invoice
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateInvoiceRequest true "Create invoice payload"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /merchant/invoices [post]
func (h *InvoiceHandler) CreateInvoice(c *gin.Context) {
	userID, ok := middleware.MustUserID(c)
	if !ok {
		return
	}

	var req CreateInvoiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, appErrors.BadRequest("validation_error", "invalid request payload", err.Error()))
		return
	}

	invoice, err := h.service.CreateInvoice(userID, req.CustomerName, req.CustomerEmail, req.Amount, req.Description, req.DueDate)
	if err != nil {
		actorID, actorRole := journeylog.ActorFromContext(c)
		journeylog.LogBestEffort(c, h.journeyLogger, journeylog.Event{
			RequestID:    journeylog.RequestIDFromContext(c),
			ActorID:      actorID,
			ActorRole:    actorRole,
			Module:       "invoice",
			EntityType:   "invoice",
			Action:       "INVOICE_CREATE",
			Result:       "FAILED",
			ErrorCode:    "invoice_create_failed",
			ErrorMessage: err.Error(),
			Metadata: map[string]any{
				"amount": req.Amount,
			},
		})
		response.Fail(c, appErrors.BadRequest("invoice_create_failed", err.Error(), nil))
		return
	}

	actorID, actorRole := journeylog.ActorFromContext(c)
	journeylog.LogBestEffort(c, h.journeyLogger, journeylog.Event{
		RequestID:  journeylog.RequestIDFromContext(c),
		JourneyID:  invoice.ID,
		ActorID:    actorID,
		ActorRole:  actorRole,
		Module:     "invoice",
		EntityType: "invoice",
		EntityID:   invoice.ID,
		Action:     "INVOICE_CREATE",
		ToStatus:   string(invoice.Status),
		Result:     "SUCCESS",
		Metadata: map[string]any{
			"amount": invoice.Amount,
		},
	})
	response.Created(c, invoice)
}

// ListInvoices godoc
// @Summary List merchant invoices
// @Description Merchant lists invoices with optional status and pagination
// @Tags invoice
// @Produce json
// @Security BearerAuth
// @Param status query string false "Invoice status"
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Page size" default(10)
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /merchant/invoices [get]
func (h *InvoiceHandler) ListInvoices(c *gin.Context) {
	userID, ok := middleware.MustUserID(c)
	if !ok {
		return
	}

	params := pagination.Parse(c.DefaultQuery("page", "1"), c.DefaultQuery("limit", "10"))
	status := c.Query("status")

	invoices, total, err := h.service.ListInvoices(userID, status, params.Page, params.Limit)
	if err != nil {
		response.Fail(c, appErrors.BadRequest("invoice_list_failed", err.Error(), nil))
		return
	}

	response.OKWithMeta(c, invoices, gin.H{
		"page":  params.Page,
		"limit": params.Limit,
		"total": total,
	})
}

// GetInvoice godoc
// @Summary Get merchant invoice detail
// @Description Merchant gets invoice detail by ID
// @Tags invoice
// @Produce json
// @Security BearerAuth
// @Param id path string true "Invoice ID"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /merchant/invoices/{id} [get]
func (h *InvoiceHandler) GetInvoice(c *gin.Context) {
	userID, ok := middleware.MustUserID(c)
	if !ok {
		return
	}

	invoice, err := h.service.InvoiceByID(userID, c.Param("id"))
	if err != nil {
		response.Fail(c, appErrors.NotFound("invoice_not_found", err.Error(), nil))
		return
	}
	response.OK(c, invoice)
}
