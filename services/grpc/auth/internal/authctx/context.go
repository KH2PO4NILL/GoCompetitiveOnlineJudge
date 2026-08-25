package authctx

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/token"
)

type claimsContextKey struct{}

// WithClaims returns a child context containing validated access-token claims.
// Only authentication middleware should call this function.
func WithClaims(ctx context.Context, claims *token.AccessClaims) context.Context {
	return context.WithValue(ctx, claimsContextKey{}, claims)
}

// ClaimsFromContext returns claims previously validated by the auth interceptor.
func ClaimsFromContext(ctx context.Context) (*token.AccessClaims, bool) {
	claims, ok := ctx.Value(claimsContextKey{}).(*token.AccessClaims)
	return claims, ok && claims != nil
}
