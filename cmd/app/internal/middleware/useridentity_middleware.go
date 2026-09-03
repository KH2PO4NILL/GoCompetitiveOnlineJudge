// Code scaffolded by goctl. Not to edit.

package middleware

import (
	"net/http"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/internal/httpbridge"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/client/userservice"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"

	"github.com/zeromicro/go-zero/rest/httpx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UserIdentityMiddleware struct {
	users userservice.UserService
}

func NewUserIdentityMiddleware(users userservice.UserService) *UserIdentityMiddleware {
	return &UserIdentityMiddleware{users: users}
}

func (m *UserIdentityMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, ok := httpbridge.BearerToken(r.Header.Get("Authorization"))
		if !ok {
			httpx.ErrorCtx(r.Context(), w, status.Error(codes.Unauthenticated, "valid bearer token required"))
			return
		}
		// ValidateToken 是服务间内部 RPC，HTTP 网关不能直接调用。
		// GetCurrentUser 使用当前 Bearer Token 完成同等身份检查，且不会突破服务鉴权边界。
		response, err := m.users.GetCurrentUser(r.Context(), &pb.GetCurrentUserReq{})
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if response == nil || response.User == nil {
			httpx.ErrorCtx(r.Context(), w, status.Error(codes.Unavailable, "authentication service returned an invalid response"))
			return
		}
		if response.User.Status != pb.UserStatus_USER_STATUS_NORMAL {
			httpx.ErrorCtx(r.Context(), w, status.Error(codes.PermissionDenied, "user account is unavailable"))
			return
		}
		next(w, r)
	}
}
