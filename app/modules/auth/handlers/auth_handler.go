package handlers

import (
	authServices "payment-sandbox/app/modules/auth/services"
	appErrors "payment-sandbox/app/shared/errors"
	"payment-sandbox/app/shared/response"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	service authServices.IAuthService
}

func NewAuthHandler(service authServices.IAuthService) *AuthHandler {
	return &AuthHandler{service: service}
}

type RegisterRequest struct {
	Name     string `json:"name" binding:"required" example:"Jane Merchant"`
	Email    string `json:"email" binding:"required,email" example:"jane.merchant@example.com"`
	Password string `json:"password" binding:"required" example:"merchant1234"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email" example:"jane.merchant@example.com"`
	Password string `json:"password" binding:"required" example:"merchant1234"`
}

type AuthUserResponse struct {
	ID    string `json:"id" example:"0196aee7-7eca-7e8c-96fb-4fdfa75b2177"`
	Name  string `json:"name" example:"Jane Merchant"`
	Email string `json:"email" example:"jane.merchant@example.com"`
	Role  string `json:"role" example:"MERCHANT" enums:"MERCHANT,ADMIN"`
}

type RegisterMerchantResponse struct {
	ID    string `json:"id" example:"0196aee7-7eca-7e8c-96fb-4fdfa75b2177"`
	Name  string `json:"name" example:"Jane Merchant"`
	Email string `json:"email" example:"jane.merchant@example.com"`
	Role  string `json:"role" example:"MERCHANT" enums:"MERCHANT,ADMIN"`
}

type LoginResponse struct {
	AccessToken string           `json:"access_token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	User        AuthUserResponse `json:"user"`
}

// RegisterMerchant godoc
// @Summary Register merchant
// @Description Register a new merchant account
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "Register payload"
// @Success 201 {object} response.Envelope{data=handlers.RegisterMerchantResponse}
// @Failure 400 {object} response.Envelope{error=response.ErrorPayload}
// @Router /auth/register [post]
func (h *AuthHandler) RegisterMerchant(c *gin.Context) {
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

// Login godoc
// @Summary Login user
// @Description Login with email and password
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login payload"
// @Success 200 {object} response.Envelope{data=handlers.LoginResponse}
// @Failure 400 {object} response.Envelope{error=response.ErrorPayload}
// @Failure 401 {object} response.Envelope{error=response.ErrorPayload}
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, appErrors.BadRequest("validation_error", "invalid request payload", err.Error()))
		return
	}

	token, user, err := h.service.Login(req.Email, req.Password)
	if err != nil {
		response.Fail(c, appErrors.Unauthorized("auth_invalid_credentials", err.Error(), nil))
		return
	}

	c.Header("Warning", `299 - "This endpoint is deprecated. Please migrate to /oauth2/token"`)
	c.Header("X-Deprecation", "true")
	response.OK(c, gin.H{
		"access_token": token,
		"user": gin.H{
			"id":    user.ID,
			"name":  user.Name,
			"email": user.Email,
			"role":  user.Role,
		},
	})
}
