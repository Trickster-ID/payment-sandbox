package repositories

import (
	"database/sql"
	"errors"
	"strings"

	"payment-sandbox/app/modules/users/models/entity"

	"github.com/jackc/pgx/v5/pgconn"
)

type IUserRepository interface {
	CreateUser(name, email, passwordHash string, role entity.Role) (entity.User, error)
	FindUserByEmail(email string) (entity.User, bool)
}

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) CreateUser(name, email, passwordHash string, role entity.Role) (entity.User, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return entity.User{}, err
	}
	defer tx.Rollback()

	email = strings.ToLower(strings.TrimSpace(email))
	var user entity.User
	err = tx.QueryRow(`
		INSERT INTO users (name, email, password_hash, role)
		VALUES ($1, $2, $3, $4)
		RETURNING id::text, name, email, role::text, created_at
	`, strings.TrimSpace(name), email, passwordHash, string(role)).
		Scan(&user.ID, &user.Name, &user.Email, &user.Role, &user.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return entity.User{}, errors.New("email already registered")
		}
		return entity.User{}, err
	}
	user.PasswordHash = passwordHash

	if role == entity.RoleMerchant {
		if _, err := tx.Exec(`INSERT INTO merchants (user_id) VALUES ($1)`, user.ID); err != nil {
			return entity.User{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return entity.User{}, err
	}
	return user, nil
}

func (r *UserRepository) FindUserByEmail(email string) (entity.User, bool) {
	var user entity.User
	err := r.db.QueryRow(`
		SELECT id::text, name, email, password_hash, role::text, created_at
		FROM users
		WHERE lower(email) = lower($1) AND deleted_at IS NULL
		LIMIT 1
	`, strings.TrimSpace(email)).
		Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash, &user.Role, &user.CreatedAt)
	if err != nil {
		return entity.User{}, false
	}
	return user, true
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
