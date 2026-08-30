package interceptor

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"
	"github.com/zeromicro/go-zero/zrpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	defaultValidationTimeout = 3 * time.Second
	maxAccessTokenLength     = 8192
)

// Principal is the authenticated user identity exposed to every RPC service.
// Role, status and school come from the identity authority's latest state.
// SessionID is populated by Auth RPC's local authenticator when a method needs
// to operate on the current login session; remote services must not rely on it.
type Principal struct {
	UserID    int64
	Role      pb.UserRole
	Status    pb.UserStatus
	SchoolID  int64
	SessionID int64
}

type principalContextKey struct{}

// WithPrincipal returns a child context containing an authenticated identity.
// Production code should only call it after successful authentication; it is
// exported so authorization helpers and unit tests can construct trusted input.
func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}

// PrincipalFromContext returns the identity injected by the shared interceptor.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(Principal)
	return principal, ok && principal.UserID > 0
}

// AuthenticationMode determines how a method handles a user access token.
type AuthenticationMode uint8

const (
	// AuthenticationRequired rejects requests without a valid access token.
	AuthenticationRequired AuthenticationMode = iota
	// AuthenticationOptional permits anonymous requests but validates and
	// injects an identity whenever the authorization header is present.
	AuthenticationOptional
	// AuthenticationAlternative attempts to authenticate a presented access
	// token but lets the handler validate an alternative credential if the token
	// is missing or invalid. Use only for endpoints such as logout that enforce
	// the fallback credential inside the handler.
	AuthenticationAlternative
	// AuthenticationSkipped does not inspect user authorization metadata. It is
	// intended for public methods or methods protected by service credentials.
	AuthenticationSkipped
)

// MethodAuthentication returns the authentication mode for a full gRPC method
// name. A nil policy makes every method require authentication.
type MethodAuthentication func(fullMethod string) AuthenticationMode

// RequestAuthentication can refine a unary method's policy using its request.
// It is useful for the small number of RPCs whose credential flow depends on a
// request field. Stream RPCs always use MethodAuthentication.
type RequestAuthentication func(fullMethod string, request any) AuthenticationMode

// TokenAuthenticator is the single backend contract used by all gRPC servers.
// Auth RPC supplies a local implementation; other services use
// NewRPCAuthenticator to delegate validation to Auth RPC.
type TokenAuthenticator interface {
	Authenticate(ctx context.Context, accessToken string) (Principal, error)
}

// TokenAuthenticatorFunc adapts a function to TokenAuthenticator.
type TokenAuthenticatorFunc func(context.Context, string) (Principal, error)

func (f TokenAuthenticatorFunc) Authenticate(ctx context.Context, accessToken string) (Principal, error) {
	return f(ctx, accessToken)
}

// AccessTokenValidator is implemented by the generated AuthService client.
type AccessTokenValidator interface {
	ValidateToken(
		ctx context.Context,
		in *pb.ValidateTokenReq,
		opts ...grpc.CallOption,
	) (*pb.ValidateTokenResp, error)
}

// NewRPCAuthenticator adapts the Auth RPC client to the shared authenticator
// contract used by every non-authentication service.
func NewRPCAuthenticator(
	validator AccessTokenValidator,
	credentials ...ServiceClientAuthConf,
) TokenAuthenticator {
	// ValidateToken carries the caller's bearer token. Register the method once
	// at the shared client boundary so no service can accidentally log it.
	zrpc.DontLogClientContentForMethod(pb.AuthService_ValidateToken_FullMethodName)

	var credential ServiceClientAuthConf
	if len(credentials) > 0 {
		credential = credentials[0]
	}
	return TokenAuthenticatorFunc(func(ctx context.Context, accessToken string) (Principal, error) {
		if validator == nil {
			return Principal{}, status.Error(codes.Unavailable, "authentication service is unavailable")
		}
		if credential.Name != "" || credential.Token != "" {
			ctx = WithOutgoingServiceCredentials(ctx, credential.Name, credential.Token)
		}
		response, err := validator.ValidateToken(ctx, &pb.ValidateTokenReq{AccessToken: accessToken})
		if err != nil {
			return Principal{}, err
		}
		if response == nil || response.Claims == nil {
			return Principal{}, status.Error(codes.Unavailable, "authentication service returned an invalid response")
		}
		return Principal{
			UserID:   response.Claims.UserId,
			Role:     response.Claims.Role,
			Status:   response.Claims.Status,
			SchoolID: response.Claims.SchoolId,
		}, nil
	})
}

type authInterceptorConfig struct {
	methodAuthentication  MethodAuthentication
	requestAuthentication RequestAuthentication
	validationTimeout     time.Duration
}

// AuthInterceptorOption customizes the shared authentication interceptor.
type AuthInterceptorOption func(*authInterceptorConfig)

// WithMethodAuthentication configures public, protected and internal methods.
func WithMethodAuthentication(policy MethodAuthentication) AuthInterceptorOption {
	return func(config *authInterceptorConfig) {
		config.methodAuthentication = policy
	}
}

// WithRequestAuthentication configures a request-aware policy for unary RPCs.
func WithRequestAuthentication(policy RequestAuthentication) AuthInterceptorOption {
	return func(config *authInterceptorConfig) {
		config.requestAuthentication = policy
	}
}

// WithValidationTimeout limits authentication backend work for each RPC.
// Non-positive values are ignored.
func WithValidationTimeout(timeout time.Duration) AuthInterceptorOption {
	return func(config *authInterceptorConfig) {
		if timeout > 0 {
			config.validationTimeout = timeout
		}
	}
}

// NewAuthInterceptor returns the unified unary authentication interceptor.
func NewAuthInterceptor(
	authenticator TokenAuthenticator,
	options ...AuthInterceptorOption,
) grpc.UnaryServerInterceptor {
	config := newAuthInterceptorConfig(options)
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		mode := config.authenticationMode(info.FullMethod)
		if !isFrameworkPublicMethod(info.FullMethod) && config.requestAuthentication != nil {
			mode = config.requestAuthentication(info.FullMethod, req)
		}
		authenticatedCtx, err := authenticateContext(ctx, authenticator, mode, config.validationTimeout)
		if err != nil {
			return nil, err
		}
		return handler(authenticatedCtx, req)
	}
}

// NewAuthStreamInterceptor applies the same authentication rules to stream
// RPCs and injects Principal through ServerStream.Context().
func NewAuthStreamInterceptor(
	authenticator TokenAuthenticator,
	options ...AuthInterceptorOption,
) grpc.StreamServerInterceptor {
	config := newAuthInterceptorConfig(options)
	return func(
		srv any,
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx, err := authenticateContext(
			stream.Context(),
			authenticator,
			config.authenticationMode(info.FullMethod),
			config.validationTimeout,
		)
		if err != nil {
			return err
		}
		return handler(srv, &contextServerStream{ServerStream: stream, ctx: ctx})
	}
}

func newAuthInterceptorConfig(options []AuthInterceptorOption) authInterceptorConfig {
	config := authInterceptorConfig{validationTimeout: defaultValidationTimeout}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	return config
}

func (c authInterceptorConfig) authenticationMode(fullMethod string) AuthenticationMode {
	if isFrameworkPublicMethod(fullMethod) {
		return AuthenticationSkipped
	}
	if c.methodAuthentication == nil {
		return AuthenticationRequired
	}
	return c.methodAuthentication(fullMethod)
}

// isFrameworkPublicMethod is the project-wide allowlist for gRPC framework
// endpoints that must remain reachable by unauthenticated health probes.
func isFrameworkPublicMethod(fullMethod string) bool {
	switch fullMethod {
	case healthpb.Health_Check_FullMethodName, healthpb.Health_Watch_FullMethodName:
		return true
	default:
		return false
	}
}

func authenticateContext(
	ctx context.Context,
	authenticator TokenAuthenticator,
	mode AuthenticationMode,
	timeout time.Duration,
) (context.Context, error) {
	if mode == AuthenticationSkipped {
		return ctx, nil
	}

	accessToken, present, err := accessTokenFromContext(ctx)
	if err != nil {
		if mode == AuthenticationAlternative {
			return ctx, nil
		}
		return nil, status.Error(codes.Unauthenticated, "missing or invalid access token")
	}
	if !present {
		if mode == AuthenticationOptional || mode == AuthenticationAlternative {
			return ctx, nil
		}
		return nil, status.Error(codes.Unauthenticated, "missing or invalid access token")
	}
	if authenticator == nil {
		if mode == AuthenticationAlternative {
			return ctx, nil
		}
		return nil, status.Error(codes.Unavailable, "authentication service is unavailable")
	}

	validationCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	principal, err := authenticator.Authenticate(validationCtx, accessToken)
	if err != nil {
		if mode == AuthenticationAlternative {
			return ctx, nil
		}
		return nil, authenticationError(err)
	}
	if err := validatePrincipal(principal); err != nil {
		if mode == AuthenticationAlternative {
			return ctx, nil
		}
		return nil, err
	}
	return WithPrincipal(ctx, principal), nil
}

func validatePrincipal(principal Principal) error {
	if principal.UserID <= 0 {
		return status.Error(codes.Unavailable, "authentication service returned an invalid response")
	}
	if principal.Status != pb.UserStatus_USER_STATUS_NORMAL {
		return status.Error(codes.PermissionDenied, "user account is unavailable")
	}
	switch principal.Role {
	case pb.UserRole_USER_ROLE_USER,
		pb.UserRole_USER_ROLE_ADMIN,
		pb.UserRole_USER_ROLE_SUPER_ADMIN:
		return nil
	default:
		return status.Error(codes.Unavailable, "authentication service returned an invalid response")
	}
}

func accessTokenFromContext(ctx context.Context) (token string, present bool, err error) {
	values := metadata.ValueFromIncomingContext(ctx, "authorization")
	if len(values) == 0 {
		return "", false, nil
	}
	if len(values) != 1 {
		return "", true, errors.New("authorization metadata must occur exactly once")
	}

	parts := strings.Fields(values[0])
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", true, errors.New("authorization metadata is not a bearer token")
	}
	if len(parts[1]) > maxAccessTokenLength {
		return "", true, errors.New("access token is too long")
	}
	return parts[1], true, nil
}

func authenticationError(err error) error {
	switch status.Code(err) {
	case codes.Unauthenticated:
		return status.Error(codes.Unauthenticated, "access token is invalid or expired")
	case codes.PermissionDenied:
		return status.Error(codes.PermissionDenied, "user account is unavailable")
	case codes.Canceled:
		return status.Error(codes.Canceled, "authentication was canceled")
	case codes.DeadlineExceeded, codes.Unavailable:
		return status.Error(codes.Unavailable, "authentication service is unavailable")
	default:
		return status.Error(codes.Unavailable, "authentication service is unavailable")
	}
}

type contextServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *contextServerStream) Context() context.Context {
	return s.ctx
}
