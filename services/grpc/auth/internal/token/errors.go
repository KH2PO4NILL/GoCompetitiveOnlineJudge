package token

import "errors"

var (
	ErrInvalidConfig  = errors.New("invalid token configuration")
	ErrTokenExpired   = errors.New("token expired")
	ErrTokenInvalid   = errors.New("token invalid")
	ErrTokenMalformed = errors.New("token malformed")
	ErrTokenNotActive = errors.New("token not active")
	ErrTokenSigning   = errors.New("token signing failed")
)
