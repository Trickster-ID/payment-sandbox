package repositories

import (
	"database/sql"
	"payment-sandbox/app/modules/oauth2/models/entity"
	userEntity "payment-sandbox/app/modules/users/models/entity"

	"github.com/lib/pq"
)

type IOAuth2Repository interface {
	// Client CRUD
	CreateClient(client entity.OAuthClient) (entity.OAuthClient, error)
	FindClientByID(id string) (entity.OAuthClient, bool)
	ListClientsByOwner(ownerID string) ([]entity.OAuthClient, error)
	DeleteClient(id, ownerID string) error

	// Authorization codes
	SaveAuthCode(code entity.AuthorizationCode) error
	FindAuthCode(code string) (entity.AuthorizationCode, bool)
	MarkAuthCodeUsed(code string) error

	// Refresh tokens
	SaveRefreshToken(token entity.RefreshToken) error
	FindRefreshToken(token string) (entity.RefreshToken, bool)
	RevokeRefreshToken(token string) error
	RevokeAllRefreshTokens(clientID, userID string) error

	// Consent
	FindConsent(userID, clientID string) (entity.Consent, bool)
	SaveConsent(consent entity.Consent) error

	// User lookup
	FindUserByID(id string) (userEntity.User, bool)
	FindUserByEmail(email string) (userEntity.User, bool)
}

type OAuth2Repository struct {
	db *sql.DB
}

func NewOAuth2Repository(db *sql.DB) *OAuth2Repository {
	return &OAuth2Repository{db: db}
}

func (r *OAuth2Repository) CreateClient(client entity.OAuthClient) (entity.OAuthClient, error) {
	var result entity.OAuthClient
	err := r.db.QueryRow(`
		INSERT INTO oauth2_clients (owner_id, client_secret, name, redirect_uris, scopes, is_first_party, is_confidential)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id::text, owner_id::text, name, redirect_uris, scopes, is_first_party, is_confidential, created_at, updated_at
	`, client.OwnerID, client.SecretHash, client.Name, pq.Array(client.RedirectURIs), pq.Array(client.Scopes), client.IsFirstParty, client.IsConfidential).
		Scan(&result.ID, &result.OwnerID, &result.Name, pq.Array(&result.RedirectURIs), pq.Array(&result.Scopes), &result.IsFirstParty, &result.IsConfidential, &result.CreatedAt, &result.UpdatedAt)
	
	if err != nil {
		return entity.OAuthClient{}, err
	}
	return result, nil
}

func (r *OAuth2Repository) FindClientByID(id string) (entity.OAuthClient, bool) {
	var client entity.OAuthClient
	err := r.db.QueryRow(`
		SELECT id::text, owner_id::text, client_secret, name, redirect_uris, scopes, is_first_party, is_confidential, created_at, updated_at
		FROM oauth2_clients
		WHERE id = $1 AND deleted_at IS NULL
	`, id).
		Scan(&client.ID, &client.OwnerID, &client.SecretHash, &client.Name, pq.Array(&client.RedirectURIs), pq.Array(&client.Scopes), &client.IsFirstParty, &client.IsConfidential, &client.CreatedAt, &client.UpdatedAt)

	if err != nil {
		return entity.OAuthClient{}, false
	}
	return client, true
}

func (r *OAuth2Repository) ListClientsByOwner(ownerID string) ([]entity.OAuthClient, error) {
	rows, err := r.db.Query(`
		SELECT id::text, owner_id::text, name, redirect_uris, scopes, is_first_party, is_confidential, created_at, updated_at
		FROM oauth2_clients
		WHERE owner_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
	`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clients []entity.OAuthClient
	for rows.Next() {
		var c entity.OAuthClient
		if err := rows.Scan(&c.ID, &c.OwnerID, &c.Name, pq.Array(&c.RedirectURIs), pq.Array(&c.Scopes), &c.IsFirstParty, &c.IsConfidential, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		clients = append(clients, c)
	}
	return clients, nil
}

func (r *OAuth2Repository) DeleteClient(id, ownerID string) error {
	_, err := r.db.Exec(`
		UPDATE oauth2_clients
		SET deleted_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND owner_id = $2 AND deleted_at IS NULL
	`, id, ownerID)
	return err
}

func (r *OAuth2Repository) SaveAuthCode(code entity.AuthorizationCode) error {
	_, err := r.db.Exec(`
		INSERT INTO oauth2_authorization_codes (code, client_id, user_id, redirect_uri, scope, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, code.Code, code.ClientID, code.UserID, code.RedirectURI, code.Scope, code.ExpiresAt)
	return err
}

func (r *OAuth2Repository) FindAuthCode(code string) (entity.AuthorizationCode, bool) {
	var c entity.AuthorizationCode
	err := r.db.QueryRow(`
		SELECT id::text, code, client_id::text, user_id::text, redirect_uri, scope, expires_at, used, created_at
		FROM oauth2_authorization_codes
		WHERE code = $1 AND used = false AND expires_at > CURRENT_TIMESTAMP
	`, code).
		Scan(&c.ID, &c.Code, &c.ClientID, &c.UserID, &c.RedirectURI, &c.Scope, &c.ExpiresAt, &c.Used, &c.CreatedAt)
	
	if err != nil {
		return entity.AuthorizationCode{}, false
	}
	return c, true
}

func (r *OAuth2Repository) MarkAuthCodeUsed(code string) error {
	_, err := r.db.Exec(`
		UPDATE oauth2_authorization_codes
		SET used = true
		WHERE code = $1
	`, code)
	return err
}

func (r *OAuth2Repository) SaveRefreshToken(token entity.RefreshToken) error {
	_, err := r.db.Exec(`
		INSERT INTO oauth2_refresh_tokens (token, client_id, user_id, scope, expires_at)
		VALUES ($1, $2, $3, $4, $5)
	`, token.Token, token.ClientID, token.UserID, token.Scope, token.ExpiresAt)
	return err
}

func (r *OAuth2Repository) FindRefreshToken(token string) (entity.RefreshToken, bool) {
	var t entity.RefreshToken
	err := r.db.QueryRow(`
		SELECT id::text, token, client_id::text, user_id::text, scope, expires_at, revoked, created_at
		FROM oauth2_refresh_tokens
		WHERE token = $1 AND revoked = false AND expires_at > CURRENT_TIMESTAMP
	`, token).
		Scan(&t.ID, &t.Token, &t.ClientID, &t.UserID, &t.Scope, &t.ExpiresAt, &t.Revoked, &t.CreatedAt)
	
	if err != nil {
		return entity.RefreshToken{}, false
	}
	return t, true
}

func (r *OAuth2Repository) RevokeRefreshToken(token string) error {
	_, err := r.db.Exec(`
		UPDATE oauth2_refresh_tokens
		SET revoked = true
		WHERE token = $1
	`, token)
	return err
}

func (r *OAuth2Repository) RevokeAllRefreshTokens(clientID, userID string) error {
	_, err := r.db.Exec(`
		UPDATE oauth2_refresh_tokens
		SET revoked = true
		WHERE client_id = $1 AND user_id = $2 AND revoked = false
	`, clientID, userID)
	return err
}

func (r *OAuth2Repository) FindConsent(userID, clientID string) (entity.Consent, bool) {
	var c entity.Consent
	err := r.db.QueryRow(`
		SELECT id::text, user_id::text, client_id::text, scope, created_at
		FROM oauth2_consents
		WHERE user_id = $1 AND client_id = $2
	`, userID, clientID).
		Scan(&c.ID, &c.UserID, &c.ClientID, &c.Scope, &c.CreatedAt)
	
	if err != nil {
		return entity.Consent{}, false
	}
	return c, true
}

func (r *OAuth2Repository) SaveConsent(consent entity.Consent) error {
	_, err := r.db.Exec(`
		INSERT INTO oauth2_consents (user_id, client_id, scope)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, client_id) DO UPDATE SET scope = EXCLUDED.scope
	`, consent.UserID, consent.ClientID, consent.Scope)
	return err
}

func (r *OAuth2Repository) FindUserByID(id string) (userEntity.User, bool) {
	var user userEntity.User
	err := r.db.QueryRow(`
		SELECT id::text, name, email, password_hash, role::text, created_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`, id).
		Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash, &user.Role, &user.CreatedAt)
	if err != nil {
		return userEntity.User{}, false
	}
	return user, true
}

func (r *OAuth2Repository) FindUserByEmail(email string) (userEntity.User, bool) {
	var user userEntity.User
	err := r.db.QueryRow(`
		SELECT id::text, name, email, password_hash, role::text, created_at
		FROM users
		WHERE lower(email) = lower($1) AND deleted_at IS NULL
	`, email).
		Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash, &user.Role, &user.CreatedAt)
	if err != nil {
		return userEntity.User{}, false
	}
	return user, true
}
