package token

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	testIssuer   = "auth.rpc"
	testAudience = "go-competitive-online-judge"
)

func TestManagerRoundTrip(t *testing.T) {
	manager := newTestManager(t, strings.Repeat("a", minimumHMACSecretBytes))
	input := AccessTokenInput{
		UserID:    42,
		Role:      2,
		Status:    1,
		SchoolID:  7,
		SessionID: 99,
	}

	raw, expiresAt, err := manager.GenerateAccessToken(input)
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}
	if raw == "" {
		t.Fatal("GenerateAccessToken() returned an empty token")
	}
	if !expiresAt.After(time.Now()) {
		t.Fatalf("GenerateAccessToken() expiration = %v, want future time", expiresAt)
	}

	claims, err := manager.ParseAccessToken(raw)
	if err != nil {
		t.Fatalf("ParseAccessToken() error = %v", err)
	}
	if claims.UserID != input.UserID || claims.SessionID != input.SessionID {
		t.Fatalf("ParseAccessToken() claims = %+v, want user %d and session %d", claims, input.UserID, input.SessionID)
	}
	if claims.TokenType != AccessTokenType || claims.ID == "" {
		t.Fatalf("ParseAccessToken() token type or ID invalid: %+v", claims)
	}
}

func TestManagerRejectsWrongSecret(t *testing.T) {
	issuer := newTestManager(t, strings.Repeat("a", minimumHMACSecretBytes))
	verifier := newTestManager(t, strings.Repeat("b", minimumHMACSecretBytes))
	raw, _, err := issuer.GenerateAccessToken(AccessTokenInput{UserID: 1, SessionID: 1})
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}

	_, err = verifier.ParseAccessToken(raw)
	if !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("ParseAccessToken() error = %v, want ErrTokenInvalid", err)
	}
}

func TestManagerRejectsExpiredToken(t *testing.T) {
	secret := strings.Repeat("a", minimumHMACSecretBytes)
	manager := newTestManager(t, secret)
	now := time.Now().UTC()
	claims := AccessClaims{
		UserID:    1,
		SessionID: 1,
		TokenType: AccessTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    testIssuer,
			Audience:  jwt.ClaimStrings{testAudience},
			ExpiresAt: jwt.NewNumericDate(now.Add(-time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Minute)),
		},
	}
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}

	_, err = manager.ParseAccessToken(raw)
	if !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("ParseAccessToken() error = %v, want ErrTokenExpired", err)
	}
}

func TestManagerRejectsUnexpectedSigningMethod(t *testing.T) {
	secret := strings.Repeat("a", minimumHMACSecretBytes)
	manager := newTestManager(t, secret)
	now := time.Now().UTC()
	claims := AccessClaims{
		UserID:    1,
		SessionID: 1,
		TokenType: AccessTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    testIssuer,
			Audience:  jwt.ClaimStrings{testAudience},
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS384, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}

	_, err = manager.ParseAccessToken(raw)
	if !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("ParseAccessToken() error = %v, want ErrTokenInvalid", err)
	}
}

func TestNewManagerValidatesConfig(t *testing.T) {
	tests := []ManagerConfig{
		{Secret: "short", Issuer: testIssuer, Audience: testAudience, AccessTTL: time.Minute},
		{Secret: strings.Repeat("a", minimumHMACSecretBytes), Audience: testAudience, AccessTTL: time.Minute},
		{Secret: strings.Repeat("a", minimumHMACSecretBytes), Issuer: testIssuer, AccessTTL: time.Minute},
		{Secret: strings.Repeat("a", minimumHMACSecretBytes), Issuer: testIssuer, Audience: testAudience},
		{Secret: strings.Repeat("a", minimumHMACSecretBytes), Issuer: testIssuer, Audience: testAudience, AccessTTL: time.Minute, Leeway: -time.Second},
	}

	for _, config := range tests {
		if _, err := NewManager(config); !errors.Is(err, ErrInvalidConfig) {
			t.Fatalf("NewManager(%+v) error = %v, want ErrInvalidConfig", config, err)
		}
	}
}

func newTestManager(t *testing.T, secret string) *Manager {
	t.Helper()
	manager, err := NewManager(ManagerConfig{
		Secret:    secret,
		Issuer:    testIssuer,
		Audience:  testAudience,
		AccessTTL: 15 * time.Minute,
		Leeway:    time.Second,
	})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	return manager
}
