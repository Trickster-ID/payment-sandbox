package handlers

import (
	userServices "payment-sandbox/app/modules/users/services"
	appErrors "payment-sandbox/app/shared/errors"
	"payment-sandbox/app/shared/response"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	service userServices.IUserService
}

func NewUserHandler(service userServices.IUserService) *UserHandler {
	return &UserHandler{service: service}
}

type RegisterRequest struct {
	Name     string `json:"name" binding:"required" example:"Jane Merchant"`
	Email    string `json:"email" binding:"required,email" example:"jane.merchant@example.com"`
	Password string `json:"password" binding:"required" example:"merchant1234"`
}

type UserResponse struct {
	ID    string `json:"id" example:"0196aee7-7eca-7e8c-96fb-4fdfa75b2177"`
	Name  string `json:"name" example:"Jane Merchant"`
	Email string `json:"email" example:"jane.merchant@example.com"`
	Role  string `json:"role" example:"MERCHANT" enums:"MERCHANT,ADMIN"`
}

// RegisterMerchant godoc
// @Summary Register merchant
// @Description Register a new merchant account
// @Tags users
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "Register payload"
// @Success 201 {object} response.Envelope{data=handlers.UserResponse}
// @Failure 400 {object} response.Envelope{error=response.ErrorPayload}
// @Router /users/register [post]
func (h *UserHandler) RegisterMerchant(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, appErrors.BadRequest("validation_error", "invalid request payload", err.Error()))
		return
	}

	user, err := h.service.RegisterMerchant(req.Name, req.Email, req.Password)
	if err != nil {
		response.Fail(c, appErrors.BadRequest("validation_error", err.Error(), nil))
		return
	}

	response.Created(c, gin.H{
		"id":    user.ID,
		"name":  user.Name,
		"email": user.Email,
		"role":  user.Role,
	})
}
