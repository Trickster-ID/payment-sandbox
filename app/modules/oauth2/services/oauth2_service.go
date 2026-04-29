package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"payment-sandbox/app/config"
	"payment-sandbox/app/modules/oauth2/models/entity"
	"payment-sandbox/app/modules/oauth2/repositories"
	authEntity "payment-sandbox/app/modules/auth/models/entity"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type ClientWithSecret struct {
	Client       entity.OAuthClient `json:"client"`
	ClientSecret string             `json:"client_secret"`
}

type OAuth2Claims struct {
	UserID   string `json:"user_id,omitempty"`
	ClientID string `json:"client_id"`
	Scope    string `json:"scope"`
	jwt.RegisteredClaims
}

type IOAuth2Service interface {
	// Client management
	RegisterClient(ownerID, name string, redirectURIs, scopes []string) (ClientWithSecret, error)
	ListClients(ownerID string) ([]entity.OAuthClient, error)
	GetClient(id string) (entity.OAuthClient, error)
	DeleteClient(clientID, ownerID string) error

	// Core OAuth2 Logic
	ValidateClient(clientID, clientSecret string) (entity.OAuthClient, error)
	IssueAuthCode(clientID, userID, redirectURI, scope string) (string, error)
	ExchangeAuthCode(code, clientID, redirectURI string) (entity.AuthorizationCode, error)
	IssueRefreshToken(clientID, userID, scope string) (string, error)
	ExchangeRefreshToken(token, clientID string) (entity.RefreshToken, error)
	IssueAccessToken(clientID, userID, scope string) (string, error)
	ValidateToken(token string) (*OAuth2Claims, error)
	ValidateUserCredentials(email, password string) (authEntity.User, error)
	RevokeRefreshToken(token, clientID string) error
}

type OAuth2Service struct {
	repo repositories.IOAuth2Repository
	cfg  config.Config
}

func NewOAuth2Service(repo repositories.IOAuth2Repository, cfg config.Config) *OAuth2Service {
	return &OAuth2Service{repo: repo, cfg: cfg}
}

func (s *OAuth2Service) RegisterClient(ownerID, name string, redirectURIs, scopes []string) (ClientWithSecret, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 255 {
		return ClientWithSecret{}, errors.New("invalid client name")
	}

	if len(redirectURIs) == 0 {
		return ClientWithSecret{}, errors.New("at least one redirect URI is required")
	}
	for _, uri := range redirectURIs {
		if !isValidURL(uri) {
			return ClientWithSecret{}, fmt.Errorf("invalid redirect URI: %s", uri)
		}
	}

	validScopes := map[string]bool{"read": true, "write": true, "admin": true}
	for _, sc := range scopes {
		if !validScopes[sc] {
			return ClientWithSecret{}, fmt.Errorf("invalid scope: %s", sc)
		}
	}

	// Generate random client secret
	plaintextSecret, err := generateRandomHex(32)
	if err != nil {
		return ClientWithSecret{}, err
	}

	secretHash, err := bcrypt.GenerateFromPassword([]byte(plaintextSecret), bcrypt.DefaultCost)
	if err != nil {
		return ClientWithSecret{}, err
	}

	client := entity.OAuthClient{
		OwnerID:        &ownerID,
		Name:           name,
		SecretHash:     string(secretHash),
		RedirectURIs:   redirectURIs,
		Scopes:         scopes,
		IsFirstParty:   false, // Merchants create third-party clients by default
		IsConfidential: true,
	}

	saved, err := s.repo.CreateClient(client)
	if err != nil {
		return ClientWithSecret{}, err
	}

	return ClientWithSecret{
		Client:       saved,
		ClientSecret: plaintextSecret,
	}, nil
}

func (s *OAuth2Service) ListClients(ownerID string) ([]entity.OAuthClient, error) {
	return s.repo.ListClientsByOwner(ownerID)
}

func (s *OAuth2Service) GetClient(id string) (entity.OAuthClient, error) {
	client, found := s.repo.FindClientByID(id)
	if !found {
		return entity.OAuthClient{}, errors.New("client not found")
	}
	return client, nil
}

func (s *OAuth2Service) DeleteClient(clientID, ownerID string) error {
	return s.repo.DeleteClient(clientID, ownerID)
}

func (s *OAuth2Service) ValidateClient(clientID, clientSecret string) (entity.OAuthClient, error) {
	client, found := s.repo.FindClientByID(clientID)
	if !found {
		return entity.OAuthClient{}, errors.New("invalid client_id")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(client.SecretHash), []byte(clientSecret)); err != nil {
		return entity.OAuthClient{}, errors.New("invalid client_secret")
	}

	return client, nil
}

func (s *OAuth2Service) IssueAuthCode(clientID, userID, redirectURI, scope string) (string, error) {
	code, err := generateRandomHex(24)
	if err != nil {
		return "", err
	}

	authCode := entity.AuthorizationCode{
		Code:        code,
		ClientID:    clientID,
		UserID:      userID,
		RedirectURI: redirectURI,
		Scope:       scope,
		ExpiresAt:   time.Now().Add(s.cfg.OAuth2AuthCodeDuration),
	}

	if err := s.repo.SaveAuthCode(authCode); err != nil {
		return "", err
	}

	return code, nil
}

func (s *OAuth2Service) ExchangeAuthCode(code, clientID, redirectURI string) (entity.AuthorizationCode, error) {
	authCode, found := s.repo.FindAuthCode(code)
	if !found {
		return entity.AuthorizationCode{}, errors.New("invalid or expired authorization code")
	}

	if authCode.ClientID != clientID {
		return entity.AuthorizationCode{}, errors.New("client mismatch")
	}

	if authCode.RedirectURI != redirectURI {
		return entity.AuthorizationCode{}, errors.New("redirect_uri mismatch")
	}

	if err := s.repo.MarkAuthCodeUsed(code); err != nil {
		return entity.AuthorizationCode{}, err
	}

	return authCode, nil
}

func (s *OAuth2Service) IssueRefreshToken(clientID, userID, scope string) (string, error) {
	token, err := generateRandomHex(32)
	if err != nil {
		return "", err
	}

	refreshToken := entity.RefreshToken{
		Token:     token,
		ClientID:  clientID,
		UserID:    userID,
		Scope:     scope,
		ExpiresAt: time.Now().Add(s.cfg.OAuth2RefreshTokenDuration),
	}

	if err := s.repo.SaveRefreshToken(refreshToken); err != nil {
		return "", err
	}

	return token, nil
}

func (s *OAuth2Service) ExchangeRefreshToken(token, clientID string) (entity.RefreshToken, error) {
	refreshToken, found := s.repo.FindRefreshToken(token)
	if !found {
		return entity.RefreshToken{}, errors.New("invalid or expired refresh token")
	}

	if refreshToken.ClientID != clientID {
		return entity.RefreshToken{}, errors.New("client mismatch")
	}

	if err := s.repo.RevokeRefreshToken(token); err != nil {
		return entity.RefreshToken{}, err
	}

	return refreshToken, nil
}

func (s *OAuth2Service) IssueAccessToken(clientID, userID, scope string) (string, error) {
	now := time.Now()
	claims := OAuth2Claims{
		UserID:   userID,
		ClientID: clientID,
		Scope:    scope,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.OAuth2AccessTokenDuration)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.JWTSecret))
}

func (s *OAuth2Service) ValidateToken(tokenString string) (*OAuth2Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &OAuth2Claims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(s.cfg.JWTSecret), nil
	})

	if err != nil || !token.Valid {
		return nil, errors.New("invalid or expired token")
	}

	claims, ok := token.Claims.(*OAuth2Claims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}

	return claims, nil
}

func (s *OAuth2Service) ValidateUserCredentials(email, password string) (authEntity.User, error) {
	user, found := s.repo.FindUserByEmail(email)
	if !found {
		return authEntity.User{}, errors.New("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return authEntity.User{}, errors.New("invalid credentials")
	}

	return user, nil
}

func (s *OAuth2Service) RevokeRefreshToken(token, clientID string) error {
	refreshToken, found := s.repo.FindRefreshToken(token)
	if !found {
		return nil // RFC 7009: return success if not found
	}

	if refreshToken.ClientID != clientID {
		return errors.New("client mismatch")
	}

	return s.repo.RevokeRefreshToken(token)
}

// Helpers

func isValidURL(u string) bool {
	parsed, err := url.Parse(u)
	if err != nil {
		return false
	}
	return parsed.Scheme != "" && parsed.Host != "" && parsed.Fragment == ""
}

func generateRandomHex(byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
