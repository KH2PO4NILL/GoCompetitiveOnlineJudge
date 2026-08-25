package authservicelogic

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/token"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

const dummyPasswordHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

type UsernameLoginLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUsernameLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UsernameLoginLogic {
	return &UsernameLoginLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UsernameLoginLogic) UsernameLogin(in *pb.UsernameLoginReq) (*pb.LoginResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "请求无效")
	}

	username := strings.TrimSpace(in.Username)
	if username == "" || !utf8.ValidString(username) || utf8.RuneCountInString(username) > 32 {
		return nil, status.Error(codes.InvalidArgument, "用户名必须为 1 到 32 个字符")
	}
	if in.Password == "" || len(in.Password) > 72 {
		return nil, status.Error(codes.InvalidArgument, "密码必须为 1 到 72 个字节")
	}

	clientInfo := loginClientInfo(l.ctx)
	rateLimitBuckets := passwordLoginRateLimitBuckets("username", strings.ToLower(username), clientInfo)
	limited, err := isPasswordLoginRateLimited(l.ctx, l.svcCtx.Redis, rateLimitBuckets)
	if err != nil {
		// Redis 短暂故障时仍执行密码验证，避免限流组件故障导致所有用户无法登录。
		l.Errorf("check username login rate limit: %v", err)
	} else if limited {
		return nil, status.Error(codes.ResourceExhausted, "登录尝试过于频繁，请稍后再试")
	}

	user, err := l.svcCtx.UsersModel.FindOneByUsername(l.ctx, username)
	if err != nil {
		l.Errorf("find user by username: %v", err)
		return nil, status.Error(codes.Internal, "login failed")
	}
	if user == nil {
		// Perform one bcrypt comparison even for an unknown username to reduce
		// the timing difference between unknown users and wrong passwords.
		_ = bcrypt.CompareHashAndPassword([]byte(dummyPasswordHash), []byte(in.Password))
		return nil, l.passwordLoginFailure(rateLimitBuckets)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(in.Password)); err != nil {
		if !errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			l.Errorf("compare password hash for user %d: %v", user.Id, err)
			return nil, status.Error(codes.Internal, "login failed")
		}
		return nil, l.passwordLoginFailure(rateLimitBuckets)
	}
	if err := clearPasswordLoginFailures(l.ctx, l.svcCtx.Redis, rateLimitBuckets); err != nil {
		l.Errorf("clear username login failures for user %d: %v", user.Id, err)
	}

	if pb.UserStatus(user.Status) != pb.UserStatus_USER_STATUS_NORMAL {
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
		return nil, status.Error(codes.Internal, "login failed")
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

func (l *UsernameLoginLogic) passwordLoginFailure(rateLimitBuckets []passwordLoginRateLimitBucket) error {
	limited, err := recordPasswordLoginFailure(l.ctx, l.svcCtx.Redis, rateLimitBuckets)
	if err != nil {
		l.Errorf("record username login failure: %v", err)
		return status.Error(codes.Unauthenticated, "用户名或密码错误")
	}
	if limited {
		return status.Error(codes.ResourceExhausted, "登录尝试过于频繁，请稍后再试")
	}
	return status.Error(codes.Unauthenticated, "用户名或密码错误")
}

type clientInfo struct {
	deviceID   sql.NullString
	deviceName sql.NullString
	ipAddress  sql.NullString
	userAgent  sql.NullString
}

func loginClientInfo(ctx context.Context) clientInfo {
	info := clientInfo{}
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		info.deviceID = nullableLoginValue(firstMetadataValue(md, "x-device-id"), 128)
		info.deviceName = nullableLoginValue(firstMetadataValue(md, "x-device-name"), 128)
		info.userAgent = nullableLoginValue(firstMetadataValue(md, "user-agent"), 512)
	}

	if remotePeer, ok := peer.FromContext(ctx); ok && remotePeer.Addr != nil {
		address := remotePeer.Addr.String()
		if host, _, err := net.SplitHostPort(address); err == nil {
			address = host
		}
		info.ipAddress = nullableLoginValue(address, 45)
	}

	return info
}

func firstMetadataValue(md metadata.MD, key string) string {
	values := md.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func nullableLoginValue(value string, maximumLength int) sql.NullString {
	value = strings.TrimSpace(value)
	if value == "" {
		return sql.NullString{}
	}
	runes := []rune(value)
	if len(runes) > maximumLength {
		value = string(runes[:maximumLength])
	}

	return sql.NullString{String: value, Valid: true}
}
