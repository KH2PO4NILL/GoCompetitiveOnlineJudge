package token

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestGenerateRefreshToken(t *testing.T) {
	plainText, hash, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken() error = %v", err)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(plainText)
	if err != nil {
		t.Fatalf("GenerateRefreshToken() token is not base64url: %v", err)
	}
	if len(decoded) != refreshTokenEntropyBytes {
		t.Fatalf("GenerateRefreshToken() entropy bytes = %d, want %d", len(decoded), refreshTokenEntropyBytes)
	}
	if len(hash) != 64 {
		t.Fatalf("GenerateRefreshToken() hash length = %d, want 64", len(hash))
	}
	if !VerifyRefreshToken(plainText, hash) {
		t.Fatal("VerifyRefreshToken() rejected the generated token")
	}
}

func TestGenerateRefreshTokenIsUnique(t *testing.T) {
	first, _, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("first GenerateRefreshToken() error = %v", err)
	}
	second, _, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("second GenerateRefreshToken() error = %v", err)
	}
	if first == second {
		t.Fatal("GenerateRefreshToken() returned duplicate tokens")
	}
}

func TestVerifyRefreshTokenRejectsInvalidValues(t *testing.T) {
	raw := "refresh-token"
	hash := HashRefreshToken(raw)
	tests := []struct {
		plainText string
		hash      string
	}{
		{"", hash},
		{"other-token", hash},
		{raw, "not-hex"},
		{raw, strings.Repeat("0", 62)},
	}

	for _, test := range tests {
		if VerifyRefreshToken(test.plainText, test.hash) {
			t.Fatalf("VerifyRefreshToken(%q, %q) = true, want false", test.plainText, test.hash)
		}
	}
}
