package authctx

import (
	"context"
	"testing"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/token"
)

func TestClaimsFromContext(t *testing.T) {
	if _, ok := ClaimsFromContext(context.Background()); ok {
		t.Fatal("ClaimsFromContext() unexpectedly found claims")
	}

	want := &token.AccessClaims{UserID: 42, SessionID: 7}
	ctx := WithClaims(context.Background(), want)
	got, ok := ClaimsFromContext(ctx)
	if !ok {
		t.Fatal("ClaimsFromContext() did not find claims")
	}
	if got != want {
		t.Fatalf("ClaimsFromContext() = %p, want %p", got, want)
	}
}
