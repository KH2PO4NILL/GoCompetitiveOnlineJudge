package userservicelogic

import (
	"context"
	"errors"
	"strings"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

var errChangeEmailConflict = errors.New("email changed concurrently")

type ChangeEmailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewChangeEmailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChangeEmailLogic {
	return &ChangeEmailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ChangeEmailLogic) ChangeEmail(in *pb.ChangeEmailReq) (*pb.ChangeEmailResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "请求参数不能为空")
	}

	actor, err := authenticatedUser(l.ctx, l.svcCtx, l.Logger)
	if err != nil {
		return nil, err
	}
	newEmail, ok := normalizeUserEmail(in.NewEmail)
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "新邮箱格式不正确")
	}
	if newEmail == actor.Email {
		return nil, status.Error(codes.InvalidArgument, "新邮箱不能与当前邮箱相同")
	}
	code := strings.TrimSpace(in.Code)
	if !isSixDigitEmailCode(code) {
		return nil, status.Error(codes.InvalidArgument, "验证码必须是 6 位数字")
	}

	var existingEmailCount int64
	err = l.svcCtx.DB.WithContext(l.ctx).
		Model(&model.Users{}).
		Where("email = ? AND deleted_at IS NULL", newEmail).
		Count(&existingEmailCount).Error
	if err != nil {
		l.Errorf("check new email before update for user %d: %v", actor.Id, err)
		return nil, status.Error(codes.Internal, "修改邮箱失败")
	}
	if existingEmailCount > 0 {
		return nil, status.Error(codes.AlreadyExists, "该邮箱已被使用")
	}

	// 必须消费发送到新邮箱且用途为 change_email 的验证码，成功后验证码立即失效。
	matched, err := consumeChangeEmailCode(l.ctx, l.svcCtx.Redis, newEmail, code)
	if err != nil {
		l.Errorf("verify change email code for user %d: %v", actor.Id, err)
		return nil, status.Error(codes.Unavailable, "验证码服务暂时不可用")
	}
	if !matched {
		return nil, status.Error(codes.Unauthenticated, "验证码错误或已过期")
	}

	var updatedUser model.Users
	err = l.svcCtx.DB.WithContext(l.ctx).Transaction(func(tx *gorm.DB) error {
		// 带上旧邮箱作为并发条件，避免覆盖另一个请求刚修改的新邮箱。
		result := tx.Model(&model.Users{}).
			Where("id = ? AND email = ? AND deleted_at IS NULL", actor.Id, actor.Email).
			Update("email", newEmail)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errChangeEmailConflict
		}

		return tx.Select("id", "username", "email", "role", "status", "school_id").
			Where("id = ? AND deleted_at IS NULL", actor.Id).
			Take(&updatedUser).Error
	})
	if err != nil {
		switch {
		case duplicateKey(err, "uk_users_email"):
			return nil, status.Error(codes.AlreadyExists, "该邮箱已被使用")
		case errors.Is(err, errChangeEmailConflict), errors.Is(err, gorm.ErrRecordNotFound):
			return nil, status.Error(codes.Aborted, "邮箱状态已发生变化，请重新获取验证码")
		default:
			l.Errorf("change email for user %d: %v", actor.Id, err)
			return nil, status.Error(codes.Internal, "修改邮箱失败")
		}
	}

	// 审计日志只记录用户 ID，不能记录邮箱地址或验证码。
	l.Infof("changed email for user %d", actor.Id)

	return &pb.ChangeEmailResp{User: userInfo(&updatedUser)}, nil
}
