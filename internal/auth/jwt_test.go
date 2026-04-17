package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestGenerateTokenPair(t *testing.T) {
	svc := NewJWTService("test-secret-key", 60, 168)

	pair, err := svc.GenerateTokenPair(1, "admin")
	if err != nil {
		t.Fatalf("GenerateTokenPair: %v", err)
	}

	if pair.AccessToken == "" {
		t.Fatal("access token is empty")
	}
	if pair.RefreshToken == "" {
		t.Fatal("refresh token is empty")
	}
	if pair.AccessToken == pair.RefreshToken {
		t.Fatal("access and refresh tokens should be different")
	}
}

func TestValidateAccessToken(t *testing.T) {
	svc := NewJWTService("test-secret-key", 60, 168)

	pair, _ := svc.GenerateTokenPair(1, "admin")

	claims, err := svc.ValidateAccessToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("ValidateAccessToken: %v", err)
	}

	if int(claims["user_id"].(float64)) != 1 {
		t.Errorf("expected user_id 1, got %v", claims["user_id"])
	}
	if claims["username"].(string) != "admin" {
		t.Errorf("expected username admin, got %v", claims["username"])
	}
}

func TestValidateAccessToken_RejectsRefreshToken(t *testing.T) {
	svc := NewJWTService("test-secret-key", 60, 168)

	pair, _ := svc.GenerateTokenPair(1, "admin")

	_, err := svc.ValidateAccessToken(pair.RefreshToken)
	if err == nil {
		t.Fatal("expected error when using refresh token as access token")
	}
}

func TestValidateRefreshToken(t *testing.T) {
	svc := NewJWTService("test-secret-key", 60, 168)

	pair, _ := svc.GenerateTokenPair(1, "admin")

	claims, err := svc.ValidateRefreshToken(pair.RefreshToken)
	if err != nil {
		t.Fatalf("ValidateRefreshToken: %v", err)
	}

	if int(claims["user_id"].(float64)) != 1 {
		t.Errorf("expected user_id 1, got %v", claims["user_id"])
	}
	if claims["token_type"].(string) != "refresh" {
		t.Error("expected token_type refresh")
	}
}

func TestValidateRefreshToken_RejectsAccessToken(t *testing.T) {
	svc := NewJWTService("test-secret-key", 60, 168)

	pair, _ := svc.GenerateTokenPair(1, "admin")

	_, err := svc.ValidateRefreshToken(pair.AccessToken)
	if err == nil {
		t.Fatal("expected error when using access token as refresh token")
	}
}

func TestValidateAccessToken_ExpiredToken(t *testing.T) {
	svc := NewJWTService("test-secret-key", 60, 168)

	// Create an expired token manually
	claims := jwt.MapClaims{
		"user_id":  1,
		"username": "admin",
		"exp":      time.Now().Add(-1 * time.Hour).Unix(),
		"iat":      time.Now().Add(-2 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte("test-secret-key"))

	_, err := svc.ValidateAccessToken(tokenStr)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestValidateAccessToken_TamperedToken(t *testing.T) {
	svc := NewJWTService("test-secret-key", 60, 168)

	pair, _ := svc.GenerateTokenPair(1, "admin")

	// Tamper with the token
	tampered := pair.AccessToken + "x"

	_, err := svc.ValidateAccessToken(tampered)
	if err == nil {
		t.Fatal("expected error for tampered token")
	}
}

func TestValidateAccessToken_WrongSecret(t *testing.T) {
	svc1 := NewJWTService("secret-1", 60, 168)
	svc2 := NewJWTService("secret-2", 60, 168)

	pair, _ := svc1.GenerateTokenPair(1, "admin")

	_, err := svc2.ValidateAccessToken(pair.AccessToken)
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
}
