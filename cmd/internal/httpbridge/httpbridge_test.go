package httpbridge

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
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

func TestProtectResponseIncludesErrors(t *testing.T) {
	for _, responseStatus := range []int{http.StatusOK, http.StatusUnauthorized, http.StatusForbidden, http.StatusInternalServerError} {
		recorder := httptest.NewRecorder()
		ProtectResponse(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(responseStatus)
		})(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil))
		if recorder.Code != responseStatus || recorder.Header().Get("Cache-Control") != "no-store" || recorder.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("response status=%d headers=%v", recorder.Code, recorder.Header())
		}
	}
}

func TestParseTimestampRejectsOutOfRangeUTC(t *testing.T) {
	for _, value := range []string{"0000-01-01T00:00:00Z", "0001-01-01T00:00:00+01:00", "9999-12-31T23:59:59-01:00", "not-a-timestamp"} {
		t.Run(value, func(t *testing.T) {
			if timestamp, err := ParseTimestamp(value, "start_at"); timestamp != nil || status.Code(err) != codes.InvalidArgument {
				t.Fatalf("ParseTimestamp returned timestamp=%v error=%v", timestamp, err)
			}
		})
	}
	for _, value := range []string{"0001-01-01T00:00:00Z", "9999-12-31T23:59:59.999999999Z", "2026-09-05T23:00:00+08:00"} {
		if timestamp, err := ParseTimestamp(value, "start_at"); err != nil || !timestamp.IsValid() {
			t.Fatalf("valid timestamp %q rejected: %v", value, err)
		}
	}
	if timestamp, err := ParseOptionalTimestamp(nil, "start_at"); timestamp != nil || err != nil {
		t.Fatalf("absent optional timestamp changed: timestamp=%v error=%v", timestamp, err)
	}
}
