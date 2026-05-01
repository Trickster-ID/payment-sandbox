package handlers

import (
	"payment-sandbox/app/middleware"
	refundEntity "payment-sandbox/app/modules/refund/models/entity"
	"payment-sandbox/app/modules/refund/services"
	walletEntity "payment-sandbox/app/modules/wallet/models/entity"
	"payment-sandbox/app/shared/audit"
	appErrors "payment-sandbox/app/shared/errors"
	"payment-sandbox/app/shared/response"

	"github.com/gin-gonic/gin"
)

type RefundHandler struct {
	service     services.IRefundService
	auditLogger audit.IAuditLogger
}

func NewRefundHandler(service services.IRefundService, auditLogger audit.IAuditLogger) *RefundHandler {
	return &RefundHandler{service: service, auditLogger: auditLogger}
}

type CreateRefundRequest struct {
	InvoiceID string `json:"invoice_id" binding:"required" example:"0196aee7-80b0-7d57-b38f-26b315d8f9bb"`
	Reason    string `json:"reason" binding:"required" example:"Customer requested cancellation"`
}

type ReviewRefundRequest struct {
	Decision string `json:"decision" binding:"required" example:"APPROVE" enums:"APPROVE,REJECT"`
}

type ProcessRefundRequest struct {
	Status string `json:"status" binding:"required" example:"SUCCESS" enums:"SUCCESS,FAILED"`
}

type RefundProcessResponse struct {
	Refund   refundEntity.Refund   `json:"refund"`
	Merchant walletEntity.Merchant `json:"merchant"`
}

type RefundResponse = refundEntity.Refund

type RefundListResponse []refundEntity.Refund

// RequestRefund godoc
// @Summary Request refund
// @Description Merchant requests refund for successful payment intent. Requires Idempotency-Key header to safely retry on network failure.
// @Tags refund
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Idempotency-Key header string true "Unique key per logical request (UUID recommended). Replaying the same key returns the original response; reusing the key with a different body returns 409."
// @Param request body CreateRefundRequest true "Refund request payload"
// @Success 201 {object} response.Envelope{data=handlers.RefundResponse}
// @Failure 400 {object} response.Envelope{error=response.ErrorPayload} "validation_error or idempotency_key_required"
// @Failure 401 {object} response.Envelope{error=response.ErrorPayload}
// @Failure 403 {object} response.Envelope{error=response.ErrorPayload}
// @Failure 409 {object} response.Envelope{error=response.ErrorPayload} "idempotency_key_conflict or idempotency_in_progress"
// @Router /merchant/refunds [post]
func (h *RefundHandler) RequestRefund(c *gin.Context) {
	userID, ok := middleware.MustUserID(c)
	if !ok {
		return
	}

	var req CreateRefundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, appErrors.BadRequest("validation_error", "invalid request payload", err.Error()))
		return
	}

	refund, err := h.service.RequestRefund(userID, req.InvoiceID, req.Reason)
	if err != nil {
		actorID, actorType := audit.ActorFromContext(c)
		audit.LogBestEffort(c, h.auditLogger, audit.Event{
			RequestID: audit.RequestIDFromContext(c),
			ActorID:   actorID,
			ActorType: actorType,
			EventType: "refund.requested",
			Metadata: map[string]any{
				"result":        "FAILED",
				"error_code":    "refund_request_failed",
				"error_message": err.Error(),
			},
		})
		response.Fail(c, appErrors.BadRequest("refund_request_failed", err.Error(), nil))
		return
	}

	actorID, actorType := audit.ActorFromContext(c)
	audit.LogBestEffort(c, h.auditLogger, audit.Event{
		RequestID:  audit.RequestIDFromContext(c),
		ActorID:    actorID,
		ActorType:  actorType,
		EventType:  "refund.requested",
		ResourceID: refund.ID,
		Metadata: map[string]any{
			"payment_intent_id": refund.PaymentIntentID,
			"to_status":         string(refund.Status),
			"result":            "SUCCESS",
			"journey_id":        refund.ID,
		},
	})
	response.Created(c, refund)
}

// MerchantListRefunds godoc
// @Summary List merchant refunds
// @Description Merchant lists their own refund requests
// @Tags refund
// @Produce json
// @Security BearerAuth
// @Param status query string false "Refund status" Enums(REQUESTED,APPROVED,REJECTED,SUCCESS,FAILED)
// @Success 200 {object} response.Envelope{data=handlers.RefundListResponse}
// @Failure 401 {object} response.Envelope{error=response.ErrorPayload}
// @Failure 403 {object} response.Envelope{error=response.ErrorPayload}
// @Router /merchant/refunds [get]
func (h *RefundHandler) MerchantListRefunds(c *gin.Context) {
	userID, ok := middleware.MustUserID(c)
	if !ok {
		return
	}
	response.OK(c, h.service.MerchantListRefunds(userID, c.Query("status")))
}

// ListRefunds godoc
// @Summary List refunds
// @Description Admin lists refunds with optional status filter
// @Tags refund
// @Produce json
// @Security BearerAuth
// @Param status query string false "Refund status" Enums(REQUESTED,APPROVED,REJECTED,SUCCESS,FAILED)
// @Success 200 {object} response.Envelope{data=handlers.RefundListResponse}
// @Failure 401 {object} response.Envelope{error=response.ErrorPayload}
// @Failure 403 {object} response.Envelope{error=response.ErrorPayload}
// @Router /admin/refunds [get]
func (h *RefundHandler) ListRefunds(c *gin.Context) {
	response.OK(c, h.service.ListRefunds(c.Query("status")))
}

// ReviewRefund godoc
// @Summary Review refund
// @Description Admin reviews refund request (APPROVE or REJECT)
// @Tags refund
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Refund ID"
// @Param request body ReviewRefundRequest true "Refund review payload"
// @Success 200 {object} response.Envelope{data=handlers.RefundResponse}
// @Failure 400 {object} response.Envelope{error=response.ErrorPayload}
// @Failure 401 {object} response.Envelope{error=response.ErrorPayload}
// @Failure 403 {object} response.Envelope{error=response.ErrorPayload}
// @Router /admin/refunds/{id}/review [patch]
func (h *RefundHandler) ReviewRefund(c *gin.Context) {
	var req ReviewRefundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, appErrors.BadRequest("validation_error", "invalid request payload", err.Error()))
		return
	}
	refund, err := h.service.ReviewRefund(c.Param("id"), req.Decision)
	if err != nil {
		actorID, actorType := audit.ActorFromContext(c)
		audit.LogBestEffort(c, h.auditLogger, audit.Event{
			RequestID:  audit.RequestIDFromContext(c),
			ActorID:    actorID,
			ActorType:  actorType,
			EventType:  "refund.reviewed",
			ResourceID: c.Param("id"),
			Metadata: map[string]any{
				"decision":      req.Decision,
				"result":        "FAILED",
				"error_code":    "refund_review_failed",
				"error_message": err.Error(),
				"journey_id":    c.Param("id"),
			},
		})
		response.Fail(c, appErrors.BadRequest("refund_review_failed", err.Error(), nil))
		return
	}

	actorID, actorType := audit.ActorFromContext(c)
	audit.LogBestEffort(c, h.auditLogger, audit.Event{
		RequestID:  audit.RequestIDFromContext(c),
		ActorID:    actorID,
		ActorType:  actorType,
		EventType:  "refund.reviewed",
		ResourceID: refund.ID,
		Metadata: map[string]any{
			"to_status":  string(refund.Status),
			"result":     "SUCCESS",
			"journey_id": refund.ID,
		},
	})
	response.OK(c, refund)
}

// ProcessRefund godoc
// @Summary Process refund
// @Description Admin processes approved refund to SUCCESS or FAILED
// @Tags refund
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Refund ID"
// @Param request body ProcessRefundRequest true "Refund process payload"
// @Success 200 {object} response.Envelope{data=handlers.RefundProcessResponse}
// @Failure 400 {object} response.Envelope{error=response.ErrorPayload}
// @Failure 401 {object} response.Envelope{error=response.ErrorPayload}
// @Failure 403 {object} response.Envelope{error=response.ErrorPayload}
// @Router /admin/refunds/{id}/process [patch]
func (h *RefundHandler) ProcessRefund(c *gin.Context) {
	var req ProcessRefundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, appErrors.BadRequest("validation_error", "invalid request payload", err.Error()))
		return
	}
	refund, merchant, err := h.service.ProcessRefund(c.Param("id"), req.Status)
	if err != nil {
		actorID, actorType := audit.ActorFromContext(c)
		audit.LogBestEffort(c, h.auditLogger, audit.Event{
			RequestID:  audit.RequestIDFromContext(c),
			ActorID:    actorID,
			ActorType:  actorType,
			EventType:  "refund.processed",
			ResourceID: c.Param("id"),
			Metadata: map[string]any{
				"to_status":     req.Status,
				"result":        "FAILED",
				"error_code":    "refund_process_failed",
				"error_message": err.Error(),
				"journey_id":    c.Param("id"),
			},
		})
		response.Fail(c, appErrors.BadRequest("refund_process_failed", err.Error(), nil))
		return
	}

	actorID, actorType := audit.ActorFromContext(c)
	audit.LogBestEffort(c, h.auditLogger, audit.Event{
		RequestID:  audit.RequestIDFromContext(c),
		ActorID:    actorID,
		ActorType:  actorType,
		EventType:  "refund.processed",
		ResourceID: refund.ID,
		Metadata: map[string]any{
			"merchant_id": merchant.ID,
			"to_status":   string(refund.Status),
			"result":      "SUCCESS",
			"journey_id":  refund.ID,
		},
	})
	response.OK(c, gin.H{"refund": refund, "merchant": merchant})
}
