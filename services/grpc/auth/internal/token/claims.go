package token

import (
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

const AccessTokenType = "access"

// AccessTokenInput contains the application data copied into an access token.
// Keep this type independent from protobuf and database models so the token
// package can be used without introducing transport or persistence coupling.
type AccessTokenInput struct {
	UserID    int64
	Role      int32
	Status    int32
	SchoolID  int64
	SessionID int64
}

// AccessClaims is the signed payload of an access token.
type AccessClaims struct {
	UserID    int64  `json:"uid"`
	Role      int32  `json:"role"`
	Status    int32  `json:"status"`
	SchoolID  int64  `json:"school_id"`
	SessionID int64  `json:"sid"`
	TokenType string `json:"token_type"`

	jwt.RegisteredClaims
}

// Validate implements jwt.ClaimsValidator and validates application claims in
// addition to the standard issuer, audience and time-based JWT claims.
func (c AccessClaims) Validate() error {
	if c.TokenType != AccessTokenType {
		return fmt.Errorf("%w: unexpected token type", ErrTokenInvalid)
	}
	if c.UserID <= 0 {
		return fmt.Errorf("%w: user id must be positive", ErrTokenInvalid)
	}
	if c.SessionID <= 0 {
		return fmt.Errorf("%w: session id must be positive", ErrTokenInvalid)
	}

	return nil
}
