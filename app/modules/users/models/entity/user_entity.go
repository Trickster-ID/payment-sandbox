package entity

import "time"

type Role string

const (
	RoleMerchant Role = "MERCHANT"
	RoleAdmin    Role = "ADMIN"
)

type User struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Role         Role      `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}
