// Code scaffolded by goctl. Not to edit.

package auth

import (
	"github.com/zeromicro/go-zero/core/logx"

	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/logic/auth"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/types"
)

func ResetPasswordHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		var (
			in  = new(types.ResetPasswordReq)
			ctx = r.Context()
		)

		if err := httpx.Parse(r, in); err != nil {
			logx.WithContext(ctx).Errorf("parse params failed: %v", err)
			httpx.Error(w, err)
			return
		}

		if err := svcCtx.Validator.Struct(in); err != nil {
			logx.WithContext(ctx).Errorf("validate params failed: %v", err)
			httpx.Error(w, err)
			return
		}

		l := auth.NewResetPasswordLogic(ctx, svcCtx)
		resp, err := l.ResetPassword(in)
		if err != nil {
			httpx.Error(w, err)
		} else {
			httpx.OkJson(w, resp)
		}
	}
}
