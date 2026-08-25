package interceptor

import (
	"context"
	"errors"
	"testing"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/authctx"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/token"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type stubTokenParser struct {
	claims   *token.AccessClaims
	err      error
	received string
}

func (s *stubTokenParser) ParseAccessToken(raw string) (*token.AccessClaims, error) {
	s.received = raw
	return s.claims, s.err
}

func TestAuthInterceptorAllowsPublicMethod(t *testing.T) {
	parser := &stubTokenParser{err: errors.New("must not be called")}
	interceptor := newAuthInterceptor(parser, func(context.Context, *token.AccessClaims) error {
		return errors.New("must not be called")
	})

	called := false
	_, err := interceptor(
		context.Background(),
		&pb.EmailCodeLoginReq{},
		&grpc.UnaryServerInfo{FullMethod: pb.AuthService_EmailCodeLogin_FullMethodName},
		func(context.Context, any) (any, error) {
			called = true
			return "ok", nil
		},
	)
	if err != nil {
		t.Fatalf("interceptor() error = %v", err)
	}
	if !called {
		t.Fatal("interceptor() did not call handler")
	}
}

func TestAuthInterceptorProtectsChangeEmailCode(t *testing.T) {
	interceptor := newAuthInterceptor(&stubTokenParser{}, func(context.Context, *token.AccessClaims) error {
		return nil
	})

	_, err := interceptor(
		context.Background(),
		&pb.SendEmailCodeReq{Purpose: pb.EmailCodePurpose_EMAIL_CODE_PURPOSE_CHANGE_EMAIL},
		&grpc.UnaryServerInfo{FullMethod: pb.AuthService_SendEmailCode_FullMethodName},
		func(context.Context, any) (any, error) { return "ok", nil },
	)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("interceptor() code = %v, want %v", status.Code(err), codes.Unauthenticated)
	}
}

func TestAuthInterceptorAllowsLogoutWithRefreshTokenAfterAccessExpiry(t *testing.T) {
	parser := &stubTokenParser{err: token.ErrTokenExpired}
	interceptor := newAuthInterceptor(parser, func(context.Context, *token.AccessClaims) error {
		return errors.New("must not validate an expired access token")
	})

	called := false
	_, err := interceptor(
		bearerContext("expired"),
		&pb.LogoutReq{RefreshToken: "valid-refresh-token"},
		&grpc.UnaryServerInfo{FullMethod: pb.AuthService_Logout_FullMethodName},
		func(ctx context.Context, _ any) (any, error) {
			called = true
			if _, ok := authctx.ClaimsFromContext(ctx); ok {
				t.Fatal("expired access token unexpectedly injected claims")
			}
			return "ok", nil
		},
	)
	if err != nil {
		t.Fatalf("interceptor() error = %v", err)
	}
	if !called {
		t.Fatal("interceptor() did not call logout handler")
	}
}

func TestAuthInterceptorRejectsLogoutWithoutUsableToken(t *testing.T) {
	interceptor := newAuthInterceptor(&stubTokenParser{}, func(context.Context, *token.AccessClaims) error {
		return nil
	})
	_, err := interceptor(
		context.Background(),
		&pb.LogoutReq{},
		&grpc.UnaryServerInfo{FullMethod: pb.AuthService_Logout_FullMethodName},
		func(context.Context, any) (any, error) { return "ok", nil },
	)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("interceptor() code = %v, want %v", status.Code(err), codes.Unauthenticated)
	}
}

func TestAuthInterceptorRejectsMalformedAuthorization(t *testing.T) {
	tests := []struct {
		name   string
		values []string
	}{
		{name: "missing"},
		{name: "wrong scheme", values: []string{"Basic abc"}},
		{name: "missing token", values: []string{"Bearer"}},
		{name: "multiple", values: []string{"Bearer one", "Bearer two"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := metadata.NewIncomingContext(
				context.Background(),
				metadata.MD{"authorization": test.values},
			)
			interceptor := newAuthInterceptor(&stubTokenParser{}, func(context.Context, *token.AccessClaims) error {
				return nil
			})

			_, err := interceptor(
				ctx,
				nil,
				&grpc.UnaryServerInfo{FullMethod: pb.UserService_GetCurrentUser_FullMethodName},
				func(context.Context, any) (any, error) { return "ok", nil },
			)
			if status.Code(err) != codes.Unauthenticated {
				t.Fatalf("interceptor() code = %v, want %v", status.Code(err), codes.Unauthenticated)
			}
		})
	}
}

func TestAuthInterceptorRejectsInvalidToken(t *testing.T) {
	parser := &stubTokenParser{err: token.ErrTokenExpired}
	interceptor := newAuthInterceptor(parser, func(context.Context, *token.AccessClaims) error {
		return nil
	})
	ctx := bearerContext("expired")

	_, err := interceptor(
		ctx,
		nil,
		&grpc.UnaryServerInfo{FullMethod: pb.UserService_GetCurrentUser_FullMethodName},
		func(context.Context, any) (any, error) { return "ok", nil },
	)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("interceptor() code = %v, want %v", status.Code(err), codes.Unauthenticated)
	}
}

func TestAuthInterceptorRejectsInvalidSession(t *testing.T) {
	claims := &token.AccessClaims{UserID: 42, SessionID: 7}
	interceptor := newAuthInterceptor(&stubTokenParser{claims: claims}, func(context.Context, *token.AccessClaims) error {
		return errSessionInvalid
	})

	_, err := interceptor(
		bearerContext("valid"),
		nil,
		&grpc.UnaryServerInfo{FullMethod: pb.UserService_GetCurrentUser_FullMethodName},
		func(context.Context, any) (any, error) { return "ok", nil },
	)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("interceptor() code = %v, want %v", status.Code(err), codes.Unauthenticated)
	}
}

func TestAuthInterceptorReportsSessionBackendFailure(t *testing.T) {
	claims := &token.AccessClaims{UserID: 42, SessionID: 7}
	interceptor := newAuthInterceptor(&stubTokenParser{claims: claims}, func(context.Context, *token.AccessClaims) error {
		return errors.New("database unavailable")
	})

	_, err := interceptor(
		bearerContext("valid"),
		nil,
		&grpc.UnaryServerInfo{FullMethod: pb.UserService_GetCurrentUser_FullMethodName},
		func(context.Context, any) (any, error) { return "ok", nil },
	)
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("interceptor() code = %v, want %v", status.Code(err), codes.Unavailable)
	}
}

func TestAuthInterceptorInjectsClaims(t *testing.T) {
	claims := &token.AccessClaims{UserID: 42, SessionID: 7}
	parser := &stubTokenParser{claims: claims}
	interceptor := newAuthInterceptor(parser, func(_ context.Context, got *token.AccessClaims) error {
		if got != claims {
			t.Fatalf("session validator claims = %p, want %p", got, claims)
		}
		return nil
	})

	called := false
	response, err := interceptor(
		bearerContext("valid-token"),
		nil,
		&grpc.UnaryServerInfo{FullMethod: pb.UserService_GetCurrentUser_FullMethodName},
		func(ctx context.Context, _ any) (any, error) {
			called = true
			got, ok := authctx.ClaimsFromContext(ctx)
			if !ok || got != claims {
				t.Fatal("handler context does not contain validated claims")
			}
			return "ok", nil
		},
	)
	if err != nil {
		t.Fatalf("interceptor() error = %v", err)
	}
	if !called || response != "ok" {
		t.Fatalf("interceptor() response = %v, called = %v", response, called)
	}
	if parser.received != "valid-token" {
		t.Fatalf("ParseAccessToken() raw = %q, want %q", parser.received, "valid-token")
	}
}

func bearerContext(raw string) context.Context {
	return metadata.NewIncomingContext(
		context.Background(),
		metadata.Pairs("authorization", "Bearer "+raw),
	)
}
