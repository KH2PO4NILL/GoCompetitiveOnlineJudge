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

type AdminIdentityMiddleware struct {
	users userservice.UserService
}

func NewAdminIdentityMiddleware(users userservice.UserService) *AdminIdentityMiddleware {
	return &AdminIdentityMiddleware{users: users}
}

func (m *AdminIdentityMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, ok := httpbridge.BearerToken(r.Header.Get("Authorization"))
		if !ok {
			httpx.ErrorCtx(r.Context(), w, status.Error(codes.Unauthenticated, "valid bearer token required"))
			return
		}
		// ValidateToken 只允许受信任内部服务调用；HTTP 网关通过用户态 RPC 校验身份。
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
		role := response.User.Role
		if role != pb.UserRole_USER_ROLE_ADMIN && role != pb.UserRole_USER_ROLE_SUPER_ADMIN {
			httpx.ErrorCtx(r.Context(), w, status.Error(codes.PermissionDenied, "administrator permission required"))
			return
		}
		next(w, r)
	}
}
