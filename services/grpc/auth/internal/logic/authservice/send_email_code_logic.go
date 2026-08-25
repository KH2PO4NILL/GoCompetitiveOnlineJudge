package authservicelogic

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

const (
	emailCodeTTL      = 5 * time.Minute
	emailCodeCooldown = time.Minute
)

const emailCodeBody = `
<h2>Go Competitive Online Judge</h2>
<p>你的验证码是：</p>
<h1>%s</h1>
<p>验证码 5 分钟内有效，请勿泄露给他人。</p>
`

type SendEmailCodeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSendEmailCodeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SendEmailCodeLogic {
	return &SendEmailCodeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func GenerateRandomCode() (string, error) {
	code, err := rand.Int(rand.Reader, big.NewInt(900_000))
	if err != nil {
		return "", fmt.Errorf("generate random code: %w", err)
	}
	return fmt.Sprintf("%06d", code.Int64()+100_000), nil
}

func (l *SendEmailCodeLogic) SendEmailCode(in *pb.SendEmailCodeReq) (*pb.SendEmailCodeResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "请求不能为空")
	}

	email, ok := normalizeEmail(in.Email)
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "邮箱格式不正确")
	}
	purpose, ok := parseEmailCodePurpose(in.Purpose)
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "验证码用途不正确")
	}

	// SetNX makes the per-email cooldown atomic under concurrent requests.
	cooldownKey := emailCodeCooldownKey(purpose, email)
	acquired, err := l.svcCtx.Redis.SetNX(l.ctx, cooldownKey, "1", emailCodeCooldown).Result()
	if err != nil {
		l.Errorf("reserve email code cooldown: %v", err)
		return nil, status.Error(codes.Unavailable, "验证码服务暂时不可用")
	}
	if !acquired {
		return nil, status.Error(codes.ResourceExhausted, "验证码发送过于频繁")
	}

	// 除单邮箱冷却外再限制单 IP 和整个服务的发送量，防止攻击者轮换邮箱消耗 SMTP 配额。
	limited, err := reserveEmailSendRate(l.ctx, l.svcCtx.Redis, emailSendRateBuckets(loginClientInfo(l.ctx)))
	if err != nil {
		l.cleanupEmailCode(purpose, email, "", cooldownKey)
		l.Errorf("reserve email send rate: %v", err)
		return nil, status.Error(codes.Unavailable, "验证码服务暂时不可用")
	}
	if limited {
		l.cleanupEmailCode(purpose, email, "", cooldownKey)
		return nil, status.Error(codes.ResourceExhausted, "验证码发送过于频繁")
	}

	allowed, err := l.emailRecipientAllowed(purpose, email)
	if err != nil {
		l.cleanupEmailCode(purpose, email, "", cooldownKey)
		l.Errorf("check email code recipient: %v", err)
		return nil, status.Error(codes.Unavailable, "验证码服务暂时不可用")
	}
	if !allowed {
		// 对“账号不存在”和“账号已存在”返回相同成功响应，避免泄露邮箱注册状态。
		return emailCodeSentResponse(), nil
	}

	code, err := GenerateRandomCode()
	if err != nil {
		l.cleanupEmailCode(purpose, email, "", cooldownKey)
		l.Errorf("generate random code: %v", err)
		return nil, status.Error(codes.Internal, "生成验证码失败")
	}

	codeKey := emailCodeKey(purpose, email)
	if err := l.svcCtx.Redis.Set(l.ctx, codeKey, code, emailCodeTTL).Err(); err != nil {
		l.cleanupEmailCode(purpose, email, code, cooldownKey)
		l.Errorf("store email code: %v", err)
		return nil, status.Error(codes.Unavailable, "验证码服务暂时不可用")
	}

	subject := "Go Competitive Online Judge - 验证码"
	if err := l.svcCtx.Mailer.SendHTML(l.ctx, email, subject, fmt.Sprintf(emailCodeBody, code)); err != nil {
		l.cleanupEmailCode(purpose, email, code, cooldownKey)
		l.Errorf("send email code: %v", err)
		return nil, status.Error(codes.Unavailable, "邮件服务暂时不可用")
	}

	return emailCodeSentResponse(), nil
}

func emailCodeSentResponse() *pb.SendEmailCodeResp {
	return &pb.SendEmailCodeResp{
		ExpiresIn:  int32(emailCodeTTL / time.Second),
		RetryAfter: int32(emailCodeCooldown / time.Second),
	}
}

func (l *SendEmailCodeLogic) emailRecipientAllowed(purpose emailCodePurpose, email string) (bool, error) {
	var user struct {
		Status int64 `gorm:"column:status"`
	}
	err := l.svcCtx.DB.WithContext(l.ctx).
		Model(&model.Users{}).
		Select("status").
		Where("email = ? AND deleted_at IS NULL", email).
		Take(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return purpose == emailCodePurposeRegister || purpose == emailCodePurposeChangeEmail, nil
	}
	if err != nil {
		return false, err
	}

	switch purpose {
	case emailCodePurposeLogin:
		return pb.UserStatus(user.Status) == pb.UserStatus_USER_STATUS_NORMAL, nil
	case emailCodePurposeResetPassword:
		return true, nil
	case emailCodePurposeRegister, emailCodePurposeChangeEmail:
		return false, nil
	default:
		return false, nil
	}
}

func (l *SendEmailCodeLogic) cleanupEmailCode(
	purpose emailCodePurpose,
	email string,
	code string,
	cooldownKey string,
) {
	// 请求可能已经超时或被客户端取消，清理必须使用独立且很短的上下文。
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(l.ctx), 3*time.Second)
	defer cancel()
	if code != "" {
		if err := deleteEmailCodeIfMatches(cleanupCtx, l.svcCtx.Redis, purpose, email, code); err != nil {
			l.Errorf("clean up unsent email code: %v", err)
		}
	}
	if err := l.svcCtx.Redis.Del(cleanupCtx, cooldownKey).Err(); err != nil {
		l.Errorf("clean up email cooldown: %v", err)
	}
}
