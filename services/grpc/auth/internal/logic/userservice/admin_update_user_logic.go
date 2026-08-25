package userservicelogic

import (
	"context"
	"errors"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	errAdminActorChanged      = errors.New("administrator state changed")
	errAdminTargetNotFound    = errors.New("admin target user not found")
	errAdminCannotManageUser  = errors.New("administrator cannot manage target user")
	errAdminAssignmentTooHigh = errors.New("assigned role is too high")
)

type AdminUpdateUserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAdminUpdateUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminUpdateUserLogic {
	return &AdminUpdateUserLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AdminUpdateUserLogic) AdminUpdateUser(in *pb.AdminUpdateUserReq) (*pb.AdminUpdateUserResp, error) {
	if in == nil || in.UserId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "用户 ID 不正确")
	}
	if in.Role == nil && in.Status == nil {
		return nil, status.Error(codes.InvalidArgument, "至少需要修改角色或状态")
	}
	if in.Role != nil && !isAssignableRole(*in.Role) {
		return nil, status.Error(codes.InvalidArgument, "用户角色不正确")
	}
	if in.Status != nil && !isManageableStatus(*in.Status) {
		return nil, status.Error(codes.InvalidArgument, "用户状态不正确")
	}

	actor, err := authenticatedUser(l.ctx, l.svcCtx, l.Logger)
	if err != nil {
		return nil, err
	}
	if !isAdministrator(actor.Role) {
		return nil, status.Error(codes.PermissionDenied, "仅管理员可以修改用户角色或状态")
	}
	if actor.Id == in.UserId {
		return nil, status.Error(codes.PermissionDenied, "不能通过管理员接口修改自己的角色或状态")
	}

	var updatedUser model.Users
	err = l.svcCtx.DB.WithContext(l.ctx).Transaction(func(tx *gorm.DB) error {
		var lockedActor model.Users
		// 在事务内重新读取并锁定操作者，防止权限检查后管理员角色被并发撤销。
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id", "role", "status").
			Where("id = ? AND deleted_at IS NULL", actor.Id).
			Take(&lockedActor).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errAdminActorChanged
		}
		if err != nil {
			return err
		}
		if !isAdministrator(lockedActor.Role) || pb.UserStatus(lockedActor.Status) != pb.UserStatus_USER_STATUS_NORMAL {
			return errAdminActorChanged
		}

		var target model.Users
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id", "role", "status").
			Where("id = ? AND deleted_at IS NULL", in.UserId).
			Take(&target).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errAdminTargetNotFound
		}
		if err != nil {
			return err
		}

		// 管理员只能管理角色等级严格低于自己的用户，不能操作同级或更高等级账号。
		if target.Role >= lockedActor.Role {
			return errAdminCannotManageUser
		}
		if in.Role != nil && int64(*in.Role) >= lockedActor.Role {
			return errAdminAssignmentTooHigh
		}

		updates := make(map[string]any, 2)
		securityStateChanged := false
		if in.Role != nil && int64(*in.Role) != target.Role {
			updates["role"] = int64(*in.Role)
			securityStateChanged = true
		}
		if in.Status != nil && int64(*in.Status) != target.Status {
			updates["status"] = int64(*in.Status)
			securityStateChanged = true
		}

		if securityStateChanged {
			if err := tx.Model(&model.Users{}).
				Where("id = ? AND deleted_at IS NULL", target.Id).
				Updates(updates).Error; err != nil {
				return err
			}

			// 角色或状态改变后撤销全部 Session，确保旧 Token 中的权限不能继续使用。
			now := time.Now().UTC()
			if err := tx.Model(&model.UserSessions{}).
				Where("user_id = ? AND revoked_at IS NULL", target.Id).
				Updates(map[string]any{
					"revoked_at":     now,
					"revoked_reason": "admin_user_updated",
				}).Error; err != nil {
				return err
			}
		}

		return tx.Select("id", "username", "email", "role", "status", "school_id").
			Where("id = ? AND deleted_at IS NULL", target.Id).
			Take(&updatedUser).Error
	})
	if err != nil {
		switch {
		case errors.Is(err, errAdminActorChanged):
			return nil, status.Error(codes.PermissionDenied, "管理员权限已发生变化")
		case errors.Is(err, errAdminTargetNotFound):
			return nil, status.Error(codes.NotFound, "用户不存在")
		case errors.Is(err, errAdminCannotManageUser):
			return nil, status.Error(codes.PermissionDenied, "不能管理同级或更高角色的用户")
		case errors.Is(err, errAdminAssignmentTooHigh):
			return nil, status.Error(codes.PermissionDenied, "不能授予同级或更高的角色")
		default:
			l.Errorf("admin %d update user %d: %v", actor.Id, in.UserId, err)
			return nil, status.Error(codes.Internal, "管理员修改用户失败")
		}
	}

	// 审计日志只记录操作者和目标用户 ID，不记录用户隐私字段。
	l.Infof("admin %d updated user %d", actor.Id, in.UserId)

	return &pb.AdminUpdateUserResp{User: userInfo(&updatedUser)}, nil
}
