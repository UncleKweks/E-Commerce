// Package services holds the business logic. Services don't know about HTTP:
// they take plain Go values, return plain Go values and errors, and the
// handlers decide which HTTP status each error becomes.
package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/UncleKweks/E-Commerce/internal/auth"
	"github.com/UncleKweks/E-Commerce/internal/dto"
	"github.com/UncleKweks/E-Commerce/internal/models"
	"gorm.io/gorm"
)

// Errors the auth service can return. Handlers check them with errors.Is.
var (
	ErrEmailTaken         = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInvalidToken       = errors.New("invalid or expired refresh token")
	ErrUserNotFound       = errors.New("user not found")
)

// AuthService handles registration, login and token refresh.
type AuthService struct {
	db         *gorm.DB
	tokens     *auth.TokenManager
	refreshTTL time.Duration
}

// NewAuthService returns an AuthService.
func NewAuthService(db *gorm.DB, tokens *auth.TokenManager, refreshTTL time.Duration) *AuthService {
	return &AuthService{db: db, tokens: tokens, refreshTTL: refreshTTL}
}

// Register creates a customer account and logs it in.
func (s *AuthService) Register(ctx context.Context, req *dto.RegisterRequest) (*dto.TokenResponse, error) {
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	user := models.User{
		Email:        normalizeEmail(req.Email),
		PasswordHash: hash,
		FirstName:    strings.TrimSpace(req.FirstName),
		LastName:     strings.TrimSpace(req.LastName),
		Phone:        req.Phone,
		Role:         models.UserRoleCustomer, // always; never taken from the request
		IsActive:     true,
	}

	// We don't check "does this email exist?" first: two sign-ups at the same
	// moment could both pass that check. The unique index decides instead,
	// and the loser gets ErrDuplicatedKey.
	if err := s.db.WithContext(ctx).Create(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, ErrEmailTaken
		}
		return nil, fmt.Errorf("create user: %w", err)
	}

	return s.issueTokens(s.db.WithContext(ctx), &user)
}

// Login checks credentials and returns new tokens.
func (s *AuthService) Login(ctx context.Context, email, password string) (*dto.TokenResponse, error) {
	var user models.User
	err := s.db.WithContext(ctx).Where("email = ?", normalizeEmail(email)).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		auth.SpendPasswordCheckTime(password)
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}

	// Same error for "no such email", "wrong password" and "disabled account",
	// so attackers can't use the login form to find out which emails exist.
	if !auth.CheckPassword(user.PasswordHash, password) || !user.IsActive {
		return nil, ErrInvalidCredentials
	}

	return s.issueTokens(s.db.WithContext(ctx), &user)
}

// Refresh swaps a valid refresh token for a new pair (rotation). Each refresh
// token works once. If a token that was already used shows up again, someone
// has a copy of it, so every session for that user is ended.
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*dto.TokenResponse, error) {
	var result *dto.TokenResponse
	reused := false

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var stored models.RefreshToken
		err := tx.Where("token_hash = ?", auth.HashRefreshToken(refreshToken)).First(&stored).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrInvalidToken
		}
		if err != nil {
			return fmt.Errorf("find refresh token: %w", err)
		}

		if stored.RevokedAt != nil {
			reused = true
			return ErrInvalidToken
		}
		if time.Now().After(stored.ExpiresAt) {
			return ErrInvalidToken
		}

		// Revoke only if still unrevoked. If two requests race with the same
		// token, only one updates a row; the other sees 0 rows and is rejected.
		res := tx.Model(&models.RefreshToken{}).
			Where("id = ? AND revoked_at IS NULL", stored.ID).
			Update("revoked_at", time.Now())
		if res.Error != nil {
			return fmt.Errorf("revoke refresh token: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return ErrInvalidToken
		}

		var user models.User
		if err := tx.First(&user, stored.UserID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInvalidToken // user was deleted
			}
			return fmt.Errorf("find user: %w", err)
		}
		if !user.IsActive {
			return ErrInvalidToken
		}

		result, err = s.issueTokens(tx, &user)
		return err
	})

	if reused {
		// Outside the transaction above, which has already rolled back.
		if revokeErr := s.revokeAllForTokenOwner(ctx, refreshToken); revokeErr != nil {
			return nil, revokeErr
		}
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Logout revokes one refresh token. It succeeds even if the token is unknown
// or already revoked, so it can't be used to test whether tokens exist.
func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	err := s.db.WithContext(ctx).Model(&models.RefreshToken{}).
		Where("token_hash = ? AND revoked_at IS NULL", auth.HashRefreshToken(refreshToken)).
		Update("revoked_at", time.Now()).Error
	if err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}

// GetUser returns an active user by ID.
func (s *AuthService) GetUser(ctx context.Context, id uint) (*models.User, error) {
	var user models.User
	err := s.db.WithContext(ctx).Where("is_active").First(&user, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}
	return &user, nil
}

// issueTokens creates an access token and stores a new refresh token using db,
// which may be a transaction.
func (s *AuthService) issueTokens(db *gorm.DB, user *models.User) (*dto.TokenResponse, error) {
	access, err := s.tokens.IssueAccessToken(user.ID, string(user.Role))
	if err != nil {
		return nil, fmt.Errorf("issue access token: %w", err)
	}

	refresh, hash, err := auth.NewRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}
	stored := models.RefreshToken{
		UserID:    user.ID,
		TokenHash: hash,
		ExpiresAt: time.Now().Add(s.refreshTTL),
	}
	if err := db.Create(&stored).Error; err != nil {
		return nil, fmt.Errorf("store refresh token: %w", err)
	}

	return &dto.TokenResponse{
		AccessToken:  access,
		RefreshToken: refresh,
		TokenType:    "Bearer",
		ExpiresIn:    int(s.tokens.TTL().Seconds()),
		User:         dto.NewUserResponse(user),
	}, nil
}

func (s *AuthService) revokeAllForTokenOwner(ctx context.Context, refreshToken string) error {
	// time.Now(), not SQL now(): every timestamp in this table comes from the
	// app's clock. The database server's clock can differ slightly, and mixing
	// the two can make a token look revoked before it was created.
	err := s.db.WithContext(ctx).Exec(`
		UPDATE refresh_tokens SET revoked_at = ?
		WHERE revoked_at IS NULL
		  AND user_id = (SELECT user_id FROM refresh_tokens WHERE token_hash = ?)`,
		time.Now(), auth.HashRefreshToken(refreshToken)).Error
	if err != nil {
		return fmt.Errorf("revoke all sessions: %w", err)
	}
	return nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
