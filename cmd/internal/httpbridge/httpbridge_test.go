package httpbridge

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc/metadata"
)

func TestForwardAuthorizationCopiesOnlyBearerHeader(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Authorization", "Bearer token-value")
	request.Header.Set("X-Service-Token", "must-not-forward")

	called := false
	ForwardAuthorization(func(_ http.ResponseWriter, request *http.Request) {
		called = true
		md, ok := metadata.FromOutgoingContext(request.Context())
		if !ok || len(md.Get("authorization")) != 1 || md.Get("authorization")[0] != "Bearer token-value" {
			t.Fatalf("authorization metadata = %v", md.Get("authorization"))
		}
		if values := md.Get("x-service-token"); len(values) != 0 {
			t.Fatalf("service credential leaked: %v", values)
		}
	})(httptest.NewRecorder(), request)

	if !called {
		t.Fatal("next handler was not called")
	}
}

func TestBearerToken(t *testing.T) {
	if token, ok := BearerToken("bearer abc"); !ok || token != "abc" {
		t.Fatalf("BearerToken() = (%q, %t)", token, ok)
	}
	if _, ok := BearerToken("Basic abc"); ok {
		t.Fatal("Basic credential must not be accepted")
	}
}
