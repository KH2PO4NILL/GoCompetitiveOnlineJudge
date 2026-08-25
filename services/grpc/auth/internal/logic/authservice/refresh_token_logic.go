package authservicelogic

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/rediskey"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/token"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	errRefreshTokenInvalid     = errors.New("refresh token is invalid or expired")
	errRefreshAccountNotUsable = errors.New("refresh token account is not usable")
)

type RefreshTokenLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRefreshTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefreshTokenLogic {
	return &RefreshTokenLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *RefreshTokenLogic) RefreshToken(in *pb.RefreshTokenReq) (*pb.RefreshTokenResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "请求参数不能为空")
	}

	rawRefreshToken := in.RefreshToken
	// Refresh Token 是随机生成的短字符串，限制输入长度可以避免恶意超长请求进入数据库查询。
	if rawRefreshToken == "" || len(rawRefreshToken) > 512 {
		return nil, status.Error(codes.Unauthenticated, "刷新令牌无效或已过期")
	}

	refreshTTL := time.Duration(l.svcCtx.Config.Token.RefreshExpireSeconds) * time.Second
	if refreshTTL <= 0 {
		l.Error("refresh token expiration is not configured")
		return nil, status.Error(codes.Internal, "刷新令牌失败")
	}

	// 数据库中只查询和保存 Refresh Token 的 SHA-256 哈希，绝不能保存或记录明文 Token。
	currentRefreshTokenHash := token.HashRefreshToken(rawRefreshToken)
	newRefreshToken, newRefreshTokenHash, err := token.GenerateRefreshToken()
	if err != nil {
		l.Errorf("generate rotated refresh token: %v", err)
		return nil, status.Error(codes.Internal, "刷新令牌失败")
	}

	var accessToken string
	var accessExpiresAt time.Time
	var rotatedSessionID int64
	err = l.svcCtx.DB.WithContext(l.ctx).Transaction(func(tx *gorm.DB) error {
		var session model.UserSessions
		// 对 Session 加行锁，确保两个并发请求不能同时使用同一个旧 Refresh Token。
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("refresh_token_hash = ?", currentRefreshTokenHash).
			Take(&session).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errRefreshTokenInvalid
		}
		if err != nil {
			return err
		}

		now := time.Now().UTC()
		// 被撤销或已经过期的 Session 不能再签发任何新 Token。
		if session.RevokedAt.Valid || !session.ExpiresAt.After(now) {
			return errRefreshTokenInvalid
		}
		if !token.VerifyRefreshToken(rawRefreshToken, session.RefreshTokenHash) {
			return errRefreshTokenInvalid
		}

		var user model.Users
		err = tx.Select("id", "role", "status", "school_id").
			Where("id = ? AND deleted_at IS NULL", session.UserId).
			Take(&user).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errRefreshAccountNotUsable
		}
		if err != nil {
			return err
		}
		// 每次刷新都读取用户的最新权限和状态，避免继续使用登录时的旧角色信息。
		if pb.UserStatus(user.Status) != pb.UserStatus_USER_STATUS_NORMAL {
			return errRefreshAccountNotUsable
		}

		schoolID := int64(0)
		if user.SchoolId.Valid {
			schoolID = user.SchoolId.Int64
		}

		// Refresh Token 必须单次使用：成功后原哈希会被新哈希替换，旧 Token 随即失效。
		result := tx.Model(&model.UserSessions{}).
			Where(
				"id = ? AND refresh_token_hash = ? AND revoked_at IS NULL",
				session.Id,
				currentRefreshTokenHash,
			).
			Updates(map[string]any{
				"refresh_token_hash": newRefreshTokenHash,
				"last_used_at":       now,
				"expires_at":         now.Add(refreshTTL),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errRefreshTokenInvalid
		}
		rotatedSessionID = session.Id

		var tokenErr error
		accessToken, accessExpiresAt, tokenErr = l.svcCtx.Token.GenerateAccessToken(token.AccessTokenInput{
			UserID:    user.Id,
			Role:      int32(user.Role),
			Status:    int32(user.Status),
			SchoolID:  schoolID,
			SessionID: session.Id,
		})
		return tokenErr
	})
	if err != nil {
		switch {
		case errors.Is(err, errRefreshTokenInvalid):
			l.revokeReplayedRefreshToken(currentRefreshTokenHash)
			return nil, status.Error(codes.Unauthenticated, "刷新令牌无效或已过期")
		case errors.Is(err, errRefreshAccountNotUsable):
			return nil, status.Error(codes.PermissionDenied, "用户账户不可用")
		default:
			l.Errorf("rotate refresh token: %v", err)
			return nil, status.Error(codes.Internal, "刷新令牌失败")
		}
	}

	// 保存旧 Token 与 Session 的短期映射。旧 Token 再次出现时可识别为重放，
	// 并撤销该设备后续已经轮换出的整个 Token family。
	historyCtx, cancel := context.WithTimeout(context.WithoutCancel(l.ctx), 2*time.Second)
	defer cancel()
	if err := l.svcCtx.Redis.Set(
		historyCtx,
		rediskey.UsedRefreshToken(currentRefreshTokenHash),
		strconv.FormatInt(rotatedSessionID, 10),
		refreshTTL,
	).Err(); err != nil {
		// 数据库轮换已经提交，此处不能再返回失败，否则客户端拿不到新 Token 且旧 Token 已失效。
		l.Errorf("record rotated refresh token for session %d: %v", rotatedSessionID, err)
	}

	expiresIn := int64(time.Until(accessExpiresAt).Seconds())
	if expiresIn < 1 {
		expiresIn = 1
	}

	return &pb.RefreshTokenResp{
		Tokens: &pb.TokenPair{
			AccessToken:  accessToken,
			RefreshToken: newRefreshToken,
			ExpiresIn:    expiresIn,
		},
	}, nil
}

func (l *RefreshTokenLogic) revokeReplayedRefreshToken(refreshTokenHash string) {
	sessionValue, err := l.svcCtx.Redis.Get(
		l.ctx,
		rediskey.UsedRefreshToken(refreshTokenHash),
	).Result()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			l.Errorf("read rotated refresh token history: %v", err)
		}
		return
	}
	sessionID, err := strconv.ParseInt(sessionValue, 10, 64)
	if err != nil || sessionID <= 0 {
		return
	}

	result := l.svcCtx.DB.WithContext(l.ctx).
		Model(&model.UserSessions{}).
		Where("id = ? AND revoked_at IS NULL", sessionID).
		Updates(map[string]any{
			"revoked_at":     time.Now().UTC(),
			"revoked_reason": "refresh_token_reuse",
		})
	if result.Error != nil {
		l.Errorf("revoke replayed refresh token session %d: %v", sessionID, result.Error)
		return
	}
	if result.RowsAffected > 0 {
		l.Errorf("detected refresh token replay and revoked session %d", sessionID)
	}
}
