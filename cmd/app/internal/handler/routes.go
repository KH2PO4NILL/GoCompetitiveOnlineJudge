// Code scaffolded by goctl. Not to edit.

package handler

import (
	"net/http"

	account "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/handler/account"
	auth "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/handler/auth"
	contest "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/handler/contest"
	contestaccount "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/handler/contestaccount"
	health "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/handler/health"
	problem "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/handler/problem"
	submission "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/handler/submission"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/svc"

	"github.com/zeromicro/go-zero/rest"
)

func RegisterHandlers(server *rest.Server, serverCtx *svc.ServiceContext) {
	server.AddRoutes(
		[]rest.Route{
			{
				Method:  http.MethodGet,
				Path:    "/healthz",
				Handler: health.HealthHandler(serverCtx),
			},
		},
	)

	server.AddRoutes(
		[]rest.Route{
			{
				Method:  http.MethodPost,
				Path:    "/auth/login/username",
				Handler: auth.UsernameLoginHandler(serverCtx),
			},
			{
				Method:  http.MethodPost,
				Path:    "/auth/login/email-password",
				Handler: auth.EmailPasswordLoginHandler(serverCtx),
			},
			{
				Method:  http.MethodPost,
				Path:    "/auth/login/email-code",
				Handler: auth.EmailCodeLoginHandler(serverCtx),
			},
			{
				Method:  http.MethodPost,
				Path:    "/auth/register",
				Handler: auth.EmailRegisterHandler(serverCtx),
			},
			{
				Method:  http.MethodPost,
				Path:    "/auth/email-codes",
				Handler: auth.SendEmailCodeHandler(serverCtx),
			},
			{
				Method:  http.MethodPost,
				Path:    "/auth/token/refresh",
				Handler: auth.RefreshTokenHandler(serverCtx),
			},
			{
				Method:  http.MethodPost,
				Path:    "/auth/logout",
				Handler: auth.LogoutHandler(serverCtx),
			},
			{
				Method:  http.MethodPost,
				Path:    "/auth/password/reset",
				Handler: auth.ResetPasswordHandler(serverCtx),
			},
		},
		rest.WithPrefix("/api/v1"),
	)

	server.AddRoutes(
		rest.WithMiddlewares(
			[]rest.Middleware{serverCtx.UserIdentity},
			[]rest.Route{
				{
					Method:  http.MethodGet,
					Path:    "/users/me",
					Handler: account.GetCurrentUserHandler(serverCtx),
				},
				{
					Method:  http.MethodPatch,
					Path:    "/users/me",
					Handler: account.UpdateProfileHandler(serverCtx),
				},
				{
					Method:  http.MethodPut,
					Path:    "/users/me/email",
					Handler: account.ChangeEmailHandler(serverCtx),
				},
				{
					Method:  http.MethodPut,
					Path:    "/users/me/password",
					Handler: account.ChangePasswordHandler(serverCtx),
				},
			}...,
		),
		rest.WithPrefix("/api/v1"),
	)

	server.AddRoutes(
		[]rest.Route{
			{
				Method:  http.MethodGet,
				Path:    "/problems",
				Handler: problem.ListProblemsHandler(serverCtx),
			},
			{
				Method:  http.MethodGet,
				Path:    "/problems/:problem_id",
				Handler: problem.GetProblemHandler(serverCtx),
			},
		},
		rest.WithPrefix("/api/v1"),
	)

	server.AddRoutes(
		[]rest.Route{
			{
				Method:  http.MethodGet,
				Path:    "/contests",
				Handler: contest.ListContestsHandler(serverCtx),
			},
			{
				Method:  http.MethodGet,
				Path:    "/contests/:contest_id",
				Handler: contest.GetContestHandler(serverCtx),
			},
			{
				Method:  http.MethodGet,
				Path:    "/contests/:contest_id/problems",
				Handler: contest.ListContestProblemsHandler(serverCtx),
			},
			{
				Method:  http.MethodGet,
				Path:    "/contests/:contest_id/scoreboard",
				Handler: contest.GetScoreboardHandler(serverCtx),
			},
		},
		rest.WithPrefix("/api/v1"),
	)

	server.AddRoutes(
		rest.WithMiddlewares(
			[]rest.Middleware{serverCtx.UserIdentity},
			[]rest.Route{
				{
					Method:  http.MethodPost,
					Path:    "/contests/:contest_id/registrations",
					Handler: contestaccount.RegisterContestHandler(serverCtx),
				},
				{
					Method:  http.MethodPost,
					Path:    "/contests/:contest_id/registrations/cancel",
					Handler: contestaccount.CancelRegistrationHandler(serverCtx),
				},
				{
					Method:  http.MethodGet,
					Path:    "/contests/:contest_id/registrations/me",
					Handler: contestaccount.GetMyContestStatusHandler(serverCtx),
				},
			}...,
		),
		rest.WithPrefix("/api/v1"),
	)

	server.AddRoutes(
		rest.WithMiddlewares(
			[]rest.Middleware{serverCtx.UserIdentity},
			[]rest.Route{
				{
					Method:  http.MethodPost,
					Path:    "/submissions",
					Handler: submission.CreateSubmissionHandler(serverCtx),
				},
				{
					Method:  http.MethodGet,
					Path:    "/submissions",
					Handler: submission.ListSubmissionsHandler(serverCtx),
				},
				{
					Method:  http.MethodGet,
					Path:    "/submissions/:submission_id",
					Handler: submission.GetSubmissionHandler(serverCtx),
				},
			}...,
		),
		rest.WithPrefix("/api/v1"),
	)
}
