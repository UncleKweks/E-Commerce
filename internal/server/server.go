// Package server builds the HTTP router: it creates the services and
// handlers, and connects each URL to its handler and middleware.
package server

import (
	"context"
	"net/http"
	"time"

	"github.com/UncleKweks/E-Commerce/internal/auth"
	"github.com/UncleKweks/E-Commerce/internal/config"
	"github.com/UncleKweks/E-Commerce/internal/handlers"
	"github.com/UncleKweks/E-Commerce/internal/middleware"
	"github.com/UncleKweks/E-Commerce/internal/response"
	"github.com/UncleKweks/E-Commerce/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// maxBodyBytes caps JSON request bodies at 1 MB. File uploads will get their own limit.
const maxBodyBytes = 1 << 20

// Server holds the dependencies every handler needs.
type Server struct {
	config *config.Config
	db     *gorm.DB
	logger *zerolog.Logger
}

// NewServer returns a Server. Dependencies are passed in rather than read
// from globals, which keeps each piece testable on its own.
func NewServer(cfg *config.Config, db *gorm.DB, logger *zerolog.Logger) *Server {
	return &Server{config: cfg, db: db, logger: logger}
}

// SetupRoutes builds the router with all middleware and routes.
func (s *Server) SetupRoutes() *gin.Engine {
	gin.SetMode(s.config.Server.GinMode)
	handlers.UseJSONFieldNames()

	router := gin.New()
	// Don't trust X-Forwarded-For from anyone. Otherwise a client can send a
	// fake IP in that header and dodge the per-IP rate limit. If you deploy
	// behind a load balancer, list its address here.
	_ = router.SetTrustedProxies(nil)

	router.Use(middleware.RequestLogger(s.logger))
	router.Use(middleware.Recovery())
	router.Use(middleware.CORS(s.config.Server.CORSAllowedOrigins))
	router.Use(middleware.BodyLimit(maxBodyBytes))

	router.NoRoute(func(c *gin.Context) {
		response.Error(c, http.StatusNotFound, "not_found", "This endpoint does not exist.")
	})
	router.GET("/health", s.healthCheckHandler)

	tokens := auth.NewTokenManager(s.config.JWT.Secret, s.config.JWT.ExpiresIn)
	authService := services.NewAuthService(s.db, tokens, s.config.JWT.RefreshTokenExpires)
	authHandler := handlers.NewAuthHandler(authService)

	// /api/v1: the version in the URL lets a future v2 change the API
	// without breaking apps that still use v1.
	api := router.Group("/api/v1")

	authRoutes := api.Group("/auth")
	// Login and register are where passwords get guessed, so they get a strict
	// limit: about 1 request every 6 seconds per IP, in bursts of up to 10.
	// Refresh tokens are 256-bit random values that can't be guessed, so refresh
	// doesn't need one.
	passwordLimit := middleware.RateLimit(6*time.Second, 10)
	authRoutes.POST("/register", passwordLimit, authHandler.Register)
	authRoutes.POST("/login", passwordLimit, authHandler.Login)
	authRoutes.POST("/refresh", authHandler.Refresh)
	authRoutes.POST("/logout", authHandler.Logout)

	protected := api.Group("")
	protected.Use(middleware.AuthRequired(tokens))
	protected.GET("/users/me", authHandler.Me)

	return router
}

// healthCheckHandler reports whether the API and its database are up.
// Load balancers and Docker call this to decide whether to send traffic here.
func (s *Server) healthCheckHandler(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	sqlDB, err := s.db.DB()
	if err == nil {
		err = sqlDB.PingContext(ctx)
	}
	if err != nil {
		zerolog.Ctx(c.Request.Context()).Error().Err(err).Msg("health check: database unreachable")
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable", "database": "down"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "database": "up"})
}
