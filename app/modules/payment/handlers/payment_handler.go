package handlers

import (
	invoiceEntity "payment-sandbox/app/modules/invoice/models/entity"
	paymentEntity "payment-sandbox/app/modules/payment/models/entity"
	appErrors "payment-sandbox/app/shared/errors"
	"payment-sandbox/app/shared/journeylog"
	"payment-sandbox/app/shared/response"

	"github.com/gin-gonic/gin"
)

type PaymentHandler struct {
	service       PaymentService
	journeyLogger journeylog.JourneyLogger
}

type PaymentService interface {
	PublicInvoiceByToken(token string) (invoiceEntity.Invoice, error)
	CreatePaymentIntent(token, method string) (paymentEntity.PaymentIntent, invoiceEntity.Invoice, error)
	ListPaymentIntents(status string) []paymentEntity.PaymentIntent
	UpdatePaymentIntentStatus(paymentID, status string) (paymentEntity.PaymentIntent, invoiceEntity.Invoice, error)
}

func NewPaymentHandler(service PaymentService, journeyLogger journeylog.JourneyLogger) *PaymentHandler {
	return &PaymentHandler{service: service, journeyLogger: journeyLogger}
}

type CreatePaymentIntentRequest struct {
	Method string `json:"method" binding:"required"`
}

type UpdatePaymentIntentStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

func (h *PaymentHandler) PublicInvoice(c *gin.Context) {
	invoice, err := h.service.PublicInvoiceByToken(c.Param("token"))
	if err != nil {
		response.Fail(c, appErrors.NotFound("invoice_not_found", err.Error(), nil))
		return
	}
	response.OK(c, invoice)
}

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

func (h *PaymentHandler) ListPaymentIntents(c *gin.Context) {
	response.OK(c, h.service.ListPaymentIntents(c.Query("status")))
}

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
