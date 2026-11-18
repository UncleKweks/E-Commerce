package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "0123456789abcdef0123456789abcdef"

func TestPasswordHashing(t *testing.T) {
	hash, err := HashPassword("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "correct horse" {
		t.Fatal("hash must not equal the password")
	}
	if !CheckPassword(hash, "correct horse") {
		t.Error("correct password was rejected")
	}
	if CheckPassword(hash, "wrong horse") {
		t.Error("wrong password was accepted")
	}

	again, _ := HashPassword("correct horse")
	if again == hash {
		t.Error("same password hashed twice should differ because of the salt")
	}
}

func TestPasswordTooLong(t *testing.T) {
	if _, err := HashPassword(strings.Repeat("a", MaxPasswordBytes+1)); err != ErrPasswordTooLong {
		t.Fatalf("got %v, want ErrPasswordTooLong", err)
	}
}

func TestAccessTokenRoundTrip(t *testing.T) {
	m := NewTokenManager(testSecret, time.Minute)
	token, err := m.IssueAccessToken(42, "admin")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := m.ParseAccessToken(token)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := claims.UserID()
	if id != 42 || claims.Role != "admin" {
		t.Errorf("got user %d role %q", id, claims.Role)
	}
}

func TestAccessTokenRejected(t *testing.T) {
	m := NewTokenManager(testSecret, time.Minute)
	valid, _ := m.IssueAccessToken(1, "customer")

	expired, _ := NewTokenManager(testSecret, -time.Minute).IssueAccessToken(1, "customer")
	otherSecret, _ := NewTokenManager("ffffffffffffffffffffffffffffffff", time.Minute).IssueAccessToken(1, "customer")

	// A token with no signature, claiming to be an admin.
	unsigned, _ := jwt.NewWithClaims(jwt.SigningMethodNone, Claims{
		Role: "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: "1", Issuer: issuer, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}).SignedString(jwt.UnsafeAllowNoneSignatureType)

	// Change one character of the payload (the middle part) of a valid token.
	parts := strings.Split(valid, ".")
	parts[1] = parts[1][:len(parts[1])-1] + flip(parts[1][len(parts[1])-1])
	tampered := strings.Join(parts, ".")

	cases := map[string]string{
		"expired":      expired,
		"wrong secret": otherSecret,
		"alg none":     unsigned,
		"tampered":     tampered,
		"garbage":      "not.a.token",
		"empty":        "",
	}
	for name, token := range cases {
		if _, err := m.ParseAccessToken(token); err != ErrInvalidToken {
			t.Errorf("%s: got %v, want ErrInvalidToken", name, err)
		}
	}
}

func flip(c byte) string {
	if c == 'A' {
		return "B"
	}
	return "A"
}

func TestRefreshToken(t *testing.T) {
	token, hash, err := NewRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	if token == hash {
		t.Fatal("stored hash must differ from the token")
	}
	if HashRefreshToken(token) != hash {
		t.Error("hashing the same token must give the same hash")
	}
	other, _, _ := NewRefreshToken()
	if other == token {
		t.Error("two refresh tokens should never match")
	}
}
