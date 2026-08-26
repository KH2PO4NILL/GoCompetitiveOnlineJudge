package interceptor

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	defaultValidationTimeout = 3 * time.Second
	maxAccessTokenLength     = 8192
)

// Principal is the authenticated user identity exposed to RPC logic.
// Its values come from Auth RPC's latest session and user state rather than
// directly from the possibly stale claims embedded in an access token.
type Principal struct {
	UserID   int64
	Role     pb.UserRole
	Status   pb.UserStatus
	SchoolID int64
}

type principalContextKey struct{}

// WithPrincipal returns a child context containing an authenticated identity.
// Production code should only call it after successful authentication; it is
// exported so other interceptors and unit tests can construct a trusted context.
func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}

// PrincipalFromContext returns the identity injected by AuthInterceptor.
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
	// AuthenticationSkipped does not inspect user authorization metadata. It is
	// intended for methods protected by a separate service-to-service mechanism.
	AuthenticationSkipped
)

// MethodAuthentication returns the authentication mode for a full gRPC method
// name. A nil policy makes every method require authentication.
type MethodAuthentication func(fullMethod string) AuthenticationMode

// AccessTokenValidator is implemented by the generated AuthService client.
// Keeping this as a small interface makes the interceptor independently
// testable without starting Auth RPC.
type AccessTokenValidator interface {
	ValidateToken(
		ctx context.Context,
		in *pb.ValidateTokenReq,
		opts ...grpc.CallOption,
	) (*pb.ValidateTokenResp, error)
}

type authInterceptorConfig struct {
	methodAuthentication MethodAuthentication
	validationTimeout    time.Duration
}

// AuthInterceptorOption customizes the shared authentication interceptor.
type AuthInterceptorOption func(*authInterceptorConfig)

// WithMethodAuthentication configures public, protected, and internal methods.
func WithMethodAuthentication(policy MethodAuthentication) AuthInterceptorOption {
	return func(config *authInterceptorConfig) {
		config.methodAuthentication = policy
	}
}

// WithValidationTimeout limits each call from a protected RPC to Auth RPC.
// Non-positive values are ignored.
func WithValidationTimeout(timeout time.Duration) AuthInterceptorOption {
	return func(config *authInterceptorConfig) {
		if timeout > 0 {
			config.validationTimeout = timeout
		}
	}
}

// NewAuthInterceptor authenticates user access tokens through Auth RPC and
// injects the returned Principal into the handler context. It deliberately does
// not parse JWTs or query sessions itself; Auth RPC remains the sole identity
// authority.
func NewAuthInterceptor(
	validator AccessTokenValidator,
	options ...AuthInterceptorOption,
) grpc.UnaryServerInterceptor {
	config := authInterceptorConfig{validationTimeout: defaultValidationTimeout}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}

	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		mode := AuthenticationRequired
		if config.methodAuthentication != nil {
			mode = config.methodAuthentication(info.FullMethod)
		}
		if mode == AuthenticationSkipped {
			return handler(ctx, req)
		}

		accessToken, present, err := accessTokenFromContext(ctx)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "missing or invalid access token")
		}
		if !present {
			if mode == AuthenticationOptional {
				return handler(ctx, req)
			}
			return nil, status.Error(codes.Unauthenticated, "missing or invalid access token")
		}
		if validator == nil {
			return nil, status.Error(codes.Unavailable, "authentication service is unavailable")
		}

		validationCtx, cancel := context.WithTimeout(ctx, config.validationTimeout)
		defer cancel()

		response, err := validator.ValidateToken(validationCtx, &pb.ValidateTokenReq{
			AccessToken: accessToken,
		})
		if err != nil {
			return nil, authenticationError(err)
		}
		if response == nil || response.Claims == nil || response.Claims.UserId <= 0 {
			return nil, status.Error(codes.Unavailable, "authentication service returned an invalid response")
		}
		if response.Claims.Status != pb.UserStatus_USER_STATUS_NORMAL {
			return nil, status.Error(codes.PermissionDenied, "user account is unavailable")
		}
		switch response.Claims.Role {
		case pb.UserRole_USER_ROLE_USER,
			pb.UserRole_USER_ROLE_ADMIN,
			pb.UserRole_USER_ROLE_SUPER_ADMIN:
		default:
			return nil, status.Error(codes.Unavailable, "authentication service returned an invalid response")
		}

		principal := Principal{
			UserID:   response.Claims.UserId,
			Role:     response.Claims.Role,
			Status:   response.Claims.Status,
			SchoolID: response.Claims.SchoolId,
		}
		return handler(WithPrincipal(ctx, principal), req)
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
