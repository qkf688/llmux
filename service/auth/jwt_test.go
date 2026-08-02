package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestSignAndParse(t *testing.T) {
	secret := "test-secret"
	token, err := Sign(secret, 1, "admin")
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if token == "" {
		t.Fatal("token is empty")
	}

	claims, err := Parse(secret, token)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if claims.UserID != 1 {
		t.Errorf("UserID = %d, want 1", claims.UserID)
	}
	if claims.Username != "admin" {
		t.Errorf("Username = %q, want admin", claims.Username)
	}
}

func TestParseWrongSecret(t *testing.T) {
	token, _ := Sign("secret-a", 1, "admin")
	if _, err := Parse("secret-b", token); err == nil {
		t.Error("Parse with wrong secret should fail")
	}
}

func TestParseExpiredToken(t *testing.T) {
	claims := Claims{
		UserID:   1,
		Username: "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte("test-secret"))

	if _, err := Parse("test-secret", tokenString); err == nil {
		t.Error("Parse expired token should fail")
	}
}

func TestParseGarbageToken(t *testing.T) {
	if _, err := Parse("test-secret", "not.a.token"); err == nil {
		t.Error("Parse garbage token should fail")
	}
}
