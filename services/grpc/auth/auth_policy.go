package main

import (
	"strings"

	sharedauth "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"
)

func authMethodAuthentication(fullMethod string) sharedauth.AuthenticationMode {
	switch fullMethod {
	case pb.AuthService_EmailRegister_FullMethodName,
		pb.AuthService_UsernameLogin_FullMethodName,
		pb.AuthService_EmailPasswordLogin_FullMethodName,
		pb.AuthService_EmailCodeLogin_FullMethodName,
		pb.AuthService_ValidateToken_FullMethodName,
		pb.AuthService_RefreshToken_FullMethodName,
		pb.AuthService_ResetPassword_FullMethodName:
		return sharedauth.AuthenticationSkipped
	default:
		return sharedauth.AuthenticationRequired
	}
}

func authRequestAuthentication(fullMethod string, request any) sharedauth.AuthenticationMode {
	switch fullMethod {
	case pb.AuthService_SendEmailCode_FullMethodName:
		in, ok := request.(*pb.SendEmailCodeReq)
		if ok && in.Purpose != pb.EmailCodePurpose_EMAIL_CODE_PURPOSE_CHANGE_EMAIL {
			return sharedauth.AuthenticationSkipped
		}
	case pb.AuthService_Logout_FullMethodName:
		in, ok := request.(*pb.LogoutReq)
		if ok && (strings.TrimSpace(in.AccessToken) != "" || strings.TrimSpace(in.RefreshToken) != "") {
			// Prefer a valid Authorization header, but let LogoutLogic validate a
			// body access/refresh token when the header is missing or expired.
			return sharedauth.AuthenticationAlternative
		}
	}
	return authMethodAuthentication(fullMethod)
}
