package interceptor

import (
	"context"
	"strings"
	"testing"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type stubAccessTokenValidator struct {
	response *pb.ValidateTokenResp
	err      error
	calls    int
	token    string
	ctx      context.Context
}

func (s *stubAccessTokenValidator) ValidateToken(
	ctx context.Context,
	in *pb.ValidateTokenReq,
	_ ...grpc.CallOption,
) (*pb.ValidateTokenResp, error) {
	s.calls++
	s.token = in.AccessToken
	s.ctx = ctx
	return s.response, s.err
}

func TestRPCAuthenticatorAddsServiceCredentials(t *testing.T) {
	validator := &stubAccessTokenValidator{response: &pb.ValidateTokenResp{Claims: &pb.AuthClaims{
		UserId: 1,
		Role:   pb.UserRole_USER_ROLE_USER,
		Status: pb.UserStatus_USER_STATUS_NORMAL,
	}}}
	authenticator := NewRPCAuthenticator(validator, ServiceClientAuthConf{Name: "problem.rpc", Token: "secret"})
	if _, err := authenticator.Authenticate(context.Background(), "access-token"); err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	md, ok := metadata.FromOutgoingContext(validator.ctx)
	if !ok || len(md.Get(ServiceNameMetadataKey)) != 1 || md.Get(ServiceNameMetadataKey)[0] != "problem.rpc" ||
		len(md.Get(ServiceTokenMetadataKey)) != 1 || md.Get(ServiceTokenMetadataKey)[0] != "secret" {
		t.Fatalf("outgoing service metadata = %v", md)
	}
}

func TestAuthInterceptorInjectsValidatedPrincipal(t *testing.T) {
	validator := &stubAccessTokenValidator{response: &pb.ValidateTokenResp{
		Claims: &pb.AuthClaims{
			UserId: 42, Role: pb.UserRole_USER_ROLE_ADMIN,
			Status: pb.UserStatus_USER_STATUS_NORMAL, SchoolId: 7,
		},
	}}
	interceptor := NewAuthInterceptor(NewRPCAuthenticator(validator))
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
		{name: "alternative malformed token", mode: AuthenticationAlternative, metadata: metadata.Pairs("authorization", "not-bearer"), wantCode: codes.OK, wantHandle: true},
		{name: "skipped malformed token", mode: AuthenticationSkipped, metadata: metadata.Pairs("authorization", "not-bearer"), wantCode: codes.OK, wantHandle: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			validator := &stubAccessTokenValidator{}
			interceptor := NewAuthInterceptor(
				NewRPCAuthenticator(validator),
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
		interceptor := NewAuthInterceptor(NewRPCAuthenticator(validator))
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
			interceptor := NewAuthInterceptor(NewRPCAuthenticator(validator))
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
		interceptor := NewAuthInterceptor(NewRPCAuthenticator(validator))
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

func TestAuthStreamInterceptorInjectsPrincipal(t *testing.T) {
	authenticator := TokenAuthenticatorFunc(func(_ context.Context, token string) (Principal, error) {
		if token != "stream-token" {
			t.Fatalf("Authenticate() token = %q", token)
		}
		return Principal{
			UserID: 9,
			Role:   pb.UserRole_USER_ROLE_USER,
			Status: pb.UserStatus_USER_STATUS_NORMAL,
		}, nil
	})
	stream := &stubServerStream{ctx: metadata.NewIncomingContext(
		context.Background(),
		metadata.Pairs("authorization", "Bearer stream-token"),
	)}
	interceptor := NewAuthStreamInterceptor(authenticator)

	err := interceptor(nil, stream, &grpc.StreamServerInfo{FullMethod: "/test.Service/Stream"}, func(_ any, stream grpc.ServerStream) error {
		principal, ok := PrincipalFromContext(stream.Context())
		if !ok || principal.UserID != 9 {
			t.Fatalf("PrincipalFromContext() = (%+v, %t)", principal, ok)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("stream interceptor error = %v", err)
	}
}

func TestAuthInterceptorUsesRequestPolicy(t *testing.T) {
	interceptor := NewAuthInterceptor(
		nil,
		WithRequestAuthentication(func(_ string, request any) AuthenticationMode {
			if request == "public" {
				return AuthenticationSkipped
			}
			return AuthenticationRequired
		}),
	)

	response, err := interceptor(context.Background(), "public", &grpc.UnaryServerInfo{}, func(_ context.Context, request any) (any, error) {
		return request, nil
	})
	if err != nil || response != "public" {
		t.Fatalf("interceptor() = (%v, %v), want (public, nil)", response, err)
	}
}

func TestAuthInterceptorAlwaysAllowsHealthCheck(t *testing.T) {
	requestPolicyCalled := false
	interceptor := NewAuthInterceptor(
		nil,
		WithMethodAuthentication(func(string) AuthenticationMode { return AuthenticationRequired }),
		WithRequestAuthentication(func(string, any) AuthenticationMode {
			requestPolicyCalled = true
			return AuthenticationRequired
		}),
	)
	handled := false

	_, err := interceptor(
		context.Background(),
		&healthpb.HealthCheckRequest{},
		&grpc.UnaryServerInfo{FullMethod: healthpb.Health_Check_FullMethodName},
		func(context.Context, any) (any, error) {
			handled = true
			return &healthpb.HealthCheckResponse{}, nil
		},
	)
	if err != nil {
		t.Fatalf("health check interceptor error = %v", err)
	}
	if !handled {
		t.Fatal("health check handler was not called")
	}
	if requestPolicyCalled {
		t.Fatal("request authentication policy must not override the health check allowlist")
	}
}

func TestAuthStreamInterceptorAlwaysAllowsHealthWatch(t *testing.T) {
	interceptor := NewAuthStreamInterceptor(
		nil,
		WithMethodAuthentication(func(string) AuthenticationMode { return AuthenticationRequired }),
	)
	stream := &stubServerStream{ctx: context.Background()}
	handled := false

	err := interceptor(
		nil,
		stream,
		&grpc.StreamServerInfo{FullMethod: healthpb.Health_Watch_FullMethodName},
		func(any, grpc.ServerStream) error {
			handled = true
			return nil
		},
	)
	if err != nil {
		t.Fatalf("health watch interceptor error = %v", err)
	}
	if !handled {
		t.Fatal("health watch handler was not called")
	}
}

func TestAuthInterceptorAlternativeCredentialFallback(t *testing.T) {
	authenticator := TokenAuthenticatorFunc(func(context.Context, string) (Principal, error) {
		return Principal{}, status.Error(codes.Unauthenticated, "expired")
	})
	interceptor := NewAuthInterceptor(
		authenticator,
		WithMethodAuthentication(func(string) AuthenticationMode { return AuthenticationAlternative }),
	)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer expired"))
	handled := false

	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, func(ctx context.Context, _ any) (any, error) {
		handled = true
		if _, ok := PrincipalFromContext(ctx); ok {
			t.Fatal("invalid access token unexpectedly injected a principal")
		}
		return nil, nil
	})
	if err != nil || !handled {
		t.Fatalf("interceptor() = handled %t, error %v", handled, err)
	}
}

type stubServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *stubServerStream) Context() context.Context {
	return s.ctx
}
