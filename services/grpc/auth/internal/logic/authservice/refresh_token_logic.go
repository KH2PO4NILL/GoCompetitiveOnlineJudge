package authservicelogic

import (
	"context"
	"errors"
	sharedauth "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"
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
		return nil, sharedauth.AuthenticationError(codes.Unauthenticated, "refresh_token_invalid", "刷新令牌无效或已过期")
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
	var previousRefreshHash string
	var browserToken string
	if in.BrowserSession {
		browserToken = rawRefreshToken
	}
	err = l.svcCtx.DB.WithContext(l.ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		sessions := model.NewUserSessionsModel(tx)
		lookup := sessions.LockRefresh
		if in.BrowserSession {
			lookup = sessions.LockBrowserSession
		}
		session, user, err := lookup(l.ctx, currentRefreshTokenHash, now)
		if errors.Is(err, model.ErrNotFound) {
			return errRefreshTokenInvalid
		}
		if errors.Is(err, model.ErrConflict) {
			return errRefreshAccountNotUsable
		}
		if err != nil {
			return err
		}

		schoolID := int64(0)
		if user.SchoolId.Valid {
			schoolID = user.SchoolId.Int64
		}

		// Refresh Token 必须单次使用：成功后原哈希会被新哈希替换，旧 Token 随即失效。
		if err := sessions.Rotate(l.ctx, session.Id, session.RefreshTokenHash, newRefreshTokenHash, now, now.Add(refreshTTL)); err != nil {
			if errors.Is(err, model.ErrConflict) {
				return errRefreshTokenInvalid
			}
			return err
		}

		rotatedSessionID = session.Id
		previousRefreshHash = session.RefreshTokenHash

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
			if !in.BrowserSession {
				l.revokeReplayedRefreshToken(currentRefreshTokenHash)
			}
			return nil, sharedauth.AuthenticationError(codes.Unauthenticated, "refresh_token_invalid", "刷新令牌无效或已过期")
		case errors.Is(err, errRefreshAccountNotUsable):
			return nil, sharedauth.AuthenticationError(codes.PermissionDenied, "account_unavailable", "用户账户不可用")
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
		rediskey.UsedRefreshToken(previousRefreshHash),
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
			AccessToken:         accessToken,
			RefreshToken:        newRefreshToken,
			BrowserSessionToken: browserToken,
			ExpiresIn:           expiresIn,
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

	affected, revokeErr := model.NewUserSessionsModel(l.svcCtx.DB).RevokeByID(l.ctx, sessionID, 0, "refresh_token_reuse", time.Now().UTC())
	if revokeErr != nil {
		l.Errorf("revoke replayed refresh token session %d: %v", sessionID, revokeErr)
		return
	}
	if affected > 0 {
		l.Errorf("detected refresh token replay and revoked session %d", sessionID)
	}
}
