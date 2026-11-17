package auth

import (
	"context"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/internal/httpbridge"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/client/authservice"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"net/http"
	"net/http/httptest"
	"testing"
)

type rejectedLoginAuth struct {
	authservice.AuthService
	revoked       *pb.LogoutReq
	authorization []string
}

func (f *rejectedLoginAuth) Logout(ctx context.Context, in *pb.LogoutReq, _ ...grpc.CallOption) (*pb.EmptyResp, error) {
	f.revoked = in
	md, _ := metadata.FromOutgoingContext(ctx)
	f.authorization = md.Get("authorization")
	return &pb.EmptyResp{}, nil
}
func TestRejectedAdminLoginPreservesOriginalCookieAndRevokesOnlyNewSession(t *testing.T) {
	req := httptest.NewRequest("POST", "https://judge.example/api/v1/admin/auth/login/username", nil)
	req.Header.Set("X-GCOJ-Browser-Session", "1")
	req.AddCookie(&http.Cookie{Name: "gcoj_admin_refresh", Value: "original-browser-session"})
	writer := httptest.NewRecorder()
	ctx := httpbridge.WithBrowserSession(req.Context(), writer, req)
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer original-access"))
	fake := &rejectedLoginAuth{}
	_, err := adminLoginResponse(ctx, &svc.ServiceContext{Auth: fake}, &pb.LoginResp{User: &pb.UserInfo{UserId: 1, Role: pb.UserRole_USER_ROLE_USER, Status: pb.UserStatus_USER_STATUS_NORMAL}, Tokens: &pb.TokenPair{AccessToken: "new-access", RefreshToken: "new-refresh", BrowserSessionToken: "new-browser"}})
	if status.Code(err) != codes.PermissionDenied || len(writer.Result().Cookies()) != 0 {
		t.Fatal("rejected login replaced original browser session", err)
	}
	if fake.revoked == nil || fake.revoked.AccessToken != "new-access" || len(fake.authorization) != 0 {
		t.Fatal("logout targeted previous login instead of rejected login")
	}
}
