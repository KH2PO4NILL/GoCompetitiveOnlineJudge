// Code scaffolded by goctl. Not to edit.

package account

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/logic/account"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
)

func AdminGetCurrentUserHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		var (
			ctx = r.Context()
		)

		l := account.NewAdminGetCurrentUserLogic(ctx, svcCtx)
		resp, err := l.AdminGetCurrentUser()
		if err != nil {
			httpx.Error(w, err)
		} else {
			httpx.OkJson(w, resp)
		}
	}
}
