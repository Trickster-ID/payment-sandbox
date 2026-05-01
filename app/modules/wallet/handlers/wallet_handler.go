package handlers

import (
	"payment-sandbox/app/middleware"
	walletEntity "payment-sandbox/app/modules/wallet/models/entity"
	walletServices "payment-sandbox/app/modules/wallet/services"
	"payment-sandbox/app/shared/audit"
	appErrors "payment-sandbox/app/shared/errors"
	"payment-sandbox/app/shared/response"

	"github.com/gin-gonic/gin"
)

type WalletHandler struct {
	service      walletServices.IWalletService
	auditLogger  audit.IAuditLogger
}

func NewWalletHandler(service walletServices.IWalletService, auditLogger audit.IAuditLogger) *WalletHandler {
	return &WalletHandler{service: service, auditLogger: auditLogger}
}

type CreateTopupRequest struct {
	Amount int64 `json:"amount" binding:"required,gt=0" example:"500000"`
}

type UpdateTopupStatusRequest struct {
	Status string `json:"status" binding:"required" example:"SUCCESS" enums:"SUCCESS,FAILED"`
}

type WalletResponse = walletEntity.Merchant

type TopupResponse = walletEntity.Topup

type TopupListResponse []walletEntity.Topup

// Wallet godoc
// @Summary Get merchant wallet
// @Description Merchant gets current wallet state
// @Tags wallet
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Envelope{data=handlers.WalletResponse}
// @Failure 401 {object} response.Envelope{error=response.ErrorPayload}
// @Failure 403 {object} response.Envelope{error=response.ErrorPayload}
// @Failure 404 {object} response.Envelope{error=response.ErrorPayload}
// @Router /merchant/wallet [get]
func (h *WalletHandler) Wallet(c *gin.Context) {
	userID, ok := middleware.MustUserID(c)
	if !ok {
		return
	}

	wallet, err := h.service.WalletByUserID(userID)
	if err != nil {
		response.Fail(c, appErrors.NotFound("wallet_not_found", err.Error(), nil))
		return
	}
	response.OK(c, wallet)
}

// CreateTopup godoc
// @Summary Create top-up request
// @Description Merchant creates top-up request with pending status. Requires Idempotency-Key header to safely retry on network failure.
// @Tags wallet
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Idempotency-Key header string true "Unique key per logical request (UUID recommended). Replaying the same key returns the original response; reusing the key with a different body returns 409."
// @Param request body CreateTopupRequest true "Create top-up payload"
// @Success 201 {object} response.Envelope{data=handlers.TopupResponse}
// @Failure 400 {object} response.Envelope{error=response.ErrorPayload} "validation_error or idempotency_key_required"
// @Failure 401 {object} response.Envelope{error=response.ErrorPayload}
// @Failure 403 {object} response.Envelope{error=response.ErrorPayload}
// @Failure 409 {object} response.Envelope{error=response.ErrorPayload} "idempotency_key_conflict or idempotency_in_progress"
// @Router /merchant/topups [post]
func (h *WalletHandler) CreateTopup(c *gin.Context) {
	userID, ok := middleware.MustUserID(c)
	if !ok {
		return
	}

	var req CreateTopupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, appErrors.BadRequest("validation_error", "invalid request payload", err.Error()))
		return
	}

	topup, err := h.service.CreateTopup(userID, req.Amount)
	if err != nil {
		actorID, actorType := audit.ActorFromContext(c)
		audit.LogBestEffort(c, h.auditLogger, audit.Event{
			RequestID: audit.RequestIDFromContext(c),
			ActorID:   actorID,
			ActorType: actorType,
			EventType: "topup.created",
			Metadata: map[string]any{
				"amount":        req.Amount,
				"result":        "FAILED",
				"error_code":    "topup_create_failed",
				"error_message": err.Error(),
			},
		})
		response.Fail(c, appErrors.BadRequest("topup_create_failed", err.Error(), nil))
		return
	}

	actorID, actorType := audit.ActorFromContext(c)
	audit.LogBestEffort(c, h.auditLogger, audit.Event{
		RequestID:  audit.RequestIDFromContext(c),
		ActorID:    actorID,
		ActorType:  actorType,
		EventType:  "topup.created",
		ResourceID: topup.ID,
		Metadata: map[string]any{
			"amount":     topup.Amount,
			"to_status":  string(topup.Status),
			"result":     "SUCCESS",
			"journey_id": topup.ID,
		},
	})
	response.Created(c, topup)
}

// ListTopups godoc
// @Summary List top-ups
// @Description Admin lists top-up requests
// @Tags wallet
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Envelope{data=handlers.TopupListResponse}
// @Failure 401 {object} response.Envelope{error=response.ErrorPayload}
// @Failure 403 {object} response.Envelope{error=response.ErrorPayload}
// @Router /admin/topups [get]
func (h *WalletHandler) ListTopups(c *gin.Context) {
	response.OK(c, h.service.ListTopups())
}

// UpdateTopupStatus godoc
// @Summary Update top-up status
// @Description Admin updates top-up status to SUCCESS or FAILED
// @Tags wallet
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Top-up ID"
// @Param request body UpdateTopupStatusRequest true "Top-up status payload"
// @Success 200 {object} response.Envelope{data=handlers.TopupResponse}
// @Failure 400 {object} response.Envelope{error=response.ErrorPayload}
// @Failure 401 {object} response.Envelope{error=response.ErrorPayload}
// @Failure 403 {object} response.Envelope{error=response.ErrorPayload}
// @Router /admin/topups/{id}/status [patch]
func (h *WalletHandler) UpdateTopupStatus(c *gin.Context) {
	var req UpdateTopupStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, appErrors.BadRequest("validation_error", "invalid request payload", err.Error()))
		return
	}
	topup, err := h.service.UpdateTopupStatus(c.Param("id"), req.Status)
	if err != nil {
		actorID, actorType := audit.ActorFromContext(c)
		audit.LogBestEffort(c, h.auditLogger, audit.Event{
			RequestID:  audit.RequestIDFromContext(c),
			ActorID:    actorID,
			ActorType:  actorType,
			EventType:  "topup.status_updated",
			ResourceID: c.Param("id"),
			Metadata: map[string]any{
				"to_status":     req.Status,
				"result":        "FAILED",
				"error_code":    "topup_update_failed",
				"error_message": err.Error(),
				"journey_id":    c.Param("id"),
			},
		})
		response.Fail(c, appErrors.BadRequest("topup_update_failed", err.Error(), nil))
		return
	}

	actorID, actorType := audit.ActorFromContext(c)
	audit.LogBestEffort(c, h.auditLogger, audit.Event{
		RequestID:  audit.RequestIDFromContext(c),
		ActorID:    actorID,
		ActorType:  actorType,
		EventType:  "topup.status_updated",
		ResourceID: topup.ID,
		Metadata: map[string]any{
			"to_status":  string(topup.Status),
			"result":     "SUCCESS",
			"journey_id": topup.ID,
		},
	})
	response.OK(c, topup)
}
