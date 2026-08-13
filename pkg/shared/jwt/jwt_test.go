package jwt

import (
	"testing"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

func TestVerifyEnforcesIssuerAndAudience(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	manager := NewManager(secret, "n0-gateway", time.Hour)

	valid, err := manager.GenerateUserToken("user-1", "user@example.com")
	if err != nil {
		t.Fatalf("generate valid token: %v", err)
	}
	if _, err := manager.Verify(valid); err != nil {
		t.Fatalf("verify valid token: %v", err)
	}

	now := time.Now().UTC()
	wrongAudience := Claims{
		RegisteredClaims: jwtlib.RegisteredClaims{
			Subject: "user-1", Issuer: "n0-gateway", Audience: jwtlib.ClaimStrings{"other"},
			ExpiresAt: jwtlib.NewNumericDate(now.Add(time.Hour)), IssuedAt: jwtlib.NewNumericDate(now),
		},
		UserID: "user-1",
		Type:   "user",
	}
	token, err := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, wrongAudience).SignedString(secret)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	if _, err := manager.Verify(token); err == nil {
		t.Fatal("expected token with wrong audience to be rejected")
	}
}
