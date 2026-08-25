package rediskey

import (
	"strings"
	"testing"
)

func TestEmailKeysDoNotExposeAddress(t *testing.T) {
	email := "private@example.com"
	keys := []string{
		EmailCode("login", email),
		EmailCodeAttempts("login", email),
		EmailCodeCooldown("login", email),
	}
	for _, key := range keys {
		if strings.Contains(key, email) {
			t.Fatalf("Redis key %q exposes email address", key)
		}
	}
}

func TestDigestIsStableAndSeparatesValues(t *testing.T) {
	if Digest("same") != Digest("same") {
		t.Fatal("Digest is not stable")
	}
	if Digest("first") == Digest("second") {
		t.Fatal("Digest does not separate inputs")
	}
}
