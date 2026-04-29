package dto

import "time"

// Client management DTOs
type RegisterClientRequest struct {
	Name         string   `json:"name" binding:"required,max=255"`
	RedirectURIs []string `json:"redirect_uris" binding:"required,gt=0,dive,url"`
	Scopes       []string `json:"scopes" binding:"required,gt=0,dive,oneof=read write admin"`
}

type ClientResponse struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	RedirectURIs   []string  `json:"redirect_uris"`
	Scopes         []string  `json:"scopes"`
	IsFirstParty   bool      `json:"is_first_party"`
	IsConfidential bool      `json:"is_confidential"`
	CreatedAt      time.Time `json:"created_at"`
}

type RegisterClientResponse struct {
	Client       ClientResponse `json:"client"`
	ClientSecret string         `json:"client_secret"`
}

// OAuth2 protocol DTOs
type AuthorizeRequest struct {
	ResponseType string `form:"response_type" binding:"required,oneof=code"`
	ClientID     string `form:"client_id" binding:"required"`
	RedirectURI  string `form:"redirect_uri" binding:"required,url"`
	Scope        string `form:"scope"`
	State        string `form:"state"`
}

type TokenRequest struct {
	GrantType    string `form:"grant_type" json:"grant_type" binding:"required,oneof=authorization_code client_credentials refresh_token password"`
	Code         string `form:"code" json:"code"`
	RedirectURI  string `form:"redirect_uri" json:"redirect_uri"`
	ClientID     string `form:"client_id" json:"client_id"`
	ClientSecret string `form:"client_secret" json:"client_secret"`
	RefreshToken string `form:"refresh_token" json:"refresh_token"`
	Scope        string `form:"scope" json:"scope"`
	Username     string `form:"username" json:"username"`
	Password     string `form:"password" json:"password"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

type RevokeRequest struct {
	Token        string `form:"token" json:"token" binding:"required"`
	ClientID     string `form:"client_id" json:"client_id" binding:"required"`
	ClientSecret string `form:"client_secret" json:"client_secret" binding:"required"`
}

type IntrospectRequest struct {
	Token string `form:"token" json:"token" binding:"required"`
}

type IntrospectResponse struct {
	Active    bool   `json:"active"`
	Scope     string `json:"scope,omitempty"`
	ClientID  string `json:"client_id,omitempty"`
	UserID    string `json:"user_id,omitempty"`
	ExpiresAt int64  `json:"exp,omitempty"`
}
