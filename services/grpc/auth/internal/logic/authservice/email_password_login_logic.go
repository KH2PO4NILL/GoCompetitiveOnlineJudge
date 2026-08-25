package authservicelogic

import (
	"context"
	"errors"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/token"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type EmailPasswordLoginLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewEmailPasswordLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EmailPasswordLoginLogic {
	return &EmailPasswordLoginLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *EmailPasswordLoginLogic) EmailPasswordLogin(in *pb.EmailPasswordLoginReq) (*pb.LoginResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "请求不能为空")
	}
	email, ok := normalizeEmail(in.Email)
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "邮箱格式无效")
	}
	if in.Password == "" || len(in.Password) > 72 {
		return nil, status.Error(codes.InvalidArgument, "密码必须为 1 到 72 个字节")
	}

	clientInfo := loginClientInfo(l.ctx)
	rateLimitBuckets := passwordLoginRateLimitBuckets("email", email, clientInfo)
	limited, err := isPasswordLoginRateLimited(l.ctx, l.svcCtx.Redis, rateLimitBuckets)
	if err != nil {
		// Fail open so a temporary Redis failure does not lock out every user.
		l.Errorf("check password login rate limit: %v", err)
	} else if limited {
		return nil, status.Error(codes.ResourceExhausted, "登录尝试过于频繁，请稍后再试")
	}

	user, err := l.svcCtx.UsersModel.FindOneByEmail(l.ctx, email)
	if err != nil {
		l.Errorf("find user by email: %v", err)
		return nil, status.Error(codes.Internal, "登录失败")
	}
	if user == nil {
		// Keep the response time close to a real password comparison so callers
		// cannot determine whether an email address has been registered.
		_ = bcrypt.CompareHashAndPassword([]byte(dummyPasswordHash), []byte(in.Password))
		return nil, l.passwordLoginFailure(rateLimitBuckets)
	}

	// Verify the plaintext password against the stored bcrypt hash.
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(in.Password)); err != nil {
		if !errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			l.Errorf("compare password hash for user %d: %v", user.Id, err)
			return nil, status.Error(codes.Internal, "登录失败")
		}
		return nil, l.passwordLoginFailure(rateLimitBuckets)
	}
	if err := clearPasswordLoginFailures(l.ctx, l.svcCtx.Redis, rateLimitBuckets); err != nil {
		l.Errorf("clear password login failures for user %d: %v", user.Id, err)
	}

	if user.Status != int64(pb.UserStatus_USER_STATUS_NORMAL) {
		return nil, status.Error(codes.PermissionDenied, "用户账户不可用")
	}

	refreshTTL := time.Duration(l.svcCtx.Config.Token.RefreshExpireSeconds) * time.Second
	if refreshTTL <= 0 {
		l.Error("refresh token expiration is not configured")
		return nil, status.Error(codes.Internal, "登录失败")
	}

	refreshToken, refreshTokenHash, err := token.GenerateRefreshToken()
	if err != nil {
		l.Errorf("generate refresh token: %v", err)
		return nil, status.Error(codes.Internal, "登录失败")
	}

	schoolID := int64(0)
	if user.SchoolId.Valid {
		schoolID = user.SchoolId.Int64
	}
	session := &model.UserSessions{
		UserId:           user.Id,
		RefreshTokenHash: refreshTokenHash,
		DeviceId:         clientInfo.deviceID,
		DeviceName:       clientInfo.deviceName,
		IpAddress:        clientInfo.ipAddress,
		UserAgent:        clientInfo.userAgent,
		ExpiresAt:        time.Now().UTC().Add(refreshTTL),
	}

	var accessToken string
	var accessExpiresAt time.Time
	err = l.svcCtx.DB.WithContext(l.ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(session).Error; err != nil {
			return err
		}

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
		l.Errorf("create login session for user %d: %v", user.Id, err)
		return nil, status.Error(codes.Internal, "登录失败")
	}

	expiresIn := int64(time.Until(accessExpiresAt).Seconds())
	if expiresIn < 1 {
		expiresIn = 1
	}

	return &pb.LoginResp{
		Tokens: &pb.TokenPair{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			ExpiresIn:    expiresIn,
		},
		User: &pb.UserInfo{
			UserId:   user.Id,
			Username: user.Username,
			Email:    user.Email,
			Role:     pb.UserRole(user.Role),
			Status:   pb.UserStatus(user.Status),
			SchoolId: schoolID,
		},
	}, nil
}

func (l *EmailPasswordLoginLogic) passwordLoginFailure(rateLimitBuckets []passwordLoginRateLimitBucket) error {
	limited, err := recordPasswordLoginFailure(l.ctx, l.svcCtx.Redis, rateLimitBuckets)
	if err != nil {
		// Authentication still fails even if rate-limit accounting is unavailable.
		l.Errorf("record password login failure: %v", err)
		return status.Error(codes.Unauthenticated, "邮箱或密码错误")
	}
	if limited {
		return status.Error(codes.ResourceExhausted, "登录尝试过于频繁，请稍后再试")
	}
	return status.Error(codes.Unauthenticated, "邮箱或密码错误")
}
