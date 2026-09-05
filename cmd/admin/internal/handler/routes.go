// Code scaffolded by goctl. Not to edit.

package handler

import (
	"net/http"

	account "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/handler/account"
	auth "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/handler/auth"
	contest "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/handler/contest"
	health "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/handler/health"
	problem "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/handler/problem"
	submission "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/handler/submission"
	user "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/handler/user"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"

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
				Handler: auth.AdminUsernameLoginHandler(serverCtx),
			},
			{
				Method:  http.MethodPost,
				Path:    "/auth/login/email-password",
				Handler: auth.AdminEmailPasswordLoginHandler(serverCtx),
			},
			{
				Method:  http.MethodPost,
				Path:    "/auth/login/email-code",
				Handler: auth.AdminEmailCodeLoginHandler(serverCtx),
			},
			{
				Method:  http.MethodPost,
				Path:    "/auth/email-codes",
				Handler: auth.AdminSendEmailCodeHandler(serverCtx),
			},
			{
				Method:  http.MethodPost,
				Path:    "/auth/token/refresh",
				Handler: auth.AdminRefreshTokenHandler(serverCtx),
			},
			{
				Method:  http.MethodPost,
				Path:    "/auth/logout",
				Handler: auth.AdminLogoutHandler(serverCtx),
			},
			{
				Method:  http.MethodPost,
				Path:    "/auth/password/reset",
				Handler: auth.AdminResetPasswordHandler(serverCtx),
			},
		},
		rest.WithPrefix("/api/v1/admin"),
	)

	server.AddRoutes(
		rest.WithMiddlewares(
			[]rest.Middleware{serverCtx.AdminIdentity},
			[]rest.Route{
				{
					Method:  http.MethodGet,
					Path:    "/users/me",
					Handler: account.AdminGetCurrentUserHandler(serverCtx),
				},
				{
					Method:  http.MethodPatch,
					Path:    "/users/me",
					Handler: account.AdminUpdateProfileHandler(serverCtx),
				},
				{
					Method:  http.MethodPut,
					Path:    "/users/me/email",
					Handler: account.AdminChangeEmailHandler(serverCtx),
				},
				{
					Method:  http.MethodPut,
					Path:    "/users/me/password",
					Handler: account.AdminChangePasswordHandler(serverCtx),
				},
			}...,
		),
		rest.WithPrefix("/api/v1/admin"),
	)

	server.AddRoutes(
		rest.WithMiddlewares(
			[]rest.Middleware{serverCtx.AdminIdentity},
			[]rest.Route{
				{
					Method:  http.MethodGet,
					Path:    "/users",
					Handler: user.ListUsersHandler(serverCtx),
				},
				{
					Method:  http.MethodGet,
					Path:    "/users/:user_id",
					Handler: user.GetUserHandler(serverCtx),
				},
				{
					Method:  http.MethodPatch,
					Path:    "/users/:user_id",
					Handler: user.AdminUpdateUserHandler(serverCtx),
				},
				{
					Method:  http.MethodDelete,
					Path:    "/users/:user_id",
					Handler: user.DeleteUserHandler(serverCtx),
				},
			}...,
		),
		rest.WithPrefix("/api/v1/admin"),
	)

	server.AddRoutes(
		rest.WithMiddlewares(
			[]rest.Middleware{serverCtx.AdminIdentity},
			[]rest.Route{
				{
					Method:  http.MethodGet,
					Path:    "/problems",
					Handler: problem.AdminListProblemsHandler(serverCtx),
				},
				{
					Method:  http.MethodPost,
					Path:    "/problems",
					Handler: problem.CreateProblemHandler(serverCtx),
				},
				{
					Method:  http.MethodGet,
					Path:    "/problems/:problem_id",
					Handler: problem.GetProblemForEditHandler(serverCtx),
				},
				{
					Method:  http.MethodPatch,
					Path:    "/problems/:problem_id",
					Handler: problem.UpdateProblemHandler(serverCtx),
				},
				{
					Method:  http.MethodPost,
					Path:    "/problems/:problem_id/publish",
					Handler: problem.PublishProblemHandler(serverCtx),
				},
				{
					Method:  http.MethodPost,
					Path:    "/problems/:problem_id/archive",
					Handler: problem.ArchiveProblemHandler(serverCtx),
				},
				{
					Method:  http.MethodDelete,
					Path:    "/problems/:problem_id",
					Handler: problem.DeleteProblemHandler(serverCtx),
				},
				{
					Method:  http.MethodPost,
					Path:    "/problems/:problem_id/restore",
					Handler: problem.RestoreProblemHandler(serverCtx),
				},
				{
					Method:  http.MethodPost,
					Path:    "/problems/:problem_id/test-data/uploads",
					Handler: problem.CreateTestDataUploadHandler(serverCtx),
				},
				{
					Method:  http.MethodPost,
					Path:    "/problems/:problem_id/test-data/uploads/:upload_id/finalize",
					Handler: problem.FinalizeTestDataUploadHandler(serverCtx),
				},
			}...,
		),
		rest.WithPrefix("/api/v1/admin"),
	)

	server.AddRoutes(
		rest.WithMiddlewares(
			[]rest.Middleware{serverCtx.AdminIdentity},
			[]rest.Route{
				{
					Method:  http.MethodGet,
					Path:    "/contests",
					Handler: contest.AdminListContestsHandler(serverCtx),
				},
				{
					Method:  http.MethodPost,
					Path:    "/contests",
					Handler: contest.CreateContestHandler(serverCtx),
				},
				{
					Method:  http.MethodGet,
					Path:    "/contests/:contest_id",
					Handler: contest.GetContestForEditHandler(serverCtx),
				},
				{
					Method:  http.MethodPatch,
					Path:    "/contests/:contest_id",
					Handler: contest.UpdateContestHandler(serverCtx),
				},
				{
					Method:  http.MethodPost,
					Path:    "/contests/:contest_id/publish",
					Handler: contest.PublishContestHandler(serverCtx),
				},
				{
					Method:  http.MethodPost,
					Path:    "/contests/:contest_id/cancel",
					Handler: contest.CancelContestHandler(serverCtx),
				},
				{
					Method:  http.MethodPost,
					Path:    "/contests/:contest_id/archive",
					Handler: contest.ArchiveContestHandler(serverCtx),
				},
				{
					Method:  http.MethodDelete,
					Path:    "/contests/:contest_id",
					Handler: contest.DeleteContestHandler(serverCtx),
				},
				{
					Method:  http.MethodPost,
					Path:    "/contests/:contest_id/restore",
					Handler: contest.RestoreContestHandler(serverCtx),
				},
				{
					Method:  http.MethodPut,
					Path:    "/contests/:contest_id/problems",
					Handler: contest.SetContestProblemsHandler(serverCtx),
				},
				{
					Method:  http.MethodGet,
					Path:    "/contests/:contest_id/registrations",
					Handler: contest.ListRegistrationsHandler(serverCtx),
				},
				{
					Method:  http.MethodPost,
					Path:    "/contests/:contest_id/registrations/:registration_id/review",
					Handler: contest.ReviewRegistrationHandler(serverCtx),
				},
				{
					Method:  http.MethodGet,
					Path:    "/contests/:contest_id/participants",
					Handler: contest.ListParticipantsHandler(serverCtx),
				},
				{
					Method:  http.MethodPost,
					Path:    "/contests/:contest_id/scoreboard/freeze",
					Handler: contest.FreezeScoreboardHandler(serverCtx),
				},
				{
					Method:  http.MethodPost,
					Path:    "/contests/:contest_id/scoreboard/unfreeze",
					Handler: contest.UnfreezeScoreboardHandler(serverCtx),
				},
			}...,
		),
		rest.WithPrefix("/api/v1/admin"),
	)

	server.AddRoutes(
		rest.WithMiddlewares(
			[]rest.Middleware{serverCtx.AdminIdentity},
			[]rest.Route{
				{
					Method:  http.MethodGet,
					Path:    "/submissions",
					Handler: submission.AdminListSubmissionsHandler(serverCtx),
				},
				{
					Method:  http.MethodGet,
					Path:    "/submissions/:submission_id",
					Handler: submission.AdminGetSubmissionHandler(serverCtx),
				},
				{
					Method:  http.MethodPost,
					Path:    "/submissions/:submission_id/rejudge",
					Handler: submission.RejudgeSubmissionHandler(serverCtx),
				},
				{
					Method:  http.MethodPost,
					Path:    "/submissions/rejudge",
					Handler: submission.BatchRejudgeSubmissionsHandler(serverCtx),
				},
			}...,
		),
		rest.WithPrefix("/api/v1/admin"),
	)
}
