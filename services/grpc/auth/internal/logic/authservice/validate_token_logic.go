package authservicelogic

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type validatedAccessTokenState struct {
	UserID           int64         `gorm:"column:user_id"`
	Role             int64         `gorm:"column:user_role"`
	Status           int64         `gorm:"column:user_status"`
	SchoolID         sql.NullInt64 `gorm:"column:school_id"`
	SessionExpiresAt time.Time     `gorm:"column:session_expires_at"`
	SessionRevokedAt sql.NullTime  `gorm:"column:session_revoked_at"`
}

type ValidateTokenLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewValidateTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ValidateTokenLogic {
	return &ValidateTokenLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ValidateTokenLogic) ValidateToken(in *pb.ValidateTokenReq) (*pb.ValidateTokenResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "请求参数不能为空")
	}

	rawAccessToken := in.AccessToken
	// 限制输入长度可以避免恶意超长字符串进入 JWT 解析流程，正常 Access Token 远小于该上限。
	if rawAccessToken == "" || len(rawAccessToken) > 8192 {
		return nil, status.Error(codes.Unauthenticated, "访问令牌无效或已过期")
	}

	// ParseAccessToken 会验证签名算法、签名、签发方、受众、有效期以及自定义声明。
	claims, err := l.svcCtx.Token.ParseAccessToken(rawAccessToken)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "访问令牌无效或已过期")
	}

	var state validatedAccessTokenState
	err = l.svcCtx.DB.WithContext(l.ctx).
		Model(&model.UserSessions{}).
		Select(`
			user_sessions.user_id AS user_id,
			user_sessions.expires_at AS session_expires_at,
			user_sessions.revoked_at AS session_revoked_at,
			users.role AS user_role,
			users.status AS user_status,
			users.school_id AS school_id
		`).
		Joins("JOIN users ON users.id = user_sessions.user_id AND users.deleted_at IS NULL").
		Where("user_sessions.id = ? AND user_sessions.user_id = ?", claims.SessionID, claims.UserID).
		Take(&state).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, status.Error(codes.Unauthenticated, "访问令牌对应的登录会话无效或已过期")
	}
	if err != nil {
		l.Errorf("validate access token session %d for user %d: %v", claims.SessionID, claims.UserID, err)
		return nil, status.Error(codes.Unavailable, "鉴权服务暂时不可用")
	}

	// JWT 签名有效并不代表会话仍然有效；退出登录或强制下线后必须通过 Session 状态立即拒绝旧 Token。
	if state.SessionRevokedAt.Valid || !state.SessionExpiresAt.After(time.Now().UTC()) {
		return nil, status.Error(codes.Unauthenticated, "访问令牌对应的登录会话无效或已过期")
	}
	if pb.UserStatus(state.Status) != pb.UserStatus_USER_STATUS_NORMAL {
		return nil, status.Error(codes.PermissionDenied, "用户账户不可用")
	}

	schoolID := int64(0)
	if state.SchoolID.Valid {
		schoolID = state.SchoolID.Int64
	}

	// 返回数据库中的最新权限，不直接信任 Token 内可能已经过时的角色、状态和学校信息。
	return &pb.ValidateTokenResp{
		Claims: &pb.AuthClaims{
			UserId:   state.UserID,
			Role:     pb.UserRole(state.Role),
			Status:   pb.UserStatus(state.Status),
			SchoolId: schoolID,
		},
	}, nil
}
