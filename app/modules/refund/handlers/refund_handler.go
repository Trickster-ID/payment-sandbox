package handlers

import (
	"payment-sandbox/app/middleware"
	refundEntity "payment-sandbox/app/modules/refund/models/entity"
	"payment-sandbox/app/modules/refund/services"
	walletEntity "payment-sandbox/app/modules/wallet/models/entity"
	appErrors "payment-sandbox/app/shared/errors"
	"payment-sandbox/app/shared/journeylog"
	"payment-sandbox/app/shared/response"

	"github.com/gin-gonic/gin"
)

type RefundHandler struct {
	service       services.IRefundService
	journeyLogger journeylog.IJourneyLogger
}

func NewRefundHandler(service services.IRefundService, journeyLogger journeylog.IJourneyLogger) *RefundHandler {
	return &RefundHandler{service: service, journeyLogger: journeyLogger}
}

type CreateRefundRequest struct {
	PaymentIntentID string `json:"payment_intent_id" binding:"required" example:"0196aee7-80b0-7d57-b38f-26b315d8f9bb"`
	Reason          string `json:"reason" binding:"required" example:"Customer requested cancellation"`
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
// @Description Merchant requests refund for successful payment intent
// @Tags refund
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateRefundRequest true "Refund request payload"
// @Success 201 {object} response.Envelope{data=handlers.RefundResponse}
// @Failure 400 {object} response.Envelope{error=response.ErrorPayload}
// @Failure 401 {object} response.Envelope{error=response.ErrorPayload}
// @Failure 403 {object} response.Envelope{error=response.ErrorPayload}
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

	refund, err := h.service.RequestRefund(userID, req.PaymentIntentID, req.Reason)
	if err != nil {
		actorID, actorRole := journeylog.ActorFromContext(c)
		journeylog.LogBestEffort(c, h.journeyLogger, journeylog.Event{
			RequestID:    journeylog.RequestIDFromContext(c),
			ActorID:      actorID,
			ActorRole:    actorRole,
			Module:       "refund",
			EntityType:   "refund",
			Action:       "REFUND_REQUEST",
			Result:       "FAILED",
			ErrorCode:    "refund_request_failed",
			ErrorMessage: err.Error(),
		})
		response.Fail(c, appErrors.BadRequest("refund_request_failed", err.Error(), nil))
		return
	}

	actorID, actorRole := journeylog.ActorFromContext(c)
	journeylog.LogBestEffort(c, h.journeyLogger, journeylog.Event{
		RequestID:  journeylog.RequestIDFromContext(c),
		JourneyID:  refund.ID,
		ActorID:    actorID,
		ActorRole:  actorRole,
		Module:     "refund",
		EntityType: "refund",
		EntityID:   refund.ID,
		Action:     "REFUND_REQUEST",
		ToStatus:   string(refund.Status),
		Result:     "SUCCESS",
		Metadata: map[string]any{
			"payment_intent_id": refund.PaymentIntentID,
		},
	})
	response.Created(c, refund)
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
		actorID, actorRole := journeylog.ActorFromContext(c)
		journeylog.LogBestEffort(c, h.journeyLogger, journeylog.Event{
			RequestID:    journeylog.RequestIDFromContext(c),
			JourneyID:    c.Param("id"),
			ActorID:      actorID,
			ActorRole:    actorRole,
			Module:       "refund",
			EntityType:   "refund",
			EntityID:     c.Param("id"),
			Action:       "REFUND_REVIEW",
			ToStatus:     req.Decision,
			Result:       "FAILED",
			ErrorCode:    "refund_review_failed",
			ErrorMessage: err.Error(),
		})
		response.Fail(c, appErrors.BadRequest("refund_review_failed", err.Error(), nil))
		return
	}

	actorID, actorRole := journeylog.ActorFromContext(c)
	journeylog.LogBestEffort(c, h.journeyLogger, journeylog.Event{
		RequestID:  journeylog.RequestIDFromContext(c),
		JourneyID:  refund.ID,
		ActorID:    actorID,
		ActorRole:  actorRole,
		Module:     "refund",
		EntityType: "refund",
		EntityID:   refund.ID,
		Action:     "REFUND_REVIEW",
		ToStatus:   string(refund.Status),
		Result:     "SUCCESS",
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
		actorID, actorRole := journeylog.ActorFromContext(c)
		journeylog.LogBestEffort(c, h.journeyLogger, journeylog.Event{
			RequestID:    journeylog.RequestIDFromContext(c),
			JourneyID:    c.Param("id"),
			ActorID:      actorID,
			ActorRole:    actorRole,
			Module:       "refund",
			EntityType:   "refund",
			EntityID:     c.Param("id"),
			Action:       "REFUND_PROCESS",
			ToStatus:     req.Status,
			Result:       "FAILED",
			ErrorCode:    "refund_process_failed",
			ErrorMessage: err.Error(),
		})
		response.Fail(c, appErrors.BadRequest("refund_process_failed", err.Error(), nil))
		return
	}

	actorID, actorRole := journeylog.ActorFromContext(c)
	journeylog.LogBestEffort(c, h.journeyLogger, journeylog.Event{
		RequestID:  journeylog.RequestIDFromContext(c),
		JourneyID:  refund.ID,
		ActorID:    actorID,
		ActorRole:  actorRole,
		Module:     "refund",
		EntityType: "refund",
		EntityID:   refund.ID,
		Action:     "REFUND_PROCESS",
		ToStatus:   string(refund.Status),
		Result:     "SUCCESS",
		Metadata: map[string]any{
			"merchant_id": merchant.ID,
		},
	})
	response.OK(c, gin.H{"refund": refund, "merchant": merchant})
}
