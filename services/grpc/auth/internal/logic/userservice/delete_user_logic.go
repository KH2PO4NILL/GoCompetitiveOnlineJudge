package userservicelogic

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
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
	errDeleteActorChanged   = errors.New("delete user actor state changed")
	errDeleteTargetNotFound = errors.New("delete target user not found")
	errDeleteTargetTooHigh  = errors.New("delete target role is too high")
)

type DeleteUserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteUserLogic {
	return &DeleteUserLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteUserLogic) DeleteUser(in *pb.DeleteUserReq) (*pb.EmptyResp, error) {
	if in == nil || in.UserId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "用户 ID 不正确")
	}

	actor, err := authenticatedUser(l.ctx, l.svcCtx, l.Logger)
	if err != nil {
		return nil, err
	}
	if !isAdministrator(actor.Role) {
		return nil, status.Error(codes.PermissionDenied, "仅管理员可以删除用户")
	}
	if actor.Id == in.UserId {
		return nil, status.Error(codes.PermissionDenied, "不能删除自己的管理员账号")
	}

	err = l.svcCtx.DB.WithContext(l.ctx).Transaction(func(tx *gorm.DB) error {
		var lockedActor model.Users
		// 删除操作不可逆影响较大，因此在事务内重新确认并锁定管理员权限。
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id", "role", "status").
			Where("id = ? AND deleted_at IS NULL", actor.Id).
			Take(&lockedActor).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errDeleteActorChanged
		}
		if err != nil {
			return err
		}
		if !isAdministrator(lockedActor.Role) || pb.UserStatus(lockedActor.Status) != pb.UserStatus_USER_STATUS_NORMAL {
			return errDeleteActorChanged
		}

		var target model.Users
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id", "role").
			Where("id = ? AND deleted_at IS NULL", in.UserId).
			Take(&target).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errDeleteTargetNotFound
		}
		if err != nil {
			return err
		}
		if target.Role >= lockedActor.Role {
			return errDeleteTargetTooHigh
		}

		now := time.Now().UTC()
		tombstoneUsername, tombstoneEmail, err := deletedUserTombstone()
		if err != nil {
			return err
		}
		// 软删除时匿名化登录标识，既减少已删除账号的隐私残留，也释放邮箱和用户名供重新注册。
		if err := tx.Model(&model.Users{}).
			Where("id = ? AND deleted_at IS NULL", target.Id).
			Updates(map[string]any{
				"username":   tombstoneUsername,
				"email":      tombstoneEmail,
				"deleted_at": now,
				"status":     int64(pb.UserStatus_USER_STATUS_DELETED),
			}).Error; err != nil {
			return err
		}

		// 删除账号必须撤销所有设备 Session，使现有 Access Token 和 Refresh Token 立即失效。
		return tx.Model(&model.UserSessions{}).
			Where("user_id = ? AND revoked_at IS NULL", target.Id).
			Updates(map[string]any{
				"revoked_at":     now,
				"revoked_reason": "user_deleted",
			}).Error
	})
	if err != nil {
		switch {
		case errors.Is(err, errDeleteActorChanged):
			return nil, status.Error(codes.PermissionDenied, "管理员权限已发生变化")
		case errors.Is(err, errDeleteTargetNotFound):
			return nil, status.Error(codes.NotFound, "用户不存在")
		case errors.Is(err, errDeleteTargetTooHigh):
			return nil, status.Error(codes.PermissionDenied, "不能删除同级或更高角色的用户")
		default:
			l.Errorf("admin %d delete user %d: %v", actor.Id, in.UserId, err)
			return nil, status.Error(codes.Internal, "删除用户失败")
		}
	}

	// 审计日志只保留操作者和被删除用户 ID。
	l.Infof("admin %d soft-deleted user %d", actor.Id, in.UserId)

	return &pb.EmptyResp{}, nil
}

func deletedUserTombstone() (string, string, error) {
	randomBytes := make([]byte, 12)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", "", fmt.Errorf("generate deleted user tombstone: %w", err)
	}
	suffix := hex.EncodeToString(randomBytes)
	return "deleted_" + suffix, "deleted_" + suffix + "@invalid.local", nil
}
