package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/client/userservice"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"
	"google.golang.org/grpc"
)

type userClientStub struct {
	userservice.UserService
	role pb.UserRole
}

func (s userClientStub) GetCurrentUser(
	context.Context, *pb.GetCurrentUserReq, ...grpc.CallOption,
) (*pb.GetCurrentUserResp, error) {
	return &pb.GetCurrentUserResp{User: &pb.UserInfo{
		UserId: 1, Role: s.role, Status: pb.UserStatus_USER_STATUS_NORMAL,
	}}, nil
}

func TestAdminIdentityRejectsUserAndAllowsAdmin(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Authorization", "Bearer token")

	userRecorder := httptest.NewRecorder()
	NewAdminIdentityMiddleware(userClientStub{role: pb.UserRole_USER_ROLE_USER}).Handle(
		func(http.ResponseWriter, *http.Request) { t.Fatal("ordinary user reached admin handler") },
	)(userRecorder, request)
	if userRecorder.Code != http.StatusForbidden {
		t.Fatalf("ordinary user status = %d", userRecorder.Code)
	}

	adminRecorder := httptest.NewRecorder()
	called := false
	NewAdminIdentityMiddleware(userClientStub{role: pb.UserRole_USER_ROLE_ADMIN}).Handle(
		func(http.ResponseWriter, *http.Request) { called = true },
	)(adminRecorder, request)
	if !called {
		t.Fatal("admin did not reach next handler")
	}
}
