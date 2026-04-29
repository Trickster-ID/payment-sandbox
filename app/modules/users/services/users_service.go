package services

import (
	"errors"
	"strings"

	"payment-sandbox/app/modules/users/models/entity"
	"payment-sandbox/app/modules/users/repositories"
	"payment-sandbox/app/shared/validator"

	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	repo repositories.IUserRepository
}

type IUserService interface {
	RegisterMerchant(name, email, password string) (entity.User, error)
}

func NewUserService(repo repositories.IUserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) RegisterMerchant(name, email, password string) (entity.User, error) {
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
