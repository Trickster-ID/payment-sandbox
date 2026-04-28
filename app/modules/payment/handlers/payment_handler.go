package handlers

import (
	invoiceEntity "payment-sandbox/app/modules/invoice/models/entity"
	paymentEntity "payment-sandbox/app/modules/payment/models/entity"
	paymentServices "payment-sandbox/app/modules/payment/services"
	appErrors "payment-sandbox/app/shared/errors"
	"payment-sandbox/app/shared/journeylog"
	"payment-sandbox/app/shared/response"

	"github.com/gin-gonic/gin"
)

type PaymentHandler struct {
	service       paymentServices.IPaymentService
	journeyLogger journeylog.IJourneyLogger
}

func NewPaymentHandler(service paymentServices.IPaymentService, journeyLogger journeylog.IJourneyLogger) *PaymentHandler {
	return &PaymentHandler{service: service, journeyLogger: journeyLogger}
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

type PaymentIntentListResponse []paymentEntity.PaymentIntent

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
		actorID, actorRole := journeylog.ActorFromContext(c)
		journeylog.LogBestEffort(c, h.journeyLogger, journeylog.Event{
			RequestID:    journeylog.RequestIDFromContext(c),
			ActorID:      actorID,
			ActorRole:    actorRole,
			Module:       "payment",
			EntityType:   "payment_intent",
			Action:       "PAYMENT_INTENT_CREATE",
			ToStatus:     req.Method,
			Result:       "FAILED",
			ErrorCode:    "payment_intent_create_failed",
			ErrorMessage: err.Error(),
		})
		response.Fail(c, appErrors.BadRequest("payment_intent_create_failed", err.Error(), nil))
		return
	}

	actorID, actorRole := journeylog.ActorFromContext(c)
	journeylog.LogBestEffort(c, h.journeyLogger, journeylog.Event{
		RequestID:  journeylog.RequestIDFromContext(c),
		JourneyID:  invoice.ID,
		ActorID:    actorID,
		ActorRole:  actorRole,
		Module:     "payment",
		EntityType: "payment_intent",
		EntityID:   intent.ID,
		Action:     "PAYMENT_INTENT_CREATE",
		ToStatus:   string(intent.Status),
		Result:     "SUCCESS",
		Metadata: map[string]any{
			"invoice_id": invoice.ID,
			"method":     intent.Method,
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
	response.OK(c, h.service.ListPaymentIntents(c.Query("status")))
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
		actorID, actorRole := journeylog.ActorFromContext(c)
		journeylog.LogBestEffort(c, h.journeyLogger, journeylog.Event{
			RequestID:    journeylog.RequestIDFromContext(c),
			JourneyID:    c.Param("id"),
			ActorID:      actorID,
			ActorRole:    actorRole,
			Module:       "payment",
			EntityType:   "payment_intent",
			EntityID:     c.Param("id"),
			Action:       "PAYMENT_INTENT_STATUS_UPDATE",
			ToStatus:     req.Status,
			Result:       "FAILED",
			ErrorCode:    "payment_intent_update_failed",
			ErrorMessage: err.Error(),
		})
		response.Fail(c, appErrors.BadRequest("payment_intent_update_failed", err.Error(), nil))
		return
	}

	actorID, actorRole := journeylog.ActorFromContext(c)
	journeylog.LogBestEffort(c, h.journeyLogger, journeylog.Event{
		RequestID:  journeylog.RequestIDFromContext(c),
		JourneyID:  invoice.ID,
		ActorID:    actorID,
		ActorRole:  actorRole,
		Module:     "payment",
		EntityType: "payment_intent",
		EntityID:   intent.ID,
		Action:     "PAYMENT_INTENT_STATUS_UPDATE",
		ToStatus:   string(intent.Status),
		Result:     "SUCCESS",
		Metadata: map[string]any{
			"invoice_id": invoice.ID,
		},
	})
	response.OK(c, gin.H{"payment_intent": intent, "invoice": invoice})
}
