package interceptor

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestServiceAuthInterceptorInjectsTrustedCaller(t *testing.T) {
	interceptor := NewServiceAuthInterceptor(
		map[string]string{"submit.rpc": "secret"},
		func(string) []string { return []string{"submit.rpc"} },
	)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		ServiceNameMetadataKey, "submit.rpc",
		ServiceTokenMetadataKey, "secret",
	))
	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/problem.ProblemService/GetJudgeProblemSpec"}, func(ctx context.Context, _ any) (any, error) {
		principal, ok := ServicePrincipalFromContext(ctx)
		if !ok || principal.Name != "submit.rpc" {
			t.Fatalf("ServicePrincipalFromContext() = (%+v, %t)", principal, ok)
		}
		return nil, nil
	})
	if err != nil {
		t.Fatalf("interceptor() error = %v", err)
	}
}

func TestServiceAuthInterceptorRejectsInvalidCallers(t *testing.T) {
	tests := []struct {
		name     string
		md       metadata.MD
		wantCode codes.Code
	}{
		{name: "missing", wantCode: codes.Unauthenticated},
		{name: "bad token", md: metadata.Pairs(ServiceNameMetadataKey, "submit.rpc", ServiceTokenMetadataKey, "bad"), wantCode: codes.Unauthenticated},
		{name: "not allowed", md: metadata.Pairs(ServiceNameMetadataKey, "judge.worker", ServiceTokenMetadataKey, "judge-secret"), wantCode: codes.PermissionDenied},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			interceptor := NewServiceAuthInterceptor(
				map[string]string{"submit.rpc": "secret", "judge.worker": "judge-secret"},
				func(string) []string { return []string{"submit.rpc"} },
			)
			ctx := metadata.NewIncomingContext(context.Background(), test.md)
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

func TestServiceAuthInterceptorSkipsUnselectedMethods(t *testing.T) {
	interceptor := NewServiceAuthInterceptor(nil, func(string) []string { return nil })
	handled := false
	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{}, func(context.Context, any) (any, error) {
		handled = true
		return nil, nil
	})
	if err != nil || !handled {
		t.Fatalf("interceptor() = handled %t, error %v", handled, err)
	}
}

func TestWithOutgoingServiceCredentials(t *testing.T) {
	ctx := WithOutgoingServiceCredentials(context.Background(), "old.rpc", "old-secret")
	ctx = WithOutgoingServiceCredentials(ctx, "submit.rpc", "secret")
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok || len(md.Get(ServiceNameMetadataKey)) != 1 || md.Get(ServiceNameMetadataKey)[0] != "submit.rpc" ||
		len(md.Get(ServiceTokenMetadataKey)) != 1 || md.Get(ServiceTokenMetadataKey)[0] != "secret" {
		t.Fatalf("outgoing metadata = %v", md)
	}
}

func TestServiceAuthStreamInterceptorInjectsTrustedCaller(t *testing.T) {
	interceptor := NewServiceAuthStreamInterceptor(
		map[string]string{"submit.rpc": "secret"},
		func(string) []string { return []string{"submit.rpc"} },
	)
	stream := &stubServerStream{ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		ServiceNameMetadataKey, "submit.rpc",
		ServiceTokenMetadataKey, "secret",
	))}

	err := interceptor(nil, stream, &grpc.StreamServerInfo{FullMethod: "/test.Service/Stream"}, func(_ any, stream grpc.ServerStream) error {
		principal, ok := ServicePrincipalFromContext(stream.Context())
		if !ok || principal.Name != "submit.rpc" {
			t.Fatalf("ServicePrincipalFromContext() = (%+v, %t)", principal, ok)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("stream interceptor error = %v", err)
	}
}
