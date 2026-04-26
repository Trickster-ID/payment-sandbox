package handlers

import (
	"payment-sandbox/app/middleware"
	walletServices "payment-sandbox/app/modules/wallet/services"
	appErrors "payment-sandbox/app/shared/errors"
	"payment-sandbox/app/shared/journeylog"
	"payment-sandbox/app/shared/response"

	"github.com/gin-gonic/gin"
)

type WalletHandler struct {
	service       walletServices.IWalletService
	journeyLogger journeylog.IJourneyLogger
}

func NewWalletHandler(service walletServices.IWalletService, journeyLogger journeylog.IJourneyLogger) *WalletHandler {
	return &WalletHandler{service: service, journeyLogger: journeyLogger}
}

type CreateTopupRequest struct {
	Amount float64 `json:"amount" binding:"required,gt=0"`
}

type UpdateTopupStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

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
		actorID, actorRole := journeylog.ActorFromContext(c)
		journeylog.LogBestEffort(c, h.journeyLogger, journeylog.Event{
			RequestID:    journeylog.RequestIDFromContext(c),
			ActorID:      actorID,
			ActorRole:    actorRole,
			Module:       "wallet",
			EntityType:   "topup",
			Action:       "TOPUP_CREATE",
			Result:       "FAILED",
			ErrorCode:    "topup_create_failed",
			ErrorMessage: err.Error(),
			Metadata: map[string]any{
				"amount": req.Amount,
			},
		})
		response.Fail(c, appErrors.BadRequest("topup_create_failed", err.Error(), nil))
		return
	}

	actorID, actorRole := journeylog.ActorFromContext(c)
	journeylog.LogBestEffort(c, h.journeyLogger, journeylog.Event{
		RequestID:  journeylog.RequestIDFromContext(c),
		JourneyID:  topup.ID,
		ActorID:    actorID,
		ActorRole:  actorRole,
		Module:     "wallet",
		EntityType: "topup",
		EntityID:   topup.ID,
		Action:     "TOPUP_CREATE",
		ToStatus:   string(topup.Status),
		Result:     "SUCCESS",
		Metadata: map[string]any{
			"amount": topup.Amount,
		},
	})
	response.Created(c, topup)
}

func (h *WalletHandler) ListTopups(c *gin.Context) {
	response.OK(c, h.service.ListTopups())
}

func (h *WalletHandler) UpdateTopupStatus(c *gin.Context) {
	var req UpdateTopupStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, appErrors.BadRequest("validation_error", "invalid request payload", err.Error()))
		return
	}
	topup, err := h.service.UpdateTopupStatus(c.Param("id"), req.Status)
	if err != nil {
		actorID, actorRole := journeylog.ActorFromContext(c)
		journeylog.LogBestEffort(c, h.journeyLogger, journeylog.Event{
			RequestID:    journeylog.RequestIDFromContext(c),
			JourneyID:    c.Param("id"),
			ActorID:      actorID,
			ActorRole:    actorRole,
			Module:       "wallet",
			EntityType:   "topup",
			EntityID:     c.Param("id"),
			Action:       "TOPUP_STATUS_UPDATE",
			ToStatus:     req.Status,
			Result:       "FAILED",
			ErrorCode:    "topup_update_failed",
			ErrorMessage: err.Error(),
		})
		response.Fail(c, appErrors.BadRequest("topup_update_failed", err.Error(), nil))
		return
	}

	actorID, actorRole := journeylog.ActorFromContext(c)
	journeylog.LogBestEffort(c, h.journeyLogger, journeylog.Event{
		RequestID:  journeylog.RequestIDFromContext(c),
		JourneyID:  topup.ID,
		ActorID:    actorID,
		ActorRole:  actorRole,
		Module:     "wallet",
		EntityType: "topup",
		EntityID:   topup.ID,
		Action:     "TOPUP_STATUS_UPDATE",
		ToStatus:   string(topup.Status),
		Result:     "SUCCESS",
	})
	response.OK(c, topup)
}
