package user

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/mapper"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"
)

func listUsers(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ListUsersReq) (*types.ListUsersResp, error) {
	statusValue, err := mapper.UserStatus(req.Status)
	if err != nil {
		return nil, err
	}
	role, err := mapper.UserRole(req.Role)
	if err != nil {
		return nil, err
	}
	response, err := svcCtx.Users.ListUsers(ctx, &pb.ListUsersReq{
		Page: req.Page, PageSize: req.PageSize, Status: statusValue,
		Role: role, SchoolId: req.SchoolID, Keyword: req.Keyword,
	})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("user list")
	}
	users := make([]types.UserInfo, 0, len(response.Users))
	for _, item := range response.Users {
		user, err := mapper.User(item)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return &types.ListUsersResp{Total: response.Total, Users: users}, nil
}

func getUser(ctx context.Context, svcCtx *svc.ServiceContext, req *types.GetUserReq) (*types.GetUserResp, error) {
	response, err := svcCtx.Users.GetUserById(ctx, &pb.GetUserByIdReq{UserId: req.UserID})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("user response")
	}
	user, err := mapper.User(response.User)
	if err != nil {
		return nil, err
	}
	return &types.GetUserResp{User: user}, nil
}

func adminUpdateUser(ctx context.Context, svcCtx *svc.ServiceContext, req *types.AdminUpdateUserReq) (*types.AdminUpdateUserResp, error) {
	role, err := mapper.OptionalUserRole(req.Role)
	if err != nil {
		return nil, err
	}
	statusValue, err := mapper.OptionalUserStatus(req.Status)
	if err != nil {
		return nil, err
	}
	response, err := svcCtx.Users.AdminUpdateUser(ctx, &pb.AdminUpdateUserReq{
		UserId: req.UserID, Role: role, Status: statusValue,
	})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, mapper.InvalidRPCResponse("user response")
	}
	user, err := mapper.User(response.User)
	if err != nil {
		return nil, err
	}
	return &types.AdminUpdateUserResp{User: user}, nil
}

func deleteUser(ctx context.Context, svcCtx *svc.ServiceContext, req *types.DeleteUserReq) (*types.EmptyResp, error) {
	_, err := svcCtx.Users.DeleteUser(ctx, &pb.DeleteUserReq{UserId: req.UserID})
	if err != nil {
		return nil, err
	}
	return &types.EmptyResp{}, nil
}
