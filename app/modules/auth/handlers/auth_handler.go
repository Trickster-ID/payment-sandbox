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
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// RegisterMerchant godoc
// @Summary Register merchant
// @Description Register a new merchant account
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "Register payload"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
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
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
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
