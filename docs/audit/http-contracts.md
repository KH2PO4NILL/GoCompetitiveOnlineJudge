# HTTP 契约清单

2026-09-05 从 `.api` 服务声明提取。middleware 为空不代表无 RPC 授权；例如匿名发送验证码仍受用途和频率限制。分页、枚举和 revision 以表内 DTO 定义为准；既有 HTTP 测试位于对应 `internal/logic/*/operations_test.go`，Mapper 与中间件另有独立测试。此清单不是浏览器验收记录。

## app

源：`cmd/app/desc/app.api`

| 方法 | 路径 | 请求 DTO | 响应 DTO | HTTP 中间件 |
| --- | --- | --- | --- | --- |
| GET | `/healthz` | — | HealthResp | 无（下游仍鉴权） |
| POST | `/api/v1/auth/login/username` | UsernameLoginReq | LoginResp | 无（下游仍鉴权） |
| POST | `/api/v1/auth/login/email-password` | EmailPasswordLoginReq | LoginResp | 无（下游仍鉴权） |
| POST | `/api/v1/auth/login/email-code` | EmailCodeLoginReq | LoginResp | 无（下游仍鉴权） |
| POST | `/api/v1/auth/register` | EmailRegisterReq | RegisterResp | 无（下游仍鉴权） |
| POST | `/api/v1/auth/email-codes` | SendEmailCodeReq | SendEmailCodeResp | 无（下游仍鉴权） |
| POST | `/api/v1/auth/token/refresh` | RefreshTokenReq | RefreshTokenResp | 无（下游仍鉴权） |
| POST | `/api/v1/auth/logout` | LogoutReq | EmptyResp | 无（下游仍鉴权） |
| POST | `/api/v1/auth/password/reset` | ResetPasswordReq | EmptyResp | 无（下游仍鉴权） |
| GET | `/api/v1/users/me` | — | GetCurrentUserResp | UserIdentity |
| PATCH | `/api/v1/users/me` | UpdateProfileReq | UpdateProfileResp | UserIdentity |
| PUT | `/api/v1/users/me/email` | ChangeEmailReq | ChangeEmailResp | UserIdentity |
| PUT | `/api/v1/users/me/password` | ChangePasswordReq | EmptyResp | UserIdentity |
| GET | `/api/v1/problems` | ListProblemsReq | ListProblemsResp | 无（下游仍鉴权） |
| GET | `/api/v1/problems/:problem_id` | GetProblemReq | GetProblemResp | 无（下游仍鉴权） |
| GET | `/api/v1/contests` | ListContestsReq | ListContestsResp | 无（下游仍鉴权） |
| GET | `/api/v1/contests/:contest_id` | GetContestReq | GetContestResp | 无（下游仍鉴权） |
| GET | `/api/v1/contests/:contest_id/problems` | ListContestProblemsReq | ListContestProblemsResp | 无（下游仍鉴权） |
| GET | `/api/v1/contests/:contest_id/scoreboard` | GetScoreboardReq | GetScoreboardResp | 无（下游仍鉴权） |
| POST | `/api/v1/contests/:contest_id/registrations` | RegisterContestReq | RegisterContestResp | UserIdentity |
| POST | `/api/v1/contests/:contest_id/registrations/cancel` | CancelRegistrationReq | CancelRegistrationResp | UserIdentity |
| GET | `/api/v1/contests/:contest_id/registrations/me` | GetMyContestStatusReq | GetMyContestStatusResp | UserIdentity |
| POST | `/api/v1/submissions` | CreateSubmissionReq | CreateSubmissionResp | UserIdentity |
| GET | `/api/v1/submissions` | ListSubmissionsReq | ListSubmissionsResp | UserIdentity |
| GET | `/api/v1/submissions/:submission_id` | GetSubmissionReq | GetSubmissionResp | UserIdentity |

## admin

源：`cmd/admin/desc/admin.api`

| 方法 | 路径 | 请求 DTO | 响应 DTO | HTTP 中间件 |
| --- | --- | --- | --- | --- |
| GET | `/healthz` | — | HealthResp | 无（下游仍鉴权） |
| POST | `/api/v1/admin/auth/login/username` | UsernameLoginReq | LoginResp | 无（下游仍鉴权） |
| POST | `/api/v1/admin/auth/login/email-password` | EmailPasswordLoginReq | LoginResp | 无（下游仍鉴权） |
| POST | `/api/v1/admin/auth/login/email-code` | EmailCodeLoginReq | LoginResp | 无（下游仍鉴权） |
| POST | `/api/v1/admin/auth/email-codes` | SendEmailCodeReq | SendEmailCodeResp | 无（下游仍鉴权） |
| POST | `/api/v1/admin/auth/token/refresh` | RefreshTokenReq | RefreshTokenResp | 无（下游仍鉴权） |
| POST | `/api/v1/admin/auth/logout` | LogoutReq | EmptyResp | 无（下游仍鉴权） |
| POST | `/api/v1/admin/auth/password/reset` | ResetPasswordReq | EmptyResp | 无（下游仍鉴权） |
| GET | `/api/v1/admin/users/me` | — | GetCurrentUserResp | AdminIdentity |
| PATCH | `/api/v1/admin/users/me` | UpdateProfileReq | UpdateProfileResp | AdminIdentity |
| PUT | `/api/v1/admin/users/me/email` | ChangeEmailReq | ChangeEmailResp | AdminIdentity |
| PUT | `/api/v1/admin/users/me/password` | ChangePasswordReq | EmptyResp | AdminIdentity |
| GET | `/api/v1/admin/users` | ListUsersReq | ListUsersResp | AdminIdentity |
| GET | `/api/v1/admin/users/:user_id` | GetUserReq | GetUserResp | AdminIdentity |
| PATCH | `/api/v1/admin/users/:user_id` | AdminUpdateUserReq | AdminUpdateUserResp | AdminIdentity |
| DELETE | `/api/v1/admin/users/:user_id` | DeleteUserReq | EmptyResp | AdminIdentity |
| GET | `/api/v1/admin/problems` | ListProblemsReq | ListProblemsResp | AdminIdentity |
| POST | `/api/v1/admin/problems` | CreateProblemReq | CreateProblemResp | AdminIdentity |
| GET | `/api/v1/admin/problems/:problem_id` | GetProblemForEditReq | GetProblemForEditResp | AdminIdentity |
| PATCH | `/api/v1/admin/problems/:problem_id` | UpdateProblemReq | UpdateProblemResp | AdminIdentity |
| POST | `/api/v1/admin/problems/:problem_id/publish` | PublishProblemReq | PublishProblemResp | AdminIdentity |
| POST | `/api/v1/admin/problems/:problem_id/archive` | ArchiveProblemReq | ArchiveProblemResp | AdminIdentity |
| DELETE | `/api/v1/admin/problems/:problem_id` | DeleteProblemReq | DeleteProblemResp | AdminIdentity |
| POST | `/api/v1/admin/problems/:problem_id/restore` | RestoreProblemReq | RestoreProblemResp | AdminIdentity |
| POST | `/api/v1/admin/problems/:problem_id/test-data/uploads` | CreateTestDataUploadReq | CreateTestDataUploadResp | AdminIdentity |
| POST | `/api/v1/admin/problems/:problem_id/test-data/uploads/:upload_id/finalize` | FinalizeTestDataUploadReq | FinalizeTestDataUploadResp | AdminIdentity |
| GET | `/api/v1/admin/contests` | ListContestsReq | ListContestsResp | AdminIdentity |
| POST | `/api/v1/admin/contests` | CreateContestReq | CreateContestResp | AdminIdentity |
| GET | `/api/v1/admin/contests/:contest_id` | GetContestForEditReq | GetContestForEditResp | AdminIdentity |
| PATCH | `/api/v1/admin/contests/:contest_id` | UpdateContestReq | UpdateContestResp | AdminIdentity |
| POST | `/api/v1/admin/contests/:contest_id/publish` | PublishContestReq | PublishContestResp | AdminIdentity |
| POST | `/api/v1/admin/contests/:contest_id/cancel` | CancelContestReq | CancelContestResp | AdminIdentity |
| POST | `/api/v1/admin/contests/:contest_id/archive` | ArchiveContestReq | ArchiveContestResp | AdminIdentity |
| DELETE | `/api/v1/admin/contests/:contest_id` | DeleteContestReq | DeleteContestResp | AdminIdentity |
| POST | `/api/v1/admin/contests/:contest_id/restore` | RestoreContestReq | RestoreContestResp | AdminIdentity |
| PUT | `/api/v1/admin/contests/:contest_id/problems` | SetContestProblemsReq | SetContestProblemsResp | AdminIdentity |
| GET | `/api/v1/admin/contests/:contest_id/registrations` | ListRegistrationsReq | ListRegistrationsResp | AdminIdentity |
| POST | `/api/v1/admin/contests/:contest_id/registrations/:registration_id/review` | ReviewRegistrationReq | ReviewRegistrationResp | AdminIdentity |
| GET | `/api/v1/admin/contests/:contest_id/participants` | ListParticipantsReq | ListParticipantsResp | AdminIdentity |
| POST | `/api/v1/admin/contests/:contest_id/scoreboard/freeze` | FreezeScoreboardReq | FreezeScoreboardResp | AdminIdentity |
| POST | `/api/v1/admin/contests/:contest_id/scoreboard/unfreeze` | UnfreezeScoreboardReq | UnfreezeScoreboardResp | AdminIdentity |
| GET | `/api/v1/admin/submissions` | ListSubmissionsReq | ListSubmissionsResp | AdminIdentity |
| GET | `/api/v1/admin/submissions/:submission_id` | GetSubmissionReq | GetSubmissionResp | AdminIdentity |
| POST | `/api/v1/admin/submissions/:submission_id/rejudge` | RejudgeSubmissionReq | RejudgeSubmissionResp | AdminIdentity |
| POST | `/api/v1/admin/submissions/rejudge` | BatchRejudgeSubmissionsReq | BatchRejudgeSubmissionsResp | AdminIdentity |
