package services

import (
	"errors"
	"strings"

	"payment-sandbox/app/middleware"
	"payment-sandbox/app/modules/auth/models/entity"
	"payment-sandbox/app/modules/auth/repositories"
	"payment-sandbox/app/shared/validator"

	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	repo repositories.IAuthRepository
	jwt  middleware.JWTService
}

type IAuthService interface {
	RegisterMerchant(name, email, password string) (entity.User, error)
	Login(email, password string) (string, entity.User, error)
}

func NewAuthService(repo repositories.IAuthRepository, jwt middleware.JWTService) *AuthService {
	return &AuthService{repo: repo, jwt: jwt}
}

func (s *AuthService) RegisterMerchant(name, email, password string) (entity.User, error) {
	name = strings.TrimSpace(name)
	email = strings.TrimSpace(strings.ToLower(email))

	if name == "" {
		return entity.User{}, errors.New("name is required")
	}
	if !validator.IsEmail(email) {
		return entity.User{}, errors.New("email is invalid")
	}
	if len(password) < 8 {
		return entity.User{}, errors.New("password minimum length is 8")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return entity.User{}, err
	}
	return s.repo.CreateUser(name, email, string(hash), entity.RoleMerchant)
}

func (s *AuthService) Login(email, password string) (string, entity.User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	user, found := s.repo.FindUserByEmail(email)
	if !found {
		return "", entity.User{}, errors.New("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", entity.User{}, errors.New("invalid credentials")
	}

	token, err := s.jwt.GenerateToken(user.ID, user.Role)
	if err != nil {
		return "", entity.User{}, err
	}
	return token, user, nil
}
