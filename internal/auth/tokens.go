package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const issuer = "ecommerce-api"

// ErrInvalidToken covers every reason a token is rejected: bad signature,
// expired, malformed. Callers don't need to know which.
var ErrInvalidToken = errors.New("invalid or expired token")

// Claims is what an access token says about its holder.
type Claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// UserID returns the user ID stored in the token's subject.
func (c *Claims) UserID() (uint, error) {
	id, err := strconv.ParseUint(c.Subject, 10, 64)
	if err != nil {
		return 0, ErrInvalidToken
	}
	return uint(id), nil
}

// TokenManager creates and verifies access tokens.
type TokenManager struct {
	secret []byte
	ttl    time.Duration
}

// NewTokenManager returns a TokenManager that signs with secret and issues
// tokens valid for ttl.
func NewTokenManager(secret string, ttl time.Duration) *TokenManager {
	return &TokenManager{secret: []byte(secret), ttl: ttl}
}

// TTL is how long issued access tokens are valid.
func (m *TokenManager) TTL() time.Duration { return m.ttl }

// IssueAccessToken returns a signed JWT for the user.
func (m *TokenManager) IssueAccessToken(userID uint, role string) (string, error) {
	now := time.Now()
	claims := Claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatUint(uint64(userID), 10),
			Issuer:    issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

// ParseAccessToken verifies the token's signature, expiry and issuer and returns its claims.
func (m *TokenManager) ParseAccessToken(tokenString string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenString, claims,
		func(*jwt.Token) (any, error) { return m.secret, nil },
		// Only accept the algorithm we sign with. Without this, an attacker can
		// send a token claiming "alg: none" (no signature) and some libraries accept it.
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// NewRefreshToken returns a random token to give the client and the hash to
// store in the database. 32 random bytes can't be guessed, so a fast hash
// (SHA-256) is safe here, unlike passwords, which need bcrypt.
func NewRefreshToken() (token, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, HashRefreshToken(token), nil
}

// HashRefreshToken returns the value stored in refresh_tokens.token_hash.
func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
