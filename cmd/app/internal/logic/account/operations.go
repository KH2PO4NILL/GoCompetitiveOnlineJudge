package account

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/mapper"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/types"
	authpb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"
)

func getCurrentUser(ctx context.Context, svcCtx *svc.ServiceContext) (*types.GetCurrentUserResp, error) {
	response, err := svcCtx.Users.GetCurrentUser(ctx, &authpb.GetCurrentUserReq{})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("current user response")
	}
	user, err := mapper.User(response.User)
	if err != nil {
		return nil, err
	}
	return &types.GetCurrentUserResp{User: user}, nil
}

func updateProfile(ctx context.Context, svcCtx *svc.ServiceContext, req *types.UpdateProfileReq) (*types.UpdateProfileResp, error) {
	response, err := svcCtx.Users.UpdateProfile(ctx, &authpb.UpdateProfileReq{
		Username: cloneString(req.Username), SchoolId: cloneInt64(req.SchoolID),
	})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("profile response")
	}
	user, err := mapper.User(response.User)
	if err != nil {
		return nil, err
	}
	return &types.UpdateProfileResp{User: user}, nil
}

func changeEmail(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ChangeEmailReq) (*types.ChangeEmailResp, error) {
	response, err := svcCtx.Users.ChangeEmail(ctx, &authpb.ChangeEmailReq{NewEmail: req.NewEmail, Code: req.Code})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("change email response")
	}
	user, err := mapper.User(response.User)
	if err != nil {
		return nil, err
	}
	return &types.ChangeEmailResp{User: user}, nil
}

func changePassword(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ChangePasswordReq) (*types.EmptyResp, error) {
	_, err := svcCtx.Auth.ChangePassword(ctx, &authpb.ChangePasswordReq{
		CurrentPassword: req.CurrentPassword, NewPassword: req.NewPassword,
	})
	if err != nil {
		return nil, err
	}
	return &types.EmptyResp{}, nil
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
