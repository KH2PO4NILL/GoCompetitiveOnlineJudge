package interceptor

import (
	"context"
	"strings"
	"testing"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type stubAccessTokenValidator struct {
	response *pb.ValidateTokenResp
	err      error
	calls    int
	token    string
}

func (s *stubAccessTokenValidator) ValidateToken(
	_ context.Context,
	in *pb.ValidateTokenReq,
	_ ...grpc.CallOption,
) (*pb.ValidateTokenResp, error) {
	s.calls++
	s.token = in.AccessToken
	return s.response, s.err
}

func TestAuthInterceptorInjectsValidatedPrincipal(t *testing.T) {
	validator := &stubAccessTokenValidator{response: &pb.ValidateTokenResp{
		Claims: &pb.AuthClaims{
			UserId: 42, Role: pb.UserRole_USER_ROLE_ADMIN,
			Status: pb.UserStatus_USER_STATUS_NORMAL, SchoolId: 7,
		},
	}}
	interceptor := NewAuthInterceptor(validator)
	ctx := metadata.NewIncomingContext(
		context.Background(),
		metadata.Pairs("authorization", "Bearer access-token"),
	)

	response, err := interceptor(ctx, "request", &grpc.UnaryServerInfo{FullMethod: "/problem.ProblemService/UpdateProblem"}, func(ctx context.Context, req any) (any, error) {
		principal, ok := PrincipalFromContext(ctx)
		if !ok {
			t.Fatal("PrincipalFromContext() did not find an identity")
		}
		if principal.UserID != 42 || principal.Role != pb.UserRole_USER_ROLE_ADMIN || principal.SchoolID != 7 {
			t.Fatalf("PrincipalFromContext() = %+v", principal)
		}
		return req, nil
	})
	if err != nil {
		t.Fatalf("interceptor() error = %v", err)
	}
	if response != "request" {
		t.Fatalf("interceptor() response = %v, want request", response)
	}
	if validator.calls != 1 || validator.token != "access-token" {
		t.Fatalf("ValidateToken() calls = %d, token = %q", validator.calls, validator.token)
	}
}

func TestAuthInterceptorAuthenticationModes(t *testing.T) {
	tests := []struct {
		name       string
		mode       AuthenticationMode
		metadata   metadata.MD
		wantCode   codes.Code
		wantCalls  int
		wantHandle bool
	}{
		{name: "required token missing", mode: AuthenticationRequired, wantCode: codes.Unauthenticated},
		{name: "optional token missing", mode: AuthenticationOptional, wantCode: codes.OK, wantHandle: true},
		{name: "optional malformed token", mode: AuthenticationOptional, metadata: metadata.Pairs("authorization", "not-bearer"), wantCode: codes.Unauthenticated},
		{name: "skipped malformed token", mode: AuthenticationSkipped, metadata: metadata.Pairs("authorization", "not-bearer"), wantCode: codes.OK, wantHandle: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			validator := &stubAccessTokenValidator{}
			interceptor := NewAuthInterceptor(
				validator,
				WithMethodAuthentication(func(string) AuthenticationMode { return test.mode }),
			)
			ctx := metadata.NewIncomingContext(context.Background(), test.metadata)
			handled := false

			_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, func(context.Context, any) (any, error) {
				handled = true
				return nil, nil
			})
			if status.Code(err) != test.wantCode {
				t.Fatalf("interceptor() code = %s, want %s", status.Code(err), test.wantCode)
			}
			if handled != test.wantHandle {
				t.Fatalf("handler called = %t, want %t", handled, test.wantHandle)
			}
			if validator.calls != test.wantCalls {
				t.Fatalf("ValidateToken() calls = %d, want %d", validator.calls, test.wantCalls)
			}
		})
	}
}

func TestAuthInterceptorRejectsInvalidAuthorizationMetadata(t *testing.T) {
	tests := []metadata.MD{
		metadata.Pairs("authorization", "Basic abc"),
		metadata.Pairs("authorization", "Bearer"),
		metadata.Pairs("authorization", "Bearer one", "authorization", "Bearer two"),
		metadata.Pairs("authorization", "Bearer "+strings.Repeat("a", maxAccessTokenLength+1)),
	}

	for _, md := range tests {
		validator := &stubAccessTokenValidator{}
		interceptor := NewAuthInterceptor(validator)
		ctx := metadata.NewIncomingContext(context.Background(), md)

		_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, func(context.Context, any) (any, error) {
			t.Fatal("handler must not be called")
			return nil, nil
		})
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("interceptor() code = %s, want %s", status.Code(err), codes.Unauthenticated)
		}
		if validator.calls != 0 {
			t.Fatalf("ValidateToken() calls = %d, want 0", validator.calls)
		}
	}
}

func TestAuthInterceptorMapsAuthenticationErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode codes.Code
	}{
		{name: "invalid token", err: status.Error(codes.Unauthenticated, "upstream detail"), wantCode: codes.Unauthenticated},
		{name: "disabled user", err: status.Error(codes.PermissionDenied, "upstream detail"), wantCode: codes.PermissionDenied},
		{name: "deadline", err: status.Error(codes.DeadlineExceeded, "upstream detail"), wantCode: codes.Unavailable},
		{name: "dependency unavailable", err: status.Error(codes.Unavailable, "upstream detail"), wantCode: codes.Unavailable},
		{name: "unexpected error", err: status.Error(codes.Internal, "upstream detail"), wantCode: codes.Unavailable},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			validator := &stubAccessTokenValidator{err: test.err}
			interceptor := NewAuthInterceptor(validator)
			ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer token"))

			_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, func(context.Context, any) (any, error) {
				t.Fatal("handler must not be called")
				return nil, nil
			})
			if status.Code(err) != test.wantCode {
				t.Fatalf("interceptor() code = %s, want %s", status.Code(err), test.wantCode)
			}
		})
	}
}

func TestAuthInterceptorRejectsInvalidAuthResponse(t *testing.T) {
	tests := []struct {
		response *pb.ValidateTokenResp
		wantCode codes.Code
	}{
		{response: nil, wantCode: codes.Unavailable},
		{response: &pb.ValidateTokenResp{}, wantCode: codes.Unavailable},
		{response: &pb.ValidateTokenResp{Claims: &pb.AuthClaims{}}, wantCode: codes.Unavailable},
		{response: &pb.ValidateTokenResp{Claims: &pb.AuthClaims{UserId: 1, Role: pb.UserRole_USER_ROLE_USER, Status: pb.UserStatus_USER_STATUS_DISABLED}}, wantCode: codes.PermissionDenied},
		{response: &pb.ValidateTokenResp{Claims: &pb.AuthClaims{UserId: 1, Role: pb.UserRole_USER_ROLE_UNSPECIFIED, Status: pb.UserStatus_USER_STATUS_NORMAL}}, wantCode: codes.Unavailable},
	}

	for _, test := range tests {
		validator := &stubAccessTokenValidator{response: test.response}
		interceptor := NewAuthInterceptor(validator)
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer token"))

		_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, func(context.Context, any) (any, error) {
			t.Fatal("handler must not be called")
			return nil, nil
		})
		if status.Code(err) != test.wantCode {
			t.Fatalf("interceptor() code = %s, want %s", status.Code(err), test.wantCode)
		}
	}
}
