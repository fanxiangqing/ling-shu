package auth

import (
	"testing"
	"time"
)

func TestTokenManagerGenerateAndParse(t *testing.T) {
	manager := NewTokenManager("secret", time.Hour)
	token, expiresAt, err := manager.Generate(7, "alice")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	if token == "" || expiresAt.IsZero() {
		t.Fatalf("expected token and expiry")
	}
	claims, err := manager.Parse(token)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if claims.UserID != 7 || claims.Username != "alice" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestTokenManagerRejectsExpiredToken(t *testing.T) {
	manager := NewTokenManager("secret", time.Nanosecond)
	token, _, err := manager.Generate(7, "alice")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	time.Sleep(time.Millisecond)
	if _, err := manager.Parse(token); err != ErrExpiredToken {
		t.Fatalf("expected expired token, got %v", err)
	}
}
