package models

import (
	"time"

	"gorm.io/gorm"
)

// User is a customer or admin account.
type User struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Email     string         `gorm:"uniqueIndex" json:"email"`
	Password  string         `json:"-"`
	FirstName string         `json:"first_name"`
	LastName  string         `json:"last_name"`
	Phone     string         `json:"phone"`
	IsActive  bool           `json:"is_active"`
	Role      UserRole       `json:"role"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	RefreshTokens []RefreshToken `gorm:"foreignKey:UserID" json:"refresh_tokens,omitempty"`
	Orders        []Order        `gorm:"foreignKey:UserID" json:"orders,omitempty"`
	CartItems     []CartItem     `gorm:"foreignKey:UserID" json:"cart_items,omitempty"`
}

// UserRole controls what a user is allowed to do.
type UserRole string

// User roles.
const (
	UserRoleCustomer UserRole = "customer"
	UserRoleAdmin    UserRole = "admin"
)

// RefreshToken is a long-lived token used to get new JWT access tokens.
type RefreshToken struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Token     string         `gorm:"uniqueIndex" json:"token"`
	UserID    uint           `json:"user_id"`
	ExpiresAt time.Time      `json:"expires_at"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}
