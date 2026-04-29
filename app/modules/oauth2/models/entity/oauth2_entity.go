package entity

import (
	"time"
)

type OAuthClient struct {
	ID             string     `json:"id"`
	OwnerID        *string    `json:"owner_id"`
	SecretHash     string     `json:"-"`
	Name           string     `json:"name"`
	RedirectURIs   []string   `json:"redirect_uris"`
	Scopes         []string   `json:"scopes"`
	IsFirstParty   bool       `json:"is_first_party"`
	IsConfidential bool       `json:"is_confidential"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	DeletedAt      *time.Time `json:"deleted_at,omitempty"`
}

type AuthorizationCode struct {
	ID          string    `json:"id"`
	Code        string    `json:"code"`
	ClientID    string    `json:"client_id"`
	UserID      string    `json:"user_id"`
	RedirectURI string    `json:"redirect_uri"`
	Scope       string    `json:"scope"`
	ExpiresAt   time.Time `json:"expires_at"`
	Used        bool      `json:"used"`
	CreatedAt   time.Time `json:"created_at"`
}

type RefreshToken struct {
	ID        string    `json:"id"`
	Token     string    `json:"token"`
	ClientID  string    `json:"client_id"`
	UserID    string    `json:"user_id"`
	Scope     string    `json:"scope"`
	ExpiresAt time.Time `json:"expires_at"`
	Revoked   bool      `json:"revoked"`
	CreatedAt time.Time `json:"created_at"`
}

type Consent struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	ClientID  string    `json:"client_id"`
	Scope     string    `json:"scope"`
	CreatedAt time.Time `json:"created_at"`
}
