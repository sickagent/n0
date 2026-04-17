package jwt

import (
	"crypto/subtle"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims represents standard + custom JWT claims.
type Claims struct {
	jwt.RegisteredClaims
	UserID string `json:"user_id"`
	Email  string `json:"email,omitempty"`
	Type   string `json:"type"` // "user" or "agent"
}

// Manager handles JWT creation and verification.
type Manager struct {
	secret []byte
	issuer string
	expiry time.Duration
}

// NewManager creates a new JWT manager.
func NewManager(secret []byte, issuer string, expiry time.Duration) *Manager {
	return &Manager{
		secret: secret,
		issuer: issuer,
		expiry: expiry,
	}
}

// GenerateUserToken creates an access token for a human user.
func (m *Manager) GenerateUserToken(userID string, email string) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    m.issuer,
			Audience:  jwt.ClaimStrings{"n0-platform"},
			ExpiresAt: jwt.NewNumericDate(now.Add(m.expiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        uuid.Must(uuid.NewRandom()).String(),
		},
		UserID: userID,
		Email:  email,
		Type:   "user",
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

// GenerateAgentToken creates an access token for an agent.
func (m *Manager) GenerateAgentToken(agentID string, userID string) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   agentID,
			Issuer:    m.issuer,
			Audience:  jwt.ClaimStrings{"n0-platform"},
			ExpiresAt: jwt.NewNumericDate(now.Add(m.expiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        uuid.Must(uuid.NewRandom()).String(),
		},
		UserID: userID,
		Type:   "agent",
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

// Verify parses and validates a token string.
func (m *Manager) Verify(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}

// ConstantTimeCompare compares two strings in constant time.
func ConstantTimeCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
