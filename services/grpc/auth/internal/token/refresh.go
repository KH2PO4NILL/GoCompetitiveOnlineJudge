package token

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
)

const refreshTokenEntropyBytes = 32

// GenerateRefreshToken returns an opaque refresh token and the SHA-256 hash
// that should be persisted in user_sessions. Never persist the plaintext token.
func GenerateRefreshToken() (plainText string, hash string, err error) {
	plainText, err = randomURLSafeString(refreshTokenEntropyBytes)
	if err != nil {
		return "", "", err
	}

	return plainText, HashRefreshToken(plainText), nil
}

// HashRefreshToken returns a lowercase hexadecimal SHA-256 digest compatible
// with the CHAR(64) refresh_token_hash column.
func HashRefreshToken(raw string) string {
	digest := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(digest[:])
}

// VerifyRefreshToken compares a plaintext refresh token with a stored hash in
// constant time after validating the stored hash format.
func VerifyRefreshToken(raw, storedHash string) bool {
	if raw == "" {
		return false
	}

	expected, err := hex.DecodeString(storedHash)
	if err != nil || len(expected) != sha256.Size {
		return false
	}
	actual := sha256.Sum256([]byte(raw))

	return subtle.ConstantTimeCompare(actual[:], expected) == 1
}

func randomURLSafeString(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
