package authservicelogic

import (
	"context"
	"errors"
	"time"
	"unicode/utf8"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/authctx"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

var errPasswordUpdateConflict = errors.New("password changed concurrently")

type ChangePasswordLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewChangePasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChangePasswordLogic {
	return &ChangePasswordLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ChangePasswordLogic) ChangePassword(in *pb.ChangePasswordReq) (*pb.EmptyResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "请求参数不能为空")
	}

	currentPassword := in.CurrentPassword
	newPassword := in.NewPassword
	if currentPassword == "" {
		return nil, status.Error(codes.InvalidArgument, "当前密码不能为空")
	}
	if len(currentPassword) > 72 {
		return nil, status.Error(codes.InvalidArgument, "当前密码错误")
	}
	if newPassword == "" {
		return nil, status.Error(codes.InvalidArgument, "新密码不能为空")
	}
	if !utf8.ValidString(newPassword) || utf8.RuneCountInString(newPassword) < 6 || len(newPassword) > 72 {
		return nil, status.Error(codes.InvalidArgument, "新密码必须包含至少 6 个字符且不能超过 72 字节")
	}

	claims, ok := authctx.ClaimsFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "用户未登录")
	}

	passwordHash, err := l.svcCtx.UsersModel.GetUserPasswordHash(l.ctx, claims.UserID)
	if err != nil {
		l.Errorf("get user password hash: %v", err)
		return nil, status.Error(codes.Internal, "内部服务器错误")
	}
	if passwordHash == "" {
		return nil, status.Error(codes.Unauthenticated, "用户不存在或登录已失效")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(currentPassword)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return nil, status.Error(codes.InvalidArgument, "当前密码错误")
		}
		l.Errorf("compare current password for user %d: %v", claims.UserID, err)
		return nil, status.Error(codes.Internal, "内部服务器错误")
	}
	if currentPassword == newPassword {
		return nil, status.Error(codes.InvalidArgument, "新密码不能与当前密码相同")
	}

	newPasswordHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		l.Errorf("generate new password hash: %v", err)
		return nil, status.Error(codes.Internal, "内部服务器错误")
	}

	now := time.Now().UTC()
	err = l.svcCtx.DB.WithContext(l.ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.Users{}).
			Where("id = ? AND password_hash = ? AND deleted_at IS NULL", claims.UserID, passwordHash).
			Update("password_hash", string(newPasswordHash))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errPasswordUpdateConflict
		}

		return tx.Model(&model.UserSessions{}).
			Where("user_id = ? AND id <> ? AND revoked_at IS NULL", claims.UserID, claims.SessionID).
			Updates(map[string]any{
				"revoked_at":     now,
				"revoked_reason": "password_changed",
			}).Error
	})
	if err != nil {
		if errors.Is(err, errPasswordUpdateConflict) {
			return nil, status.Error(codes.Aborted, "密码状态已发生变化，请重试")
		}
		l.Errorf("change password for user %d: %v", claims.UserID, err)
		return nil, status.Error(codes.Internal, "内部服务器错误")
	}

	return &pb.EmptyResp{}, nil
}
