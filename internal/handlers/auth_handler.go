// Package handlers turns HTTP requests into service calls and service
// results into HTTP responses. Handlers stay thin: read input, call a
// service, map the outcome to a status code.
package handlers

import (
	"errors"
	"net/http"

	"github.com/UncleKweks/E-Commerce/internal/auth"
	"github.com/UncleKweks/E-Commerce/internal/dto"
	"github.com/UncleKweks/E-Commerce/internal/middleware"
	"github.com/UncleKweks/E-Commerce/internal/response"
	"github.com/UncleKweks/E-Commerce/internal/services"
	"github.com/gin-gonic/gin"
)

// AuthHandler serves the /auth and /users/me endpoints.
type AuthHandler struct {
	auth *services.AuthService
}

// NewAuthHandler returns an AuthHandler.
func NewAuthHandler(authService *services.AuthService) *AuthHandler {
	return &AuthHandler{auth: authService}
}

// Register handles POST /auth/register.
func (h *AuthHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if !bindJSON(c, &req) {
		return
	}

	tokens, err := h.auth.Register(c.Request.Context(), &req)
	switch {
	case errors.Is(err, services.ErrEmailTaken):
		response.Error(c, http.StatusConflict, "email_taken", "An account with this email already exists.")
	case errors.Is(err, auth.ErrPasswordTooLong):
		response.ValidationError(c, map[string]string{"password": "must be at most 72 bytes"})
	case err != nil:
		internalError(c, err)
	default:
		response.Created(c, tokens)
	}
}

// Login handles POST /auth/login.
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if !bindJSON(c, &req) {
		return
	}

	tokens, err := h.auth.Login(c.Request.Context(), req.Email, req.Password)
	switch {
	case errors.Is(err, services.ErrInvalidCredentials):
		response.Error(c, http.StatusUnauthorized, "invalid_credentials", "Email or password is incorrect.")
	case err != nil:
		internalError(c, err)
	default:
		response.OK(c, tokens)
	}
}

// Refresh handles POST /auth/refresh.
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req dto.RefreshRequest
	if !bindJSON(c, &req) {
		return
	}

	tokens, err := h.auth.Refresh(c.Request.Context(), req.RefreshToken)
	switch {
	case errors.Is(err, services.ErrInvalidToken):
		response.Error(c, http.StatusUnauthorized, "invalid_refresh_token", "Your session has expired. Please log in again.")
	case err != nil:
		internalError(c, err)
	default:
		response.OK(c, tokens)
	}
}

// Logout handles POST /auth/logout.
func (h *AuthHandler) Logout(c *gin.Context) {
	var req dto.RefreshRequest
	if !bindJSON(c, &req) {
		return
	}
	if err := h.auth.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		internalError(c, err)
		return
	}
	response.NoContent(c)
}

// Me handles GET /users/me and returns the logged-in user.
func (h *AuthHandler) Me(c *gin.Context) {
	user, err := h.auth.GetUser(c.Request.Context(), middleware.UserID(c))
	switch {
	case errors.Is(err, services.ErrUserNotFound):
		// The token is valid but the account was deleted or disabled since it was issued.
		response.Error(c, http.StatusUnauthorized, "unauthorized", "Please log in again.")
	case err != nil:
		internalError(c, err)
	default:
		response.OK(c, dto.NewUserResponse(user))
	}
}
