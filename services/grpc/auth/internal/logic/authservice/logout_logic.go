package authservicelogic

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/authctx"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/rediskey"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/token"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type LogoutLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLogoutLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LogoutLogic {
	return &LogoutLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *LogoutLogic) Logout(in *pb.LogoutReq) (*pb.EmptyResp, error) {
	claims, ok := authctx.ClaimsFromContext(l.ctx)
	if ok {
		if err := l.revokeLogoutSession(claims.SessionID, claims.UserID); err != nil {
			return nil, err
		}
		return &pb.EmptyResp{}, nil
	}
	if in == nil {
		return nil, status.Error(codes.Unauthenticated, "用户未登录")
	}

	rawRefreshToken := strings.TrimSpace(in.RefreshToken)
	if rawRefreshToken == "" || len(rawRefreshToken) > 512 {
		return nil, status.Error(codes.Unauthenticated, "退出令牌无效")
	}
	refreshHash := token.HashRefreshToken(rawRefreshToken)

	now := time.Now().UTC()
	result := l.svcCtx.DB.WithContext(l.ctx).
		Model(&model.UserSessions{}).
		Where("refresh_token_hash = ? AND revoked_at IS NULL", refreshHash).
		Updates(map[string]any{
			"revoked_at":     now,
			"revoked_reason": "logout",
		})
	if result.Error != nil {
		l.Errorf("revoke session by refresh token during logout: %v", result.Error)
		return nil, status.Error(codes.Internal, "退出登录失败")
	}
	if result.RowsAffected == 0 {
		// Refresh Token 可能已经轮换。若它属于当前 Token family，则仍撤销对应 Session。
		usedSessionID, err := l.svcCtx.Redis.Get(l.ctx, rediskey.UsedRefreshToken(refreshHash)).Result()
		if err == nil {
			sessionID, parseErr := strconv.ParseInt(usedSessionID, 10, 64)
			if parseErr == nil && sessionID > 0 {
				if err := l.revokeLogoutSession(sessionID, 0); err != nil {
					return nil, err
				}
			}
		} else if !errors.Is(err, redis.Nil) {
			l.Errorf("read rotated refresh token during logout: %v", err)
		}
	}

	return &pb.EmptyResp{}, nil
}

func (l *LogoutLogic) revokeLogoutSession(sessionID, userID int64) error {
	query := l.svcCtx.DB.WithContext(l.ctx).
		Model(&model.UserSessions{}).
		Where("id = ? AND revoked_at IS NULL", sessionID)
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	result := query.Updates(map[string]any{
		"revoked_at":     time.Now().UTC(),
		"revoked_reason": "logout",
	})
	if result.Error != nil {
		l.Errorf("revoke session %d during logout: %v", sessionID, result.Error)
		return status.Error(codes.Internal, "退出登录失败")
	}
	return nil
}
