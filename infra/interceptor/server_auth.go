package interceptor

import "google.golang.org/grpc"

// ServerAuthInterceptors is the complete authentication stack for a gRPC
// server. Register both slices so unary and stream methods share identical user
// and service-identity boundaries.
type ServerAuthInterceptors struct {
	Unary  []grpc.UnaryServerInterceptor
	Stream []grpc.StreamServerInterceptor
}

// NewServerAuthInterceptors builds the project-wide interceptor stack. Service
// authentication runs first for internal method allowlists, followed by user
// authentication for public, optional and protected methods.
func NewServerAuthInterceptors(
	authenticator TokenAuthenticator,
	serviceCredentials map[string]string,
	servicePolicy ServiceAuthenticationPolicy,
	options ...AuthInterceptorOption,
) ServerAuthInterceptors {
	return ServerAuthInterceptors{
		Unary: []grpc.UnaryServerInterceptor{
			NewServiceAuthInterceptor(serviceCredentials, servicePolicy),
			NewAuthInterceptor(authenticator, options...),
		},
		Stream: []grpc.StreamServerInterceptor{
			NewServiceAuthStreamInterceptor(serviceCredentials, servicePolicy),
			NewAuthStreamInterceptor(authenticator, options...),
		},
	}
}
