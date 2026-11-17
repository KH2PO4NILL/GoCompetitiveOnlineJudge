package auth

import (
	"context"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/internal/httpbridge"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/mapper"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"
)

func usernameLogin(ctx context.Context, svcCtx *svc.ServiceContext, req *types.UsernameLoginReq) (*types.LoginResp, error) {
	response, err := svcCtx.Auth.UsernameLogin(ctx, &pb.UsernameLoginReq{Username: req.Username, Password: req.Password})
	if err != nil {
		return nil, err
	}
	return adminLoginResponse(ctx, svcCtx, response)
}

func emailPasswordLogin(ctx context.Context, svcCtx *svc.ServiceContext, req *types.EmailPasswordLoginReq) (*types.LoginResp, error) {
	response, err := svcCtx.Auth.EmailPasswordLogin(ctx, &pb.EmailPasswordLoginReq{Email: req.Email, Password: req.Password})
	if err != nil {
		return nil, err
	}
	return adminLoginResponse(ctx, svcCtx, response)
}

func emailCodeLogin(ctx context.Context, svcCtx *svc.ServiceContext, req *types.EmailCodeLoginReq) (*types.LoginResp, error) {
	response, err := svcCtx.Auth.EmailCodeLogin(ctx, &pb.EmailCodeLoginReq{Email: req.Email, Code: req.Code})
	if err != nil {
		return nil, err
	}
	return adminLoginResponse(ctx, svcCtx, response)
}

func sendEmailCode(ctx context.Context, svcCtx *svc.ServiceContext, req *types.SendEmailCodeReq) (*types.SendEmailCodeResp, error) {
	purpose, err := mapper.EmailCodePurpose(req.Purpose)
	if err != nil {
		return nil, err
	}
	response, err := svcCtx.Auth.SendEmailCode(ctx, &pb.SendEmailCodeReq{Email: req.Email, Purpose: purpose})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("email code response")
	}
	return &types.SendEmailCodeResp{ExpiresIn: response.ExpiresIn, RetryAfter: response.RetryAfter}, nil
}

func refreshToken(ctx context.Context, svcCtx *svc.ServiceContext, req *types.RefreshTokenReq) (*types.RefreshTokenResp, error) {
	response, err := svcCtx.Auth.RefreshToken(ctx, &pb.RefreshTokenReq{RefreshToken: req.RefreshToken, BrowserSession: httpbridge.UsesBrowserSession(ctx)})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("refresh response")
	}
	httpbridge.CaptureBrowserSession(ctx, response.GetTokens().GetBrowserSessionToken())
	tokens, err := mapper.TokenPair(response.Tokens)
	if err != nil {
		return nil, err
	}
	return &types.RefreshTokenResp{Tokens: tokens}, nil
}

func logout(ctx context.Context, svcCtx *svc.ServiceContext, req *types.LogoutReq) (*types.EmptyResp, error) {
	input := &pb.LogoutReq{AccessToken: req.AccessToken, RefreshToken: req.RefreshToken}
	if httpbridge.UsesBrowserSession(ctx) {
		input.BrowserSessionToken = req.RefreshToken
		input.RefreshToken = ""
	}
	_, err := svcCtx.Auth.Logout(ctx, input)
	if err != nil {
		return nil, err
	}
	return &types.EmptyResp{}, nil
}

func resetPassword(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ResetPasswordReq) (*types.EmptyResp, error) {
	_, err := svcCtx.Auth.ResetPassword(ctx, &pb.ResetPasswordReq{Email: req.Email, Code: req.Code, NewPassword: req.NewPassword})
	if err != nil {
		return nil, err
	}
	return &types.EmptyResp{}, nil
}

func adminLoginResponse(ctx context.Context, services *svc.ServiceContext, response *pb.LoginResp) (*types.LoginResp, error) {
	result, err := mapper.Login(response)
	if err != nil {
		return nil, err
	}
	if result.User.Status != "normal" || (result.User.Role != "admin" && result.User.Role != "super_admin") {
		// Revoke only the just-issued login, never an older Authorization header or cookie.
		revokeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		md, _ := metadata.FromOutgoingContext(revokeCtx)
		md = md.Copy()
		md.Delete("authorization")
		revokeCtx = metadata.NewOutgoingContext(revokeCtx, md)
		_, _ = services.Auth.Logout(revokeCtx, &pb.LogoutReq{AccessToken: response.Tokens.AccessToken, RefreshToken: response.Tokens.RefreshToken})
		return nil, status.Error(codes.PermissionDenied, "此账号没有管理权限")
	}
	httpbridge.CaptureBrowserSession(ctx, response.Tokens.BrowserSessionToken)
	return result, nil
}
