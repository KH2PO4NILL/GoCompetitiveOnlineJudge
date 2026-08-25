package userservicelogic

import (
	"context"
	"errors"
	"net/mail"
	"strings"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/authctx"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/rediskey"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

const consumeChangeEmailCodeScript = `
local saved = redis.call("GET", KEYS[1])
if not saved then
    return 0
end

local attempts = redis.call("INCR", KEYS[2])
if attempts == 1 then
    local ttl = redis.call("PTTL", KEYS[1])
    if ttl > 0 then
        redis.call("PEXPIRE", KEYS[2], ttl)
    else
        redis.call("EXPIRE", KEYS[2], 300)
    end
end

if attempts > 5 then
    redis.call("DEL", KEYS[1])
    redis.call("DEL", KEYS[2])
    return -1
end

if saved ~= ARGV[1] then
    return 0
end

redis.call("DEL", KEYS[1])
redis.call("DEL", KEYS[2])
return 1
`

func authenticatedUser(ctx context.Context, svcCtx *svc.ServiceContext, logger logx.Logger) (*model.Users, error) {
	claims, ok := authctx.ClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "用户未登录")
	}

	user, err := findActiveUserByID(ctx, svcCtx.DB, claims.UserID)
	if err != nil {
		logger.Errorf("find authenticated user %d: %v", claims.UserID, err)
		return nil, status.Error(codes.Internal, "读取用户信息失败")
	}
	if user == nil {
		return nil, status.Error(codes.Unauthenticated, "用户不存在或登录已失效")
	}
	// 权限判断必须使用数据库中的最新状态，不能只相信 Access Token 内可能过时的角色和状态。
	if pb.UserStatus(user.Status) != pb.UserStatus_USER_STATUS_NORMAL {
		return nil, status.Error(codes.PermissionDenied, "用户账户不可用")
	}

	return user, nil
}

func findActiveUserByID(ctx context.Context, db *gorm.DB, userID int64) (*model.Users, error) {
	var user model.Users
	err := db.WithContext(ctx).
		// 用户服务查询使用字段白名单，避免把密码哈希加载到业务逻辑内存中。
		Select("id", "username", "email", "role", "status", "school_id").
		Where("id = ? AND deleted_at IS NULL", userID).
		Take(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func userInfo(user *model.Users) *pb.UserInfo {
	if user == nil {
		return nil
	}

	schoolID := int64(0)
	if user.SchoolId.Valid {
		schoolID = user.SchoolId.Int64
	}
	return &pb.UserInfo{
		UserId:   user.Id,
		Username: user.Username,
		Email:    user.Email,
		Role:     pb.UserRole(user.Role),
		Status:   pb.UserStatus(user.Status),
		SchoolId: schoolID,
	}
}

func isAdministrator(role int64) bool {
	return pb.UserRole(role) == pb.UserRole_USER_ROLE_ADMIN ||
		pb.UserRole(role) == pb.UserRole_USER_ROLE_SUPER_ADMIN
}

func isAssignableRole(role pb.UserRole) bool {
	switch role {
	case pb.UserRole_USER_ROLE_USER,
		pb.UserRole_USER_ROLE_ADMIN,
		pb.UserRole_USER_ROLE_SUPER_ADMIN:
		return true
	default:
		return false
	}
}

func isManageableStatus(userStatus pb.UserStatus) bool {
	switch userStatus {
	case pb.UserStatus_USER_STATUS_NORMAL,
		pb.UserStatus_USER_STATUS_DISABLED,
		pb.UserStatus_USER_STATUS_BANNED:
		return true
	default:
		return false
	}
}

func normalizeUserEmail(value string) (string, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || len(value) > 254 {
		return "", false
	}
	address, err := mail.ParseAddress(value)
	if err != nil || address.Address != value {
		return "", false
	}
	return value, true
}

func isSixDigitEmailCode(value string) bool {
	if len(value) != 6 {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}

func consumeChangeEmailCode(ctx context.Context, client *redis.Client, email, code string) (bool, error) {
	codeKey := rediskey.EmailCode("change_email", email)
	attemptsKey := rediskey.EmailCodeAttempts("change_email", email)
	matched, err := client.Eval(
		ctx,
		consumeChangeEmailCodeScript,
		[]string{codeKey, attemptsKey},
		code,
	).Int()
	return matched == 1, err
}

func duplicateKey(err error, keyName string) bool {
	var mysqlErr *mysqlDriver.MySQLError
	return errors.As(err, &mysqlErr) &&
		mysqlErr.Number == 1062 &&
		strings.Contains(mysqlErr.Message, keyName)
}
