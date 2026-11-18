// Package middleware holds code that runs before (and after) handlers:
// logging, authentication, rate limiting and other cross-cutting concerns.
package middleware

import (
	"net/http"
	"strings"

	"github.com/UncleKweks/E-Commerce/internal/auth"
	"github.com/UncleKweks/E-Commerce/internal/response"
	"github.com/gin-gonic/gin"
)

const (
	userIDKey = "auth.user_id"
	roleKey   = "auth.role"
)

// AuthRequired rejects requests without a valid "Authorization: Bearer <token>"
// header. On success the user's ID and role are available via UserID and Role.
func AuthRequired(tokens *auth.TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		token, found := strings.CutPrefix(header, "Bearer ")
		if !found || token == "" {
			response.Error(c, http.StatusUnauthorized, "unauthorized", "You need to log in to do this.")
			return
		}

		claims, err := tokens.ParseAccessToken(token)
		if err != nil {
			response.Error(c, http.StatusUnauthorized, "unauthorized", "Your session has expired. Please log in again.")
			return
		}
		userID, err := claims.UserID()
		if err != nil {
			response.Error(c, http.StatusUnauthorized, "unauthorized", "Your session has expired. Please log in again.")
			return
		}

		c.Set(userIDKey, userID)
		c.Set(roleKey, claims.Role)
		c.Next()
	}
}

// RequireRole allows only users with one of the given roles. It must run after AuthRequired.
// 401 means "we don't know who you are"; 403 means "we know, and you're not allowed".
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := Role(c)
		for _, r := range roles {
			if role == r {
				c.Next()
				return
			}
		}
		response.Error(c, http.StatusForbidden, "forbidden", "You don't have permission to do this.")
	}
}

// UserID returns the logged-in user's ID. Only valid behind AuthRequired.
func UserID(c *gin.Context) uint {
	return c.GetUint(userIDKey)
}

// Role returns the logged-in user's role. Only valid behind AuthRequired.
func Role(c *gin.Context) string {
	return c.GetString(roleKey)
}
