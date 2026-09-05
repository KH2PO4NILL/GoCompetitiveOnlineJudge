package httpbridge

import (
	"sync"

	authpb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"
	contestpb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"
	problempb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"
	submitpb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"
	"github.com/zeromicro/go-zero/zrpc"
)

var registerSensitiveMethodsOnce sync.Once

// RegisterSensitiveRPCMethods 禁止 go-zero 客户端日志记录密码、Token、验证码、
// 源码、邀请码、预签名上传地址和判题存储键等敏感 RPC 内容。
func RegisterSensitiveRPCMethods() {
	registerSensitiveMethodsOnce.Do(func() {
		methods := []string{
			authpb.AuthService_EmailRegister_FullMethodName,
			authpb.AuthService_UsernameLogin_FullMethodName,
			authpb.AuthService_EmailPasswordLogin_FullMethodName,
			authpb.AuthService_EmailCodeLogin_FullMethodName,
			authpb.AuthService_SendEmailCode_FullMethodName,
			authpb.AuthService_ValidateToken_FullMethodName,
			authpb.AuthService_RefreshToken_FullMethodName,
			authpb.AuthService_Logout_FullMethodName,
			authpb.AuthService_ChangePassword_FullMethodName,
			authpb.AuthService_ResetPassword_FullMethodName,
			authpb.UserService_ChangeEmail_FullMethodName,
			problempb.ProblemService_CreateProblem_FullMethodName,
			problempb.ProblemService_GetProblemForEdit_FullMethodName,
			problempb.ProblemService_UpdateProblem_FullMethodName,
			problempb.ProblemService_PublishProblem_FullMethodName,
			problempb.ProblemService_ArchiveProblem_FullMethodName,
			problempb.ProblemService_RestoreProblem_FullMethodName,
			problempb.ProblemService_CreateTestDataUpload_FullMethodName,
			problempb.ProblemService_FinalizeTestDataUpload_FullMethodName,
			contestpb.ContestService_CreateContest_FullMethodName,
			contestpb.ContestService_GetContestForEdit_FullMethodName,
			contestpb.ContestService_UpdateContest_FullMethodName,
			contestpb.ContestService_PublishContest_FullMethodName,
			contestpb.ContestService_CancelContest_FullMethodName,
			contestpb.ContestService_ArchiveContest_FullMethodName,
			contestpb.ContestService_RestoreContest_FullMethodName,
			contestpb.ContestService_SetContestProblems_FullMethodName,
			contestpb.ContestService_RegisterContest_FullMethodName,
			contestpb.ContestService_CancelRegistration_FullMethodName,
			contestpb.ContestService_GetMyContestStatus_FullMethodName,
			contestpb.ContestService_ListRegistrations_FullMethodName,
			contestpb.ContestService_ReviewRegistration_FullMethodName,
			contestpb.ContestService_FreezeScoreboard_FullMethodName,
			contestpb.ContestService_UnfreezeScoreboard_FullMethodName,
			submitpb.SubmitService_CreateSubmission_FullMethodName,
			submitpb.SubmitService_GetSubmission_FullMethodName,
		}
		for _, method := range methods {
			zrpc.DontLogClientContentForMethod(method)
		}
	})
}
