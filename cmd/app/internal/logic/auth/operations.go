package auth

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/mapper"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/types"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"
)

func usernameLogin(ctx context.Context, svcCtx *svc.ServiceContext, req *types.UsernameLoginReq) (*types.LoginResp, error) {
	response, err := svcCtx.Auth.UsernameLogin(ctx, &pb.UsernameLoginReq{Username: req.Username, Password: req.Password})
	if err != nil {
		return nil, err
	}
	return mapper.Login(response)
}

func emailPasswordLogin(ctx context.Context, svcCtx *svc.ServiceContext, req *types.EmailPasswordLoginReq) (*types.LoginResp, error) {
	response, err := svcCtx.Auth.EmailPasswordLogin(ctx, &pb.EmailPasswordLoginReq{Email: req.Email, Password: req.Password})
	if err != nil {
		return nil, err
	}
	return mapper.Login(response)
}

func emailCodeLogin(ctx context.Context, svcCtx *svc.ServiceContext, req *types.EmailCodeLoginReq) (*types.LoginResp, error) {
	response, err := svcCtx.Auth.EmailCodeLogin(ctx, &pb.EmailCodeLoginReq{Email: req.Email, Code: req.Code})
	if err != nil {
		return nil, err
	}
	return mapper.Login(response)
}

func emailRegister(ctx context.Context, svcCtx *svc.ServiceContext, req *types.EmailRegisterReq) (*types.RegisterResp, error) {
	response, err := svcCtx.Auth.EmailRegister(ctx, &pb.EmailRegisterReq{
		Email: req.Email, Username: req.Username, Password: req.Password, Code: req.Code, SchoolId: req.SchoolID,
	})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("register response")
	}
	user, err := mapper.User(response.User)
	if err != nil {
		return nil, err
	}
	return &types.RegisterResp{User: user}, nil
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
	response, err := svcCtx.Auth.RefreshToken(ctx, &pb.RefreshTokenReq{RefreshToken: req.RefreshToken})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("refresh response")
	}
	tokens, err := mapper.TokenPair(response.Tokens)
	if err != nil {
		return nil, err
	}
	return &types.RefreshTokenResp{Tokens: tokens}, nil
}

func logout(ctx context.Context, svcCtx *svc.ServiceContext, req *types.LogoutReq) (*types.EmptyResp, error) {
	_, err := svcCtx.Auth.Logout(ctx, &pb.LogoutReq{AccessToken: req.AccessToken, RefreshToken: req.RefreshToken})
	if err != nil {
		return nil, err
	}
	return &types.EmptyResp{}, nil
}

func resetPassword(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ResetPasswordReq) (*types.EmptyResp, error) {
	_, err := svcCtx.Auth.ResetPassword(ctx, &pb.ResetPasswordReq{
		Email: req.Email, Code: req.Code, NewPassword: req.NewPassword,
	})
	if err != nil {
		return nil, err
	}
	return &types.EmptyResp{}, nil
}
