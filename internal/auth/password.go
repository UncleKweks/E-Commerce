// Package auth holds the security building blocks: password hashing,
// access tokens (JWTs) and refresh tokens. It knows nothing about HTTP or the database.
package auth

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

// bcryptCost sets how slow hashing is. Each +1 doubles the work.
// 12 takes roughly 0.2-0.3s: unnoticeable at login, crippling for an attacker
// trying billions of guesses against a stolen database.
const bcryptCost = 12

// MaxPasswordBytes is bcrypt's hard limit; it can't hash anything longer.
const MaxPasswordBytes = 72

// ErrPasswordTooLong is returned for passwords over MaxPasswordBytes.
var ErrPasswordTooLong = errors.New("password is longer than 72 bytes")

// HashPassword returns a bcrypt hash of password. The hash includes a random
// salt, so hashing the same password twice gives two different results.
func HashPassword(password string) (string, error) {
	if len(password) > MaxPasswordBytes {
		return "", ErrPasswordTooLong
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword reports whether password matches hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// dummyHash is checked when a login email doesn't exist, so that request
// takes as long as a real one. Otherwise attackers could time responses
// to discover which emails have accounts.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password-for-timing"), bcryptCost)

// SpendPasswordCheckTime does the same work as CheckPassword and discards the result.
func SpendPasswordCheckTime(password string) {
	_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
}
