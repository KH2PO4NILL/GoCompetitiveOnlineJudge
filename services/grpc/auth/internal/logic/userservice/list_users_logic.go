package userservicelogic

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ListUsersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListUsersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListUsersLogic {
	return &ListUsersLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListUsersLogic) ListUsers(in *pb.ListUsersReq) (*pb.ListUsersResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "请求参数不能为空")
	}

	actor, err := authenticatedUser(l.ctx, l.svcCtx, l.Logger)
	if err != nil {
		return nil, err
	}
	if !isAdministrator(actor.Role) {
		return nil, status.Error(codes.PermissionDenied, "仅管理员可以查看用户列表")
	}

	page := int(in.Page)
	if page == 0 {
		page = 1
	}
	pageSize := int(in.PageSize)
	if pageSize == 0 {
		pageSize = 20
	}
	if page < 1 || pageSize < 1 || pageSize > 100 {
		return nil, status.Error(codes.InvalidArgument, "分页参数不正确，单页数量必须为 1 到 100")
	}
	if in.Status != pb.UserStatus_USER_STATUS_UNSPECIFIED && !isManageableStatus(in.Status) {
		return nil, status.Error(codes.InvalidArgument, "用户状态筛选条件不正确")
	}
	if in.Role != pb.UserRole_USER_ROLE_UNSPECIFIED && !isAssignableRole(in.Role) {
		return nil, status.Error(codes.InvalidArgument, "用户角色筛选条件不正确")
	}
	if in.SchoolId < 0 {
		return nil, status.Error(codes.InvalidArgument, "学校 ID 不正确")
	}
	keyword := strings.TrimSpace(in.Keyword)
	if !utf8.ValidString(keyword) || utf8.RuneCountInString(keyword) > 100 {
		return nil, status.Error(codes.InvalidArgument, "搜索关键词不能超过 100 个字符")
	}

	// 列表查询始终排除软删除用户，并且只选择响应需要的字段，避免密码哈希进入业务层内存。
	query := l.svcCtx.DB.WithContext(l.ctx).
		Model(&model.Users{}).
		Where("deleted_at IS NULL")
	if in.Status != pb.UserStatus_USER_STATUS_UNSPECIFIED {
		query = query.Where("status = ?", int64(in.Status))
	}
	if in.Role != pb.UserRole_USER_ROLE_UNSPECIFIED {
		query = query.Where("role = ?", int64(in.Role))
	}
	if in.SchoolId > 0 {
		query = query.Where("school_id = ?", in.SchoolId)
	}
	if keyword != "" {
		// LOCATE 按普通文本搜索，不会把用户输入中的 % 或 _ 当作 SQL 通配符。
		query = query.Where("LOCATE(?, username) > 0 OR LOCATE(?, email) > 0", keyword, keyword)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		l.Errorf("count users: %v", err)
		return nil, status.Error(codes.Internal, "读取用户列表失败")
	}

	users := make([]model.Users, 0, pageSize)
	if total > 0 {
		err = query.
			Select("id", "username", "email", "role", "status", "school_id").
			Order("id DESC").
			Limit(pageSize).
			Offset((page - 1) * pageSize).
			Find(&users).Error
		if err != nil {
			l.Errorf("list users: %v", err)
			return nil, status.Error(codes.Internal, "读取用户列表失败")
		}
	}

	responseUsers := make([]*pb.UserInfo, 0, len(users))
	for i := range users {
		responseUsers = append(responseUsers, userInfo(&users[i]))
	}

	return &pb.ListUsersResp{Total: total, Users: responseUsers}, nil
}
