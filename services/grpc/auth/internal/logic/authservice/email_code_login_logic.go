package authservicelogic

import (
	"context"
	"strings"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/token"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type EmailCodeLoginLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewEmailCodeLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EmailCodeLoginLogic {
	return &EmailCodeLoginLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *EmailCodeLoginLogic) EmailCodeLogin(in *pb.EmailCodeLoginReq) (*pb.LoginResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	email, ok := normalizeEmail(in.Email)
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "invalid email address")
	}

	code := strings.TrimSpace(in.Code)
	if !isSixDigitCode(code) {
		return nil, status.Error(codes.InvalidArgument, "verification code must contain 6 digits")
	}

	user, err := l.svcCtx.UsersModel.FindOneByEmail(l.ctx, email)
	if err != nil {
		l.Errorf("find user by email: %v", err)
		return nil, status.Error(codes.Internal, "login failed")
	}

	matched, err := consumeEmailCode(l.ctx, l.svcCtx.Redis, emailCodePurposeLogin, email, code)
	if err != nil {
		l.Errorf("verify email login code: %v", err)
		return nil, status.Error(codes.Unavailable, "验证服务不可用")
	}
	if !matched || user == nil {
		return nil, status.Error(codes.Unauthenticated, "验证码错误或用户不存在")
	}

	if pb.UserStatus(user.Status) != pb.UserStatus_USER_STATUS_NORMAL {
		return nil, status.Error(codes.PermissionDenied, "用户状态异常")
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

	clientInfo := loginClientInfo(l.ctx)
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
		l.Errorf("create email login session for user %d: %v", user.Id, err)
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
