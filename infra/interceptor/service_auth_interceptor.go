package interceptor

import (
	"context"
	"crypto/subtle"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	ServiceNameMetadataKey  = "x-service-name"
	ServiceTokenMetadataKey = "x-service-token"
	maxServiceNameLength    = 128
	maxServiceTokenLength   = 4096
)

// ServicePrincipal identifies a trusted service-to-service caller. It is
// deliberately separate from Principal so an internal service can never be
// mistaken for an administrator user.
type ServicePrincipal struct {
	Name string
}

type servicePrincipalContextKey struct{}

func WithServicePrincipal(ctx context.Context, principal ServicePrincipal) context.Context {
	return context.WithValue(ctx, servicePrincipalContextKey{}, principal)
}

func ServicePrincipalFromContext(ctx context.Context) (ServicePrincipal, bool) {
	principal, ok := ctx.Value(servicePrincipalContextKey{}).(ServicePrincipal)
	return principal, ok && principal.Name != ""
}

// WithOutgoingServiceCredentials attaches credentials to an internal gRPC
// client call. Callers should obtain token from their own secret configuration
// and must never forward these metadata values from an external request.
func WithOutgoingServiceCredentials(ctx context.Context, name, token string) context.Context {
	md, _ := metadata.FromOutgoingContext(ctx)
	md = md.Copy()
	md.Set(ServiceNameMetadataKey, strings.TrimSpace(name))
	md.Set(ServiceTokenMetadataKey, token)
	return metadata.NewOutgoingContext(ctx, md)
}

// ServiceAuthenticationPolicy returns the service names allowed to call a
// method. An empty result means the method is not service-authenticated.
type ServiceAuthenticationPolicy func(fullMethod string) []string

// NewServiceAuthInterceptor validates static service credentials for methods
// selected by policy and injects a ServicePrincipal into the handler context.
// Credential values are compared in constant time and are never logged.
func NewServiceAuthInterceptor(
	credentials map[string]string,
	policy ServiceAuthenticationPolicy,
) grpc.UnaryServerInterceptor {
	trusted := cloneServiceCredentials(credentials)
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		authenticatedCtx, err := authenticateServiceContext(ctx, info.FullMethod, trusted, policy)
		if err != nil {
			return nil, err
		}
		return handler(authenticatedCtx, req)
	}
}

// NewServiceAuthStreamInterceptor applies service-to-service authentication to
// stream RPCs and exposes the trusted caller through ServerStream.Context().
func NewServiceAuthStreamInterceptor(
	credentials map[string]string,
	policy ServiceAuthenticationPolicy,
) grpc.StreamServerInterceptor {
	trusted := cloneServiceCredentials(credentials)
	return func(
		srv any,
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx, err := authenticateServiceContext(stream.Context(), info.FullMethod, trusted, policy)
		if err != nil {
			return err
		}
		return handler(srv, &contextServerStream{ServerStream: stream, ctx: ctx})
	}
}

func authenticateServiceContext(
	ctx context.Context,
	fullMethod string,
	trusted map[string]string,
	policy ServiceAuthenticationPolicy,
) (context.Context, error) {
	if policy == nil {
		return ctx, nil
	}
	allowed := policy(fullMethod)
	if len(allowed) == 0 {
		return ctx, nil
	}

	name, token, err := serviceCredentialsFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "missing or invalid service credentials")
	}
	if !containsService(allowed, name) {
		return nil, status.Error(codes.PermissionDenied, "service caller is not allowed")
	}
	expected, ok := trusted[name]
	if !ok || !constantTimeStringEqual(expected, token) {
		return nil, status.Error(codes.Unauthenticated, "missing or invalid service credentials")
	}
	return WithServicePrincipal(ctx, ServicePrincipal{Name: name}), nil
}

func cloneServiceCredentials(credentials map[string]string) map[string]string {
	result := make(map[string]string, len(credentials))
	for rawName, token := range credentials {
		name := strings.TrimSpace(rawName)
		if name != "" && token != "" {
			result[name] = token
		}
	}
	return result
}

func serviceCredentialsFromContext(ctx context.Context) (string, string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", "", status.Error(codes.Unauthenticated, "service metadata is required")
	}
	names := md.Get(ServiceNameMetadataKey)
	tokens := md.Get(ServiceTokenMetadataKey)
	if len(names) != 1 || len(tokens) != 1 {
		return "", "", status.Error(codes.Unauthenticated, "service metadata must occur exactly once")
	}
	name := strings.TrimSpace(names[0])
	token := tokens[0]
	if name == "" || token == "" || len(name) > maxServiceNameLength || len(token) > maxServiceTokenLength {
		return "", "", status.Error(codes.Unauthenticated, "invalid service metadata")
	}
	return name, token, nil
}

func containsService(allowed []string, name string) bool {
	for _, candidate := range allowed {
		if strings.TrimSpace(candidate) == name {
			return true
		}
	}
	return false
}

func constantTimeStringEqual(expected, actual string) bool {
	if len(expected) != len(actual) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(actual)) == 1
}
