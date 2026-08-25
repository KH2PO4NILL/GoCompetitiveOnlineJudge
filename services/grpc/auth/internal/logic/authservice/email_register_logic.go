package authservicelogic

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type EmailRegisterLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewEmailRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EmailRegisterLogic {
	return &EmailRegisterLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *EmailRegisterLogic) EmailRegister(in *pb.EmailRegisterReq) (*pb.RegisterResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "请求不能为空")
	}

	email, ok := normalizeEmail(in.Email)
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "邮箱格式不正确")
	}
	username := strings.TrimSpace(in.Username)
	if username == "" || !utf8.ValidString(username) || utf8.RuneCountInString(username) > 32 {
		return nil, status.Error(codes.InvalidArgument, "用户名必须包含 1 到 32 个字符")
	}
	if in.Password == "" || !utf8.ValidString(in.Password) || utf8.RuneCountInString(in.Password) < 6 || len(in.Password) > 72 {
		return nil, status.Error(codes.InvalidArgument, "密码必须包含至少 6 个字符且不能超过 72 字节")
	}
	code := strings.TrimSpace(in.Code)
	if !isSixDigitCode(code) {
		return nil, status.Error(codes.InvalidArgument, "验证码必须是 6 位数字")
	}

	if exists, err := l.svcCtx.UsersModel.ExistsByEmail(l.ctx, email); err != nil {
		l.Errorf("check email exists: %v", err)
		return nil, status.Error(codes.Internal, "注册失败")
	} else if exists {
		return nil, status.Error(codes.AlreadyExists, "邮箱已注册")
	}
	if exists, err := l.svcCtx.UsersModel.ExistsByUsername(l.ctx, username); err != nil {
		l.Errorf("check username exists: %v", err)
		return nil, status.Error(codes.Internal, "注册失败")
	} else if exists {
		return nil, status.Error(codes.AlreadyExists, "用户名已存在")
	}

	schoolID := sql.NullInt64{}
	if in.SchoolId > 0 {
		school, err := l.svcCtx.SchoolsModel.FindOne(l.ctx, in.SchoolId)
		if err != nil {
			l.Errorf("find school %d: %v", in.SchoolId, err)
			return nil, status.Error(codes.Internal, "注册失败")
		}
		if school == nil || school.Status != 1 {
			return nil, status.Error(codes.InvalidArgument, "学校不存在或不可用")
		}
		schoolID = sql.NullInt64{Int64: in.SchoolId, Valid: true}
	}

	// Registration codes are purpose-scoped, attempt-limited and one-time use.
	matched, err := consumeEmailCode(l.ctx, l.svcCtx.Redis, emailCodePurposeRegister, email, code)
	if err != nil {
		l.Errorf("verify registration email code: %v", err)
		return nil, status.Error(codes.Unavailable, "验证码服务暂时不可用")
	}
	if !matched {
		return nil, status.Error(codes.Unauthenticated, "验证码错误或已过期")
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		l.Errorf("generate password hash: %v", err)
		return nil, status.Error(codes.Internal, "注册失败")
	}

	newUser := &model.Users{
		Email:        email,
		Username:     username,
		PasswordHash: string(passwordHash),
		SchoolId:     schoolID,
		Role:         int64(pb.UserRole_USER_ROLE_USER),
		Status:       int64(pb.UserStatus_USER_STATUS_NORMAL),
	}
	if err := l.svcCtx.UsersModel.InsertOne(l.ctx, newUser); err != nil {
		if message, duplicate := registrationDuplicateMessage(err); duplicate {
			return nil, status.Error(codes.AlreadyExists, message)
		}
		l.Errorf("insert user: %v", err)
		return nil, status.Error(codes.Internal, "注册失败")
	}

	// Audit trail deliberately excludes email, password and verification code.
	l.Infof("registered user %d", newUser.Id)

	return &pb.RegisterResp{
		User: &pb.UserInfo{
			UserId:   newUser.Id,
			Username: newUser.Username,
			Email:    newUser.Email,
			Role:     pb.UserRole(newUser.Role),
			Status:   pb.UserStatus(newUser.Status),
			SchoolId: in.SchoolId,
		},
	}, nil
}

func registrationDuplicateMessage(err error) (string, bool) {
	var mysqlErr *mysqlDriver.MySQLError
	if !errors.As(err, &mysqlErr) || mysqlErr.Number != 1062 {
		return "", false
	}

	switch {
	case strings.Contains(mysqlErr.Message, "uk_users_email"):
		return "邮箱已注册", true
	case strings.Contains(mysqlErr.Message, "uk_users_username"):
		return "用户名已存在", true
	default:
		return "邮箱或用户名已存在", true
	}
}
