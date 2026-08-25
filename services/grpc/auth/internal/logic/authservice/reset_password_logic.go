package authservicelogic

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
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

var errResetPasswordConflict = errors.New("password changed concurrently")

type ResetPasswordLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResetPasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResetPasswordLogic {
	return &ResetPasswordLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ResetPasswordLogic) ResetPassword(in *pb.ResetPasswordReq) (*pb.EmptyResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "请求参数不能为空")
	}

	email, ok := normalizeEmail(in.Email)
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "邮箱格式不正确")
	}

	code := strings.TrimSpace(in.Code)
	if !isSixDigitCode(code) {
		return nil, status.Error(codes.InvalidArgument, "验证码必须是 6 位数字")
	}
	if in.NewPassword == "" || !utf8.ValidString(in.NewPassword) || utf8.RuneCountInString(in.NewPassword) < 6 || len(in.NewPassword) > 72 {
		return nil, status.Error(codes.InvalidArgument, "新密码必须包含至少 6 个字符且不能超过 72 字节")
	}

	user, err := l.svcCtx.UsersModel.FindOneByEmail(l.ctx, email)
	if err != nil {
		l.Errorf("find user before resetting password: %v", err)
		return nil, status.Error(codes.Internal, "重置密码失败")
	}

	// 重置密码验证码按用途隔离、限制尝试次数且只能使用一次，不能与登录或注册验证码混用。
	matched, err := consumeEmailCode(
		l.ctx,
		l.svcCtx.Redis,
		emailCodePurposeResetPassword,
		email,
		code,
	)
	if err != nil {
		l.Errorf("verify reset password email code: %v", err)
		return nil, status.Error(codes.Unavailable, "验证码服务暂时不可用")
	}
	if !matched {
		return nil, status.Error(codes.Unauthenticated, "验证码错误或已过期")
	}
	if user == nil {
		// 对不存在的邮箱也返回成功，避免通过重置密码接口探测某个邮箱是否已经注册。
		return &pb.EmptyResp{}, nil
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(in.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		l.Errorf("generate reset password hash for user %d: %v", user.Id, err)
		return nil, status.Error(codes.Internal, "重置密码失败")
	}

	now := time.Now().UTC()
	err = l.svcCtx.DB.WithContext(l.ctx).Transaction(func(tx *gorm.DB) error {
		// 带上旧密码哈希作为并发条件，防止覆盖另一个请求刚刚修改的新密码。
		result := tx.Model(&model.Users{}).
			Where(
				"id = ? AND email = ? AND password_hash = ? AND deleted_at IS NULL",
				user.Id,
				email,
				user.PasswordHash,
			).
			Update("password_hash", string(passwordHash))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errResetPasswordConflict
		}

		// 密码重置属于高风险操作，必须撤销该账号所有设备的 Session，使旧 Token 全部失效。
		return tx.Model(&model.UserSessions{}).
			Where("user_id = ? AND revoked_at IS NULL", user.Id).
			Updates(map[string]any{
				"revoked_at":     now,
				"revoked_reason": "password_reset",
			}).Error
	})
	if err != nil {
		if errors.Is(err, errResetPasswordConflict) {
			return nil, status.Error(codes.Aborted, "账户状态已发生变化，请重新获取验证码")
		}
		l.Errorf("reset password for user %d: %v", user.Id, err)
		return nil, status.Error(codes.Internal, "重置密码失败")
	}

	// 清除旧的密码登录失败记录，避免用户重置成功后仍被该邮箱的限流状态阻挡。
	loginBuckets := passwordLoginRateLimitBuckets("email", email, loginClientInfo(l.ctx))
	if err := clearPasswordLoginFailures(l.ctx, l.svcCtx.Redis, loginBuckets); err != nil {
		l.Errorf("clear password login failures after reset for user %d: %v", user.Id, err)
	}

	// 审计日志只记录用户 ID，不能写入邮箱、验证码或任何形式的密码。
	l.Infof("reset password and revoked sessions for user %d", user.Id)

	return &pb.EmptyResp{}, nil
}
