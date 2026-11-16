package models

import (
	"time"

	"gorm.io/gorm"
)

// User is a customer or admin account.
type User struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Email        string         `json:"email"`
	PasswordHash string         `json:"-"`
	FirstName    string         `json:"first_name"`
	LastName     string         `json:"last_name"`
	Phone        *string        `json:"phone,omitempty"` // pointer: the column is nullable
	Role         UserRole       `json:"role"`
	IsActive     bool           `json:"is_active"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-"`
}

// UserRole controls what a user is allowed to do.
type UserRole string

// User roles.
const (
	UserRoleCustomer UserRole = "customer"
	UserRoleAdmin    UserRole = "admin"
)

// RefreshToken lets a user get new access tokens without logging in again.
// Only a hash of the token is stored.
type RefreshToken struct {
	ID        uint       `gorm:"primaryKey"`
	UserID    uint       `gorm:"not null"`
	TokenHash string     `gorm:"not null"`
	ExpiresAt time.Time  `gorm:"not null"`
	RevokedAt *time.Time // nil until the token is revoked
	CreatedAt time.Time
}
