package authenticator

import (
	"context"
	"database/sql"
	"errors"
	"time"

	sharedauth "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/token"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

const maxAccessTokenLength = 8192

type accessTokenParser interface {
	ParseAccessToken(raw string) (*token.AccessClaims, error)
}

// Authenticator is Auth RPC's local backend for the project-wide shared gRPC
// interceptor. It is also used by ValidateToken so local and remote callers
// observe exactly the same session, role and account-status checks.
type Authenticator struct {
	parser accessTokenParser
	db     *gorm.DB
}

func New(parser accessTokenParser, db *gorm.DB) *Authenticator {
	return &Authenticator{parser: parser, db: db}
}

type validatedAccessTokenState struct {
	UserID           int64         `gorm:"column:user_id"`
	Role             int64         `gorm:"column:user_role"`
	Status           int64         `gorm:"column:user_status"`
	SchoolID         sql.NullInt64 `gorm:"column:school_id"`
	SessionExpiresAt time.Time     `gorm:"column:session_expires_at"`
	SessionRevokedAt sql.NullTime  `gorm:"column:session_revoked_at"`
}

func (a *Authenticator) Authenticate(ctx context.Context, rawAccessToken string) (sharedauth.Principal, error) {
	if a == nil || a.parser == nil || a.db == nil {
		return sharedauth.Principal{}, status.Error(codes.Unavailable, "鉴权服务暂时不可用")
	}
	if rawAccessToken == "" || len(rawAccessToken) > maxAccessTokenLength {
		return sharedauth.Principal{}, status.Error(codes.Unauthenticated, "访问令牌无效或已过期")
	}

	claims, err := a.parser.ParseAccessToken(rawAccessToken)
	if err != nil {
		return sharedauth.Principal{}, status.Error(codes.Unauthenticated, "访问令牌无效或已过期")
	}

	var state validatedAccessTokenState
	err = a.db.WithContext(ctx).
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
		return sharedauth.Principal{}, status.Error(codes.Unauthenticated, "访问令牌对应的登录会话无效或已过期")
	}
	if err != nil {
		logx.WithContext(ctx).Errorf("validate access token session %d for user %d: %v", claims.SessionID, claims.UserID, err)
		return sharedauth.Principal{}, status.Error(codes.Unavailable, "鉴权服务暂时不可用")
	}
	if state.SessionRevokedAt.Valid || !state.SessionExpiresAt.After(time.Now().UTC()) {
		return sharedauth.Principal{}, status.Error(codes.Unauthenticated, "访问令牌对应的登录会话无效或已过期")
	}
	if pb.UserStatus(state.Status) != pb.UserStatus_USER_STATUS_NORMAL {
		return sharedauth.Principal{}, status.Error(codes.PermissionDenied, "用户账户不可用")
	}
	role := pb.UserRole(state.Role)
	switch role {
	case pb.UserRole_USER_ROLE_USER,
		pb.UserRole_USER_ROLE_ADMIN,
		pb.UserRole_USER_ROLE_SUPER_ADMIN:
	default:
		logx.WithContext(ctx).Errorf("user %d has invalid role %d", state.UserID, state.Role)
		return sharedauth.Principal{}, status.Error(codes.Unavailable, "鉴权服务暂时不可用")
	}

	schoolID := int64(0)
	if state.SchoolID.Valid {
		schoolID = state.SchoolID.Int64
	}
	return sharedauth.Principal{
		UserID:    state.UserID,
		Role:      role,
		Status:    pb.UserStatus(state.Status),
		SchoolID:  schoolID,
		SessionID: claims.SessionID,
	}, nil
}
