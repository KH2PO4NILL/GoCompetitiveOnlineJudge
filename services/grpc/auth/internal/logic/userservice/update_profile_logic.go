package userservicelogic

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

var errProfileSchoolUnavailable = errors.New("profile school is unavailable")

type UpdateProfileLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateProfileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateProfileLogic {
	return &UpdateProfileLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateProfileLogic) UpdateProfile(in *pb.UpdateProfileReq) (*pb.UpdateProfileResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "请求参数不能为空")
	}
	if in.Username == nil && in.SchoolId == nil {
		return nil, status.Error(codes.InvalidArgument, "至少需要修改一个字段")
	}

	actor, err := authenticatedUser(l.ctx, l.svcCtx, l.Logger)
	if err != nil {
		return nil, err
	}

	updates := make(map[string]any, 2)
	schoolChanged := false
	if in.Username != nil {
		username := strings.TrimSpace(*in.Username)
		if username == "" || !utf8.ValidString(username) || utf8.RuneCountInString(username) > 32 {
			return nil, status.Error(codes.InvalidArgument, "用户名必须包含 1 到 32 个字符")
		}
		updates["username"] = username
	}
	if in.SchoolId != nil {
		if *in.SchoolId < 0 {
			return nil, status.Error(codes.InvalidArgument, "学校 ID 不正确")
		}
		if *in.SchoolId == 0 {
			// optional school_id = 0 明确表示清空学校，而 nil 表示不修改该字段。
			updates["school_id"] = nil
		} else {
			updates["school_id"] = *in.SchoolId
		}
		schoolChanged = (actor.SchoolId.Valid && actor.SchoolId.Int64 != *in.SchoolId) ||
			(!actor.SchoolId.Valid && *in.SchoolId != 0)
	}

	var updatedUser model.Users
	err = l.svcCtx.DB.WithContext(l.ctx).Transaction(func(tx *gorm.DB) error {
		if in.SchoolId != nil && *in.SchoolId > 0 {
			var school model.Schools
			err := tx.Select("id").
				Where("id = ? AND status = 1", *in.SchoolId).
				Take(&school).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errProfileSchoolUnavailable
			}
			if err != nil {
				return err
			}
		}

		// 只更新用户名和学校两个白名单字段，客户端不能借此修改角色或账号状态。
		if err := tx.Model(&model.Users{}).
			Where("id = ? AND deleted_at IS NULL", actor.Id).
			Updates(updates).Error; err != nil {
			return err
		}

		if schoolChanged {
			// school_id 属于 Access Token 的授权声明。资料修改后撤销全部 Session，
			// 避免旧 Token 在过期前继续携带原学校权限；客户端需要重新登录。
			now := time.Now().UTC()
			if err := tx.Model(&model.UserSessions{}).
				Where("user_id = ? AND revoked_at IS NULL", actor.Id).
				Updates(map[string]any{
					"revoked_at":     now,
					"revoked_reason": "school_changed",
				}).Error; err != nil {
				return err
			}
		}

		return tx.Select("id", "username", "email", "role", "status", "school_id").
			Where("id = ? AND deleted_at IS NULL", actor.Id).
			Take(&updatedUser).Error
	})
	if err != nil {
		switch {
		case errors.Is(err, errProfileSchoolUnavailable):
			return nil, status.Error(codes.InvalidArgument, "学校不存在或不可用")
		case duplicateKey(err, "uk_users_username"):
			return nil, status.Error(codes.AlreadyExists, "用户名已存在")
		case errors.Is(err, gorm.ErrRecordNotFound):
			return nil, status.Error(codes.Aborted, "用户状态已发生变化，请重试")
		default:
			l.Errorf("update profile for user %d: %v", actor.Id, err)
			return nil, status.Error(codes.Internal, "修改用户资料失败")
		}
	}

	l.Infof("updated profile for user %d", actor.Id)

	return &pb.UpdateProfileResp{User: userInfo(&updatedUser)}, nil
}
