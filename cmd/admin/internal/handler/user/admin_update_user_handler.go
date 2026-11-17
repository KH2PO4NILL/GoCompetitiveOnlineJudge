// Code scaffolded by goctl. Not to edit.

package user

import (
	"github.com/zeromicro/go-zero/core/logx"

	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/logic/user"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"
)

func AdminUpdateUserHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		var (
			in  = new(types.AdminUpdateUserReq)
			ctx = r.Context()
		)

		if err := httpx.Parse(r, in); err != nil {
			logx.WithContext(ctx).Error("parse admin user update params failed")
			httpx.Error(w, err)
			return
		}

		if err := svcCtx.Validator.Struct(in); err != nil {
			logx.WithContext(ctx).Error("validate admin user update params failed")
			httpx.Error(w, err)
			return
		}

		l := user.NewAdminUpdateUserLogic(ctx, svcCtx)
		resp, err := l.AdminUpdateUser(in)
		if err != nil {
			httpx.Error(w, err)
		} else {
			httpx.OkJson(w, resp)
		}
	}
}
