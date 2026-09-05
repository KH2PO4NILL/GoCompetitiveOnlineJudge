package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/internal/httpbridge"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/client/userservice"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type userClientStub struct {
	userservice.UserService
	response          *pb.GetCurrentUserResp
	authorizationSeen *string
}

func (s userClientStub) GetCurrentUser(
	ctx context.Context, _ *pb.GetCurrentUserReq, _ ...grpc.CallOption,
) (*pb.GetCurrentUserResp, error) {
	if s.authorizationSeen != nil {
		md, _ := metadata.FromOutgoingContext(ctx)
		values := md.Get("authorization")
		if len(values) == 1 {
			*s.authorizationSeen = values[0]
		}
	}
	return s.response, nil
}

func TestUserIdentityRequiresAndValidatesBearerToken(t *testing.T) {
	middleware := NewUserIdentityMiddleware(userClientStub{response: &pb.GetCurrentUserResp{User: &pb.UserInfo{
		UserId: 1, Role: pb.UserRole_USER_ROLE_USER, Status: pb.UserStatus_USER_STATUS_NORMAL,
	}}})

	missingRecorder := httptest.NewRecorder()
	middleware.Handle(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next must not run without a token")
	})(missingRecorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if missingRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("missing token status = %d", missingRecorder.Code)
	}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Authorization", "Bearer token")
	recorder := httptest.NewRecorder()
	called := false
	middleware.Handle(func(http.ResponseWriter, *http.Request) { called = true })(recorder, request)
	if !called {
		t.Fatal("next handler was not called for a valid user")
	}
}

func TestUserIdentityUsesForwardedBearerForUserRPC(t *testing.T) {
	seen := ""
	middleware := NewUserIdentityMiddleware(userClientStub{
		response: &pb.GetCurrentUserResp{User: &pb.UserInfo{
			UserId: 1, Role: pb.UserRole_USER_ROLE_USER, Status: pb.UserStatus_USER_STATUS_NORMAL,
		}},
		authorizationSeen: &seen,
	})
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Authorization", "Bearer token")

	handler := httpbridge.ForwardAuthorization(middleware.Handle(func(http.ResponseWriter, *http.Request) {}))
	handler(httptest.NewRecorder(), request)
	if seen != "Bearer token" {
		t.Fatalf("authorization metadata = %q", seen)
	}
}
