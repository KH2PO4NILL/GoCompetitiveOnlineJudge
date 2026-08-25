package token

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const minimumHMACSecretBytes = 32

type ManagerConfig struct {
	Secret    string
	Issuer    string
	Audience  string
	AccessTTL time.Duration
	Leeway    time.Duration
}

// Manager signs and validates access JWTs. A Manager is safe for concurrent
// use after construction.
type Manager struct {
	secret    []byte
	issuer    string
	audience  string
	accessTTL time.Duration
	parser    *jwt.Parser
	now       func() time.Time
}

func NewManager(config ManagerConfig) (*Manager, error) {
	if len(config.Secret) < minimumHMACSecretBytes {
		return nil, fmt.Errorf("%w: secret must contain at least %d bytes", ErrInvalidConfig, minimumHMACSecretBytes)
	}
	if strings.TrimSpace(config.Issuer) == "" {
		return nil, fmt.Errorf("%w: issuer is required", ErrInvalidConfig)
	}
	if strings.TrimSpace(config.Audience) == "" {
		return nil, fmt.Errorf("%w: audience is required", ErrInvalidConfig)
	}
	if config.AccessTTL <= 0 {
		return nil, fmt.Errorf("%w: access TTL must be positive", ErrInvalidConfig)
	}
	if config.Leeway < 0 {
		return nil, fmt.Errorf("%w: leeway cannot be negative", ErrInvalidConfig)
	}

	secret := append([]byte(nil), config.Secret...)
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(config.Issuer),
		jwt.WithAudience(config.Audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(config.Leeway),
	)

	return &Manager{
		secret:    secret,
		issuer:    config.Issuer,
		audience:  config.Audience,
		accessTTL: config.AccessTTL,
		parser:    parser,
		now:       time.Now,
	}, nil
}

// GenerateAccessToken creates a short-lived HS256 access token. The returned
// expiration time can be used to populate expires_in in the RPC response.
func (m *Manager) GenerateAccessToken(input AccessTokenInput) (string, time.Time, error) {
	issuedAt := m.now().UTC()
	expiresAt := issuedAt.Add(m.accessTTL)
	tokenID, err := randomURLSafeString(16)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("%w: generate token id: %v", ErrTokenSigning, err)
	}

	claims := AccessClaims{
		UserID:    input.UserID,
		Role:      input.Role,
		Status:    input.Status,
		SchoolID:  input.SchoolID,
		SessionID: input.SessionID,
		TokenType: AccessTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   strconv.FormatInt(input.UserID, 10),
			Audience:  jwt.ClaimStrings{m.audience},
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			NotBefore: jwt.NewNumericDate(issuedAt),
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			ID:        tokenID,
		},
	}
	if err := claims.Validate(); err != nil {
		return "", time.Time{}, err
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("%w: %v", ErrTokenSigning, err)
	}

	return signed, expiresAt, nil
}

// ParseAccessToken verifies the signature, algorithm, issuer, audience,
// expiration and application-specific access claims.
func (m *Manager) ParseAccessToken(raw string) (*AccessClaims, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, ErrTokenMalformed
	}

	claims := new(AccessClaims)
	parsed, err := m.parser.ParseWithClaims(raw, claims, func(parsed *jwt.Token) (any, error) {
		if parsed.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, ErrTokenInvalid
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, classifyParseError(err)
	}
	if parsed == nil || !parsed.Valid {
		return nil, ErrTokenInvalid
	}

	return claims, nil
}

func classifyParseError(err error) error {
	category := ErrTokenInvalid
	switch {
	case errors.Is(err, jwt.ErrTokenExpired):
		category = ErrTokenExpired
	case errors.Is(err, jwt.ErrTokenNotValidYet):
		category = ErrTokenNotActive
	case errors.Is(err, jwt.ErrTokenMalformed):
		category = ErrTokenMalformed
	}

	return fmt.Errorf("%w: %v", category, err)
}
