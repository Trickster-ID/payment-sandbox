package handlers

import (
	"net/http"
	"net/url"
	"payment-sandbox/app/middleware"
	"payment-sandbox/app/modules/oauth2/models/dto"
	"payment-sandbox/app/modules/oauth2/services"
	appErrors "payment-sandbox/app/shared/errors"
	"payment-sandbox/app/shared/response"
	"strings"

	"github.com/gin-gonic/gin"
)

type OAuth2Handler struct {
	service services.IOAuth2Service
}

func NewOAuth2Handler(service services.IOAuth2Service) *OAuth2Handler {
	return &OAuth2Handler{service: service}
}

// Client Management

// RegisterClient godoc
// @Summary Register OAuth2 client
// @Description Register a new OAuth2 client for the merchant
// @Tags oauth2
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.RegisterClientRequest true "Register payload"
// @Success 201 {object} response.Envelope{data=dto.RegisterClientResponse}
// @Failure 400 {object} response.Envelope{error=response.ErrorPayload}
// @Router /merchant/clients [post]
func (h *OAuth2Handler) RegisterClient(c *gin.Context) {
	var req dto.RegisterClientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, appErrors.BadRequest("validation_error", err.Error(), nil))
		return
	}

	userID, _ := middleware.MustUserID(c)

	result, err := h.service.RegisterClient(userID, req.Name, req.RedirectURIs, req.Scopes)
	if err != nil {
		response.Fail(c, appErrors.Internal("registration_error", err.Error(), nil))
		return
	}

	response.Created(c, dto.RegisterClientResponse{
		Client: dto.ClientResponse{
			ID:             result.Client.ID,
			Name:           result.Client.Name,
			RedirectURIs:   result.Client.RedirectURIs,
			Scopes:         result.Client.Scopes,
			IsFirstParty:   result.Client.IsFirstParty,
			IsConfidential: result.Client.IsConfidential,
			CreatedAt:      result.Client.CreatedAt,
		},
		ClientSecret: result.ClientSecret,
	})
}

// ListClients godoc
// @Summary List OAuth2 clients
// @Description List all OAuth2 clients owned by the merchant
// @Tags oauth2
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Envelope{data=[]dto.ClientResponse}
// @Router /merchant/clients [get]
func (h *OAuth2Handler) ListClients(c *gin.Context) {
	userID, _ := middleware.MustUserID(c)

	clients, err := h.service.ListClients(userID)
	if err != nil {
		response.Fail(c, appErrors.Internal("list_error", err.Error(), nil))
		return
	}

	var res []dto.ClientResponse
	for _, client := range clients {
		res = append(res, dto.ClientResponse{
			ID:             client.ID,
			Name:           client.Name,
			RedirectURIs:   client.RedirectURIs,
			Scopes:         client.Scopes,
			IsFirstParty:   client.IsFirstParty,
			IsConfidential: client.IsConfidential,
			CreatedAt:      client.CreatedAt,
		})
	}

	response.OK(c, res)
}

func (h *OAuth2Handler) DeleteClient(c *gin.Context) {
	clientID := c.Param("id")
	userID, _ := middleware.MustUserID(c)

	if err := h.service.DeleteClient(clientID, userID); err != nil {
		response.Fail(c, appErrors.Internal("delete_error", err.Error(), nil))
		return
	}

	response.OK(c, gin.H{"status": "deleted"})
}

// OAuth2 Protocol

// Authorize godoc
// @Summary Authorize client
// @Description Authorization endpoint (RFC 6749 Section 4.1.1)
// @Tags oauth2
// @Param response_type query string true "Response type (code)"
// @Param client_id query string true "Client ID"
// @Param redirect_uri query string true "Redirect URI"
// @Param scope query string false "Scopes"
// @Param state query string false "State"
// @Success 302 "Redirect to client callback"
// @Router /oauth2/authorize [get]
func (h *OAuth2Handler) Authorize(c *gin.Context) {
	var req dto.AuthorizeRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, appErrors.BadRequest("invalid_request", err.Error(), nil))
		return
	}

	// 1. Validate Client
	_, err := h.service.GetClient(req.ClientID)
	if err != nil {
		response.Fail(c, appErrors.BadRequest("invalid_client", "client not found", nil))
		return
	}

	// 2. Ensure User is Logged In
	userID, ok := middleware.MustUserID(c)
	if !ok {
		// In a real OAuth server, we would redirect to login page with 'continue' URL.
		// For this sandbox, the middleware already handled the failure response if not logged in.
		return
	}

	// 3. Issue Auth Code
	code, err := h.service.IssueAuthCode(req.ClientID, userID, req.RedirectURI, req.Scope)
	if err != nil {
		response.Fail(c, appErrors.Internal("auth_code_error", "failed to issue auth code", nil))
		return
	}

	// 4. Redirect back with code
	target, _ := url.Parse(req.RedirectURI)
	q := target.Query()
	q.Set("code", code)
	if req.State != "" {
		q.Set("state", req.State)
	}
	target.RawQuery = q.Encode()

	c.Redirect(http.StatusFound, target.String())
}

func (h *OAuth2Handler) ApproveAuthorize(c *gin.Context) {
	var req dto.AuthorizeRequest // Use same DTO for simplicity
	if err := c.ShouldBind(&req); err != nil {
		response.Fail(c, appErrors.BadRequest("invalid_request", err.Error(), nil))
		return
	}

	userID, _ := middleware.MustUserID(c)

	code, err := h.service.IssueAuthCode(req.ClientID, userID, req.RedirectURI, req.Scope)
	if err != nil {
		response.Fail(c, appErrors.Internal("auth_code_error", "failed to issue auth code", nil))
		return
	}

	target, _ := url.Parse(req.RedirectURI)
	q := target.Query()
	q.Set("code", code)
	if req.State != "" {
		q.Set("state", req.State)
	}
	target.RawQuery = q.Encode()

	response.OK(c, gin.H{"redirect_uri": target.String()})
}

// Token godoc
// @Summary Issue token
// @Description Token endpoint (RFC 6749 Section 3.2)
// @Tags oauth2
// @Accept x-www-form-urlencoded
// @Produce json
// @Param grant_type formData string true "Grant type" Enums(authorization_code, client_credentials, refresh_token, password)
// @Param code formData string false "Auth code"
// @Param redirect_uri formData string false "Redirect URI"
// @Param client_id formData string false "Client ID"
// @Param client_secret formData string false "Client Secret"
// @Param refresh_token formData string false "Refresh token"
// @Param username formData string false "Username (password grant)"
// @Param password formData string false "Password (password grant)"
// @Param scope formData string false "Scopes"
// @Success 200 {object} response.Envelope{data=dto.TokenResponse}
// @Router /oauth2/token [post]
func (h *OAuth2Handler) Token(c *gin.Context) {
	var req dto.TokenRequest
	if err := c.ShouldBind(&req); err != nil {
		response.Fail(c, appErrors.BadRequest("invalid_request", err.Error(), nil))
		return
	}

	switch req.GrantType {
	case "authorization_code":
		h.handleAuthCodeGrant(c, req)
	case "client_credentials":
		h.handleClientCredentialsGrant(c, req)
	case "refresh_token":
		h.handleRefreshTokenGrant(c, req)
	case "password":
		h.handlePasswordGrant(c, req)
	default:
		response.Fail(c, appErrors.BadRequest("unsupported_grant_type", "grant_type not supported", nil))
	}
}

func (h *OAuth2Handler) handlePasswordGrant(c *gin.Context, req dto.TokenRequest) {
	// 1. Validate Client
	client, err := h.service.ValidateClient(req.ClientID, req.ClientSecret)
	if err != nil {
		response.Fail(c, appErrors.Unauthorized("invalid_client", "client authentication failed", nil))
		return
	}

	// 2. Validate User Credentials
	user, err := h.service.ValidateUserCredentials(req.Username, req.Password)
	if err != nil {
		response.Fail(c, appErrors.Unauthorized("invalid_grant", "invalid user credentials", nil))
		return
	}

	// 3. Issue Tokens
	scope := req.Scope
	if scope == "" {
		scope = strings.Join(client.Scopes, " ")
	}

	accessToken, _ := h.service.IssueAccessToken(client.ID, user.ID, scope, user.Role)
	refreshToken, _ := h.service.IssueRefreshToken(client.ID, user.ID, scope)

	response.OK(c, dto.TokenResponse{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    3600,
		RefreshToken: refreshToken,
		Scope:        scope,
	})
}

func (h *OAuth2Handler) handleAuthCodeGrant(c *gin.Context, req dto.TokenRequest) {
	// 1. Validate Client
	client, err := h.service.ValidateClient(req.ClientID, req.ClientSecret)
	if err != nil {
		response.Fail(c, appErrors.Unauthorized("invalid_client", "client authentication failed", nil))
		return
	}

	// 2. Exchange Code
	authCode, err := h.service.ExchangeAuthCode(req.Code, req.ClientID, req.RedirectURI)
	if err != nil {
		response.Fail(c, appErrors.BadRequest("invalid_grant", err.Error(), nil))
		return
	}

	// 3. Fetch User for Role
	user, err := h.service.GetUserByID(authCode.UserID)
	if err != nil {
		response.Fail(c, appErrors.Internal("user_lookup_error", "failed to fetch user", nil))
		return
	}

	// 4. Issue Tokens
	accessToken, _ := h.service.IssueAccessToken(client.ID, authCode.UserID, authCode.Scope, user.Role)
	refreshToken, _ := h.service.IssueRefreshToken(client.ID, authCode.UserID, authCode.Scope)

	response.OK(c, dto.TokenResponse{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    3600,
		RefreshToken: refreshToken,
		Scope:        authCode.Scope,
	})
}

func (h *OAuth2Handler) handleClientCredentialsGrant(c *gin.Context, req dto.TokenRequest) {
	client, err := h.service.ValidateClient(req.ClientID, req.ClientSecret)
	if err != nil {
		response.Fail(c, appErrors.Unauthorized("invalid_client", "client authentication failed", nil))
		return
	}

	// Client credentials grant uses client's own scopes or requested subset
	scope := req.Scope
	if scope == "" {
		scope = strings.Join(client.Scopes, " ")
	}

	accessToken, _ := h.service.IssueAccessToken(client.ID, "", scope, "") // No user_id for CC grant

	response.OK(c, dto.TokenResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   3600,
		Scope:       scope,
	})
}

func (h *OAuth2Handler) handleRefreshTokenGrant(c *gin.Context, req dto.TokenRequest) {
	client, err := h.service.ValidateClient(req.ClientID, req.ClientSecret)
	if err != nil {
		response.Fail(c, appErrors.Unauthorized("invalid_client", "client authentication failed", nil))
		return
	}

	oldToken, err := h.service.ExchangeRefreshToken(req.RefreshToken, client.ID)
	if err != nil {
		response.Fail(c, appErrors.BadRequest("invalid_grant", err.Error(), nil))
		return
	}

	user, err := h.service.GetUserByID(oldToken.UserID)
	if err != nil {
		response.Fail(c, appErrors.Internal("user_lookup_error", "failed to fetch user", nil))
		return
	}

	accessToken, _ := h.service.IssueAccessToken(client.ID, oldToken.UserID, oldToken.Scope, user.Role)
	refreshToken, _ := h.service.IssueRefreshToken(client.ID, oldToken.UserID, oldToken.Scope)

	response.OK(c, dto.TokenResponse{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    3600,
		RefreshToken: refreshToken,
		Scope:        oldToken.Scope,
	})
}

// Introspect godoc
// @Summary Introspect token
// @Description Introspection endpoint (RFC 7662)
// @Tags oauth2
// @Accept x-www-form-urlencoded
// @Produce json
// @Param token formData string true "Token to introspect"
// @Success 200 {object} response.Envelope{data=dto.IntrospectResponse}
// @Router /oauth2/introspect [post]
func (h *OAuth2Handler) Introspect(c *gin.Context) {
	var req dto.IntrospectRequest
	if err := c.ShouldBind(&req); err != nil {
		response.Fail(c, appErrors.BadRequest("invalid_request", err.Error(), nil))
		return
	}

	claims, err := h.service.ValidateToken(req.Token)
	if err != nil {
		response.OK(c, dto.IntrospectResponse{Active: false})
		return
	}

	res := dto.IntrospectResponse{
		Active:   true,
		Scope:    claims.Scope,
		ClientID: claims.ClientID,
		UserID:   claims.UserID,
	}
	if claims.ExpiresAt != nil {
		res.ExpiresAt = claims.ExpiresAt.Unix()
	}

	response.OK(c, res)
}

// Revoke godoc
// @Summary Revoke token
// @Description Token revocation endpoint (RFC 7009)
// @Tags oauth2
// @Accept x-www-form-urlencoded
// @Produce json
// @Param token formData string true "Token to revoke"
// @Param client_id formData string true "Client ID"
// @Param client_secret formData string true "Client Secret"
// @Success 200 {object} response.Envelope{data=map[string]string}
// @Router /oauth2/revoke [post]
func (h *OAuth2Handler) Revoke(c *gin.Context) {
	var req dto.RevokeRequest
	if err := c.ShouldBind(&req); err != nil {
		response.Fail(c, appErrors.BadRequest("invalid_request", err.Error(), nil))
		return
	}

	// Validate Client
	client, err := h.service.ValidateClient(req.ClientID, req.ClientSecret)
	if err != nil {
		response.Fail(c, appErrors.Unauthorized("invalid_client", "client authentication failed", nil))
		return
	}

	if err := h.service.RevokeRefreshToken(req.Token, client.ID); err != nil {
		response.Fail(c, appErrors.Internal("revoke_error", err.Error(), nil))
		return
	}

	response.OK(c, gin.H{"status": "revoked"})
}

// UserInfo godoc
// @Summary User information
// @Description UserInfo endpoint
// @Tags oauth2
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Envelope{data=map[string]string}
// @Router /oauth2/userinfo [get]
func (h *OAuth2Handler) UserInfo(c *gin.Context) {
	userID, ok := middleware.MustUserID(c)
	if !ok {
		response.Fail(c, appErrors.Unauthorized("auth_required", "authentication required", nil))
		return
	}
	
	response.OK(c, gin.H{
		"sub": userID,
	})
}
