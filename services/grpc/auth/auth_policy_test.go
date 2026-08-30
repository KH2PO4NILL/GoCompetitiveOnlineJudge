package main

import (
	"testing"

	sharedauth "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"
)

func TestAuthRequestAuthentication(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		request  any
		wantMode sharedauth.AuthenticationMode
	}{
		{name: "login is public", method: pb.AuthService_UsernameLogin_FullMethodName, request: &pb.UsernameLoginReq{}, wantMode: sharedauth.AuthenticationSkipped},
		{name: "change email code is protected", method: pb.AuthService_SendEmailCode_FullMethodName, request: &pb.SendEmailCodeReq{Purpose: pb.EmailCodePurpose_EMAIL_CODE_PURPOSE_CHANGE_EMAIL}, wantMode: sharedauth.AuthenticationRequired},
		{name: "register code is public", method: pb.AuthService_SendEmailCode_FullMethodName, request: &pb.SendEmailCodeReq{Purpose: pb.EmailCodePurpose_EMAIL_CODE_PURPOSE_REGISTER}, wantMode: sharedauth.AuthenticationSkipped},
		{name: "refresh logout uses alternative credential", method: pb.AuthService_Logout_FullMethodName, request: &pb.LogoutReq{RefreshToken: "refresh"}, wantMode: sharedauth.AuthenticationAlternative},
		{name: "header logout remains protected", method: pb.AuthService_Logout_FullMethodName, request: &pb.LogoutReq{}, wantMode: sharedauth.AuthenticationRequired},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := authRequestAuthentication(test.method, test.request); got != test.wantMode {
				t.Fatalf("authRequestAuthentication() = %v, want %v", got, test.wantMode)
			}
		})
	}
}
