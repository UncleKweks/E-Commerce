// Package config loads application settings from environment variables and .env.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// minJWTSecretLength is 32 bytes (256 bits), the minimum safe key size for HS256.
const minJWTSecretLength = 32

// Config holds all application settings.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	JWT      JWTConfig
	AWS      AWSConfig
	Upload   UploadConfig
	SMTP     SMTPConfig
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port    string
	GinMode string
	// CORSAllowedOrigins lists the websites whose browser code may call this API.
	CORSAllowedOrigins []string
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

// JWTConfig holds token signing and expiry settings.
type JWTConfig struct {
	Secret              string
	ExpiresIn           time.Duration
	RefreshTokenExpires time.Duration
}

// AWSConfig holds AWS (or LocalStack) credentials, S3 and queue settings.
type AWSConfig struct {
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	S3Bucket        string
	S3Endpoint      string
	EventQueueName  string
}

// SMTPConfig holds outgoing email settings.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

// UploadConfig holds file upload settings.
type UploadConfig struct {
	Path        string
	MaxFileSize int64

	// UploadProvider  can be s3 or local
	UploadProvider string
}

// Load reads .env (if present) and the environment, falling back to defaults.
// It returns every invalid setting at once, so the app refuses to start
// with a broken config instead of quietly running with a wrong value.
func Load() (*Config, error) {
	_ = godotenv.Load()

	var errs []error
	duration := func(key, def string) time.Duration {
		d, err := time.ParseDuration(getEnv(key, def))
		if err != nil || d <= 0 {
			errs = append(errs, fmt.Errorf("%s must be a positive duration like 15m or 168h", key))
		}
		return d
	}
	integer := func(key, def string) int64 {
		n, err := strconv.ParseInt(getEnv(key, def), 10, 64)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s must be a whole number", key))
		}
		return n
	}

	cfg := &Config{
		Server: ServerConfig{
			Port:               getEnv("PORT", "8080"),
			GinMode:            getEnv("GIN_MODE", "debug"),
			CORSAllowedOrigins: splitList(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "password"),
			Name:     getEnv("DB_NAME", "ecommerce_shop"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		JWT: JWTConfig{
			// No default on purpose: a default secret would be public on GitHub.
			Secret:              os.Getenv("JWT_SECRET"),
			ExpiresIn:           duration("JWT_EXPIRATION", "15m"),
			RefreshTokenExpires: duration("REFRESH_TOKEN_EXPIRATION", "168h"),
		},
		AWS: AWSConfig{
			Region:          getEnv("AWS_REGION", "us-east-1"),
			AccessKeyID:     getEnv("AWS_ACCESS_KEY_ID", "test"),
			SecretAccessKey: getEnv("AWS_SECRET_ACCESS_KEY", "test"),
			S3Bucket:        getEnv("AWS_S3_BUCKET", "ecommerce-uploads"),
			S3Endpoint:      getEnv("AWS_S3_ENDPOINT", "http://localhost:4566"),
			EventQueueName:  getEnv("AWS_EVENT_QUEUE_NAME", "ecommerce-events"),
		},
		Upload: UploadConfig{
			Path:           getEnv("UPLOAD_PATH", "./uploads"),
			MaxFileSize:    integer("MAX_UPLOAD_SIZE", "10485760"),
			UploadProvider: getEnv("UPLOAD_PROVIDER", "local"),
		},
		SMTP: SMTPConfig{
			Host:     getEnv("SMTP_HOST", "localhost"),
			Port:     int(integer("SMTP_PORT", "1025")),
			Username: getEnv("SMTP_USERNAME", ""),
			Password: getEnv("SMTP_PASSWORD", ""),
			From:     getEnv("SMTP_FROM", "noreply@shop.com"),
		},
	}

	if len(cfg.JWT.Secret) < minJWTSecretLength {
		errs = append(errs, fmt.Errorf("JWT_SECRET must be at least %d characters; generate one with: openssl rand -hex 32", minJWTSecretLength))
	}
	if cfg.Server.GinMode != "debug" && cfg.Server.GinMode != "release" && cfg.Server.GinMode != "test" {
		errs = append(errs, errors.New("GIN_MODE must be debug, release or test"))
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("invalid configuration: %w", errors.Join(errs...))
	}
	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// splitList turns "a, b,c" into ["a", "b", "c"].
func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
