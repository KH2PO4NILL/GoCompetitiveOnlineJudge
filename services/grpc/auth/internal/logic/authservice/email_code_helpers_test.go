package authservicelogic

import (
	"strings"
	"testing"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"
)

func TestNormalizeEmail(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
		ok    bool
	}{
		{name: "normalizes", input: " User@Example.COM ", want: "user@example.com", ok: true},
		{name: "rejects empty", input: " ", ok: false},
		{name: "rejects display name", input: "User <user@example.com>", ok: false},
		{name: "rejects malformed", input: "not-an-email", ok: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := normalizeEmail(test.input)
			if ok != test.ok || got != test.want {
				t.Fatalf("normalizeEmail(%q) = (%q, %v), want (%q, %v)", test.input, got, ok, test.want, test.ok)
			}
		})
	}
}

func TestEmailCodeKeysSeparatePurposes(t *testing.T) {
	email := "user@example.com"
	registerKey := emailCodeKey(emailCodePurposeRegister, email)
	loginKey := emailCodeKey(emailCodePurposeLogin, email)
	if registerKey == loginKey {
		t.Fatalf("register key %q equals login key", registerKey)
	}
	if strings.Contains(registerKey, email) || strings.Contains(loginKey, email) {
		t.Fatal("email code Redis key exposes the plaintext email address")
	}
}

func TestParseEmailCodePurpose(t *testing.T) {
	if purpose, ok := parseEmailCodePurpose(pb.EmailCodePurpose_EMAIL_CODE_PURPOSE_REGISTER); !ok || purpose != emailCodePurposeRegister {
		t.Fatalf("parseEmailCodePurpose(register) = (%q, %v)", purpose, ok)
	}
	if _, ok := parseEmailCodePurpose(pb.EmailCodePurpose_EMAIL_CODE_PURPOSE_UNSPECIFIED); ok {
		t.Fatal("parseEmailCodePurpose(unspecified) unexpectedly succeeded")
	}
}

func TestIsSixDigitCode(t *testing.T) {
	for _, value := range []string{"000000", "123456", "999999"} {
		if !isSixDigitCode(value) {
			t.Fatalf("isSixDigitCode(%q) = false", value)
		}
	}
	for _, value := range []string{"12345", "1234567", "12a456", "１２３４５６"} {
		if isSixDigitCode(value) {
			t.Fatalf("isSixDigitCode(%q) = true", value)
		}
	}
}
