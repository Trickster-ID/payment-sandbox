package handlers

import (
	"net/http"

	"payment-sandbox/app/modules/ledger/repositories"
	appErrors "payment-sandbox/app/shared/errors"
	"payment-sandbox/app/shared/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type LedgerHandler struct {
	repo repositories.IRepository
}

func NewLedgerHandler(repo repositories.IRepository) *LedgerHandler {
	return &LedgerHandler{repo: repo}
}

// GetMerchantAccount godoc
// @Summary Get merchant ledger account
// @Description Admin fetches the wallet account for a merchant
// @Tags ledger
// @Produce json
// @Security BearerAuth
// @Param merchant_id path string true "Merchant UUID"
// @Success 200 {object} response.Envelope{}
// @Failure 400 {object} response.Envelope{error=response.ErrorPayload}
// @Failure 401 {object} response.Envelope{error=response.ErrorPayload}
// @Failure 403 {object} response.Envelope{error=response.ErrorPayload}
// @Router /admin/ledger/accounts/{merchant_id} [get]
func (h *LedgerHandler) GetMerchantAccount(c *gin.Context) {
	merchantID, err := uuid.Parse(c.Param("merchant_id"))
	if err != nil {
		response.Fail(c, appErrors.BadRequest("invalid_merchant_id", "merchant_id must be a valid UUID", nil))
		return
	}
	account, err := h.repo.GetAccountByMerchantID(c.Request.Context(), merchantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
		return
	}
	response.OK(c, account)
}
