// Package dto defines the JSON shapes the API accepts and returns
// (Data Transfer Objects). They are deliberately separate from the database
// models, so the API only exposes and accepts what we choose.
package dto

import (
	"time"

	"github.com/UncleKweks/E-Commerce/internal/models"
)

// RegisterRequest is the body of POST /auth/register.
// There is no Role field: clients can never choose their own role.
type RegisterRequest struct {
	Email     string  `json:"email" binding:"required,email,max=254"`
	Password  string  `json:"password" binding:"required,min=8,max=72"`
	FirstName string  `json:"first_name" binding:"required,max=100"`
	LastName  string  `json:"last_name" binding:"required,max=100"`
	Phone     *string `json:"phone" binding:"omitempty,e164"`
}

// LoginRequest is the body of POST /auth/login.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,max=72"`
}

// RefreshRequest is the body of POST /auth/refresh and POST /auth/logout.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// TokenResponse is returned after register, login and refresh.
type TokenResponse struct {
	AccessToken  string        `json:"access_token"`
	RefreshToken string        `json:"refresh_token"`
	TokenType    string        `json:"token_type"`
	ExpiresIn    int           `json:"expires_in"` // seconds until the access token expires
	User         *UserResponse `json:"user,omitempty"`
}

// UserResponse is the public view of a user. It never includes the password hash.
type UserResponse struct {
	ID        uint      `json:"id"`
	Email     string    `json:"email"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Phone     *string   `json:"phone,omitempty"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// NewUserResponse converts a database user into its public view.
func NewUserResponse(u *models.User) *UserResponse {
	return &UserResponse{
		ID:        u.ID,
		Email:     u.Email,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Phone:     u.Phone,
		Role:      string(u.Role),
		CreatedAt: u.CreatedAt.UTC(), // the driver returns local time; the API always speaks UTC
	}
}
