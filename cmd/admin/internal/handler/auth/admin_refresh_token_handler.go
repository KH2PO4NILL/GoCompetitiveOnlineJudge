// Code scaffolded by goctl. Not to edit.

package auth

import (
	"github.com/zeromicro/go-zero/core/logx"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/internal/httpbridge"
	sharedauth "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/logic/auth"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"
)

func AdminRefreshTokenHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		if err := httpbridge.CheckBrowserSession(r); err != nil {
			httpx.Error(w, err)
			return
		}
		var (
			in  = new(types.RefreshTokenReq)
			ctx = httpbridge.WithBrowserSession(r.Context(), w, r)
		)

		if err := httpx.Parse(r, in); err != nil {
			logx.WithContext(ctx).Errorf("parse params failed: %v", err)
			httpx.Error(w, err)
			return
		}

		in.RefreshToken = httpbridge.BrowserRefreshToken(r, in.RefreshToken)
		if in.RefreshToken == "" {
			httpx.Error(w, status.Error(codes.Unauthenticated, "没有可恢复的浏览器会话"))
			return
		}

		if err := svcCtx.Validator.Struct(in); err != nil {
			logx.WithContext(ctx).Errorf("validate params failed: %v", err)
			httpx.Error(w, err)
			return
		}

		l := auth.NewAdminRefreshTokenLogic(ctx, svcCtx)
		resp, err := l.AdminRefreshToken(in)
		if err != nil {
			if reason := sharedauth.AuthenticationReason(status.Convert(err)); reason == "refresh_token_invalid" || reason == "account_unavailable" {
				httpbridge.SetBrowserSession(w, r, "")
			}
			httpx.Error(w, err)
		} else {
			httpx.OkJson(w, resp)
		}
	}
}
