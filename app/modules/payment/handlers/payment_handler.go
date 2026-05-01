package handlers

import (
	invoiceEntity "payment-sandbox/app/modules/invoice/models/entity"
	paymentEntity "payment-sandbox/app/modules/payment/models/entity"
	paymentServices "payment-sandbox/app/modules/payment/services"
	"payment-sandbox/app/shared/audit"
	appErrors "payment-sandbox/app/shared/errors"
	"payment-sandbox/app/shared/response"

	"github.com/gin-gonic/gin"
)

type PaymentHandler struct {
	service     paymentServices.IPaymentService
	auditLogger audit.IAuditLogger
}

func NewPaymentHandler(service paymentServices.IPaymentService, auditLogger audit.IAuditLogger) *PaymentHandler {
	return &PaymentHandler{service: service, auditLogger: auditLogger}
}

type CreatePaymentIntentRequest struct {
	Method string `json:"method" binding:"required" example:"WALLET" enums:"WALLET,VA_DUMMY,EWALLET_DUMMY"`
}

type UpdatePaymentIntentStatusRequest struct {
	Status string `json:"status" binding:"required" example:"SUCCESS" enums:"SUCCESS,FAILED"`
}

type PaymentIntentCreateResponse struct {
	PaymentIntent paymentEntity.PaymentIntent `json:"payment_intent"`
	Invoice       invoiceEntity.Invoice       `json:"invoice"`
}

type PublicInvoiceResponse = invoiceEntity.Invoice

type PaymentIntentWithAmount struct {
	ID        string                      `json:"id"`
	InvoiceID string                      `json:"invoice_id"`
	Method    paymentEntity.PaymentMethod `json:"method"`
	Status    paymentEntity.PaymentStatus `json:"status"`
	Amount    int64                       `json:"amount"`
	CreatedAt string                      `json:"created_at"`
	UpdatedAt string                      `json:"updated_at"`
}

type PaymentIntentListResponse []PaymentIntentWithAmount

type PaymentIntentStatusUpdateResponse struct {
	PaymentIntent paymentEntity.PaymentIntent `json:"payment_intent"`
	Invoice       invoiceEntity.Invoice       `json:"invoice"`
}

// PublicInvoice godoc
// @Summary Get invoice by payment token
// @Description Public endpoint to fetch invoice detail by payment link token
// @Tags payment
// @Produce json
// @Param token path string true "Payment token"
// @Success 200 {object} response.Envelope{data=handlers.PublicInvoiceResponse}
// @Failure 404 {object} response.Envelope{error=response.ErrorPayload}
// @Router /pay/{token} [get]
func (h *PaymentHandler) PublicInvoice(c *gin.Context) {
	invoice, err := h.service.PublicInvoiceByToken(c.Param("token"))
	if err != nil {
		response.Fail(c, appErrors.NotFound("invoice_not_found", err.Error(), nil))
		return
	}
	response.OK(c, invoice)
}

// CreatePaymentIntent godoc
// @Summary Create payment intent
// @Description Public endpoint to create payment intent for an invoice token
// @Tags payment
// @Accept json
// @Produce json
// @Param token path string true "Payment token"
// @Param request body CreatePaymentIntentRequest true "Payment intent payload"
// @Success 201 {object} response.Envelope{data=handlers.PaymentIntentCreateResponse}
// @Failure 400 {object} response.Envelope{error=response.ErrorPayload}
// @Router /pay/{token}/intents [post]
func (h *PaymentHandler) CreatePaymentIntent(c *gin.Context) {
	var req CreatePaymentIntentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, appErrors.BadRequest("validation_error", "invalid request payload", err.Error()))
		return
	}

	intent, invoice, err := h.service.CreatePaymentIntent(c.Param("token"), req.Method)
	if err != nil {
		actorID, actorType := audit.ActorFromContext(c)
		audit.LogBestEffort(c, h.auditLogger, audit.Event{
			RequestID: audit.RequestIDFromContext(c),
			ActorID:   actorID,
			ActorType: actorType,
			EventType: "payment.intent_created",
			Metadata: map[string]any{
				"method":        req.Method,
				"result":        "FAILED",
				"error_code":    "payment_intent_create_failed",
				"error_message": err.Error(),
			},
		})
		response.Fail(c, appErrors.BadRequest("payment_intent_create_failed", err.Error(), nil))
		return
	}

	actorID, actorType := audit.ActorFromContext(c)
	audit.LogBestEffort(c, h.auditLogger, audit.Event{
		RequestID:  audit.RequestIDFromContext(c),
		ActorID:    actorID,
		ActorType:  actorType,
		EventType:  "payment.intent_created",
		ResourceID: intent.ID,
		Metadata: map[string]any{
			"invoice_id": invoice.ID,
			"method":     string(intent.Method),
			"to_status":  string(intent.Status),
			"result":     "SUCCESS",
			"journey_id": invoice.ID,
		},
	})
	response.Created(c, gin.H{"payment_intent": intent, "invoice": invoice})
}

// ListPaymentIntents godoc
// @Summary List payment intents
// @Description Admin lists payment intents with optional status filter
// @Tags payment
// @Produce json
// @Security BearerAuth
// @Param status query string false "Payment intent status" Enums(PENDING,SUCCESS,FAILED)
// @Success 200 {object} response.Envelope{data=handlers.PaymentIntentListResponse}
// @Failure 401 {object} response.Envelope{error=response.ErrorPayload}
// @Failure 403 {object} response.Envelope{error=response.ErrorPayload}
// @Router /admin/payment-intents [get]
func (h *PaymentHandler) ListPaymentIntents(c *gin.Context) {
	intents := h.service.ListPaymentIntents(c.Query("status"))

	// Convert to response with amounts
	result := make(PaymentIntentListResponse, len(intents))
	for i, intent := range intents {
		invoice, err := h.service.GetInvoiceByID(intent.InvoiceID)
		var amount int64
		if err == nil {
			amount = invoice.Amount
		}

		result[i] = PaymentIntentWithAmount{
			ID:        intent.ID,
			InvoiceID: intent.InvoiceID,
			Method:    intent.Method,
			Status:    intent.Status,
			Amount:    amount,
			CreatedAt: intent.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt: intent.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	response.OK(c, result)
}

// UpdatePaymentIntentStatus godoc
// @Summary Update payment intent status
// @Description Admin updates payment intent status to SUCCESS or FAILED
// @Tags payment
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Payment intent ID"
// @Param request body UpdatePaymentIntentStatusRequest true "Payment status payload"
// @Success 200 {object} response.Envelope{data=handlers.PaymentIntentStatusUpdateResponse}
// @Failure 400 {object} response.Envelope{error=response.ErrorPayload}
// @Failure 401 {object} response.Envelope{error=response.ErrorPayload}
// @Failure 403 {object} response.Envelope{error=response.ErrorPayload}
// @Router /admin/payment-intents/{id}/status [patch]
func (h *PaymentHandler) UpdatePaymentIntentStatus(c *gin.Context) {
	var req UpdatePaymentIntentStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, appErrors.BadRequest("validation_error", "invalid request payload", err.Error()))
		return
	}

	intent, invoice, err := h.service.UpdatePaymentIntentStatus(c.Param("id"), req.Status)
	if err != nil {
		actorID, actorType := audit.ActorFromContext(c)
		audit.LogBestEffort(c, h.auditLogger, audit.Event{
			RequestID:  audit.RequestIDFromContext(c),
			ActorID:    actorID,
			ActorType:  actorType,
			EventType:  "payment.status_updated",
			ResourceID: c.Param("id"),
			Metadata: map[string]any{
				"to_status":     req.Status,
				"result":        "FAILED",
				"error_code":    "payment_intent_update_failed",
				"error_message": err.Error(),
				"journey_id":    c.Param("id"),
			},
		})
		response.Fail(c, appErrors.BadRequest("payment_intent_update_failed", err.Error(), nil))
		return
	}

	actorID, actorType := audit.ActorFromContext(c)
	audit.LogBestEffort(c, h.auditLogger, audit.Event{
		RequestID:  audit.RequestIDFromContext(c),
		ActorID:    actorID,
		ActorType:  actorType,
		EventType:  "payment.status_updated",
		ResourceID: intent.ID,
		Metadata: map[string]any{
			"invoice_id": invoice.ID,
			"to_status":  string(intent.Status),
			"result":     "SUCCESS",
			"journey_id": invoice.ID,
		},
	})
	response.OK(c, gin.H{"payment_intent": intent, "invoice": invoice})
}
