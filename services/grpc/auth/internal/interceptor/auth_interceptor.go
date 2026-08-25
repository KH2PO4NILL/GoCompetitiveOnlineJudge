package interceptor

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/authctx"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/token"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

var errSessionInvalid = errors.New("login session is invalid")

type accessTokenParser interface {
	ParseAccessToken(raw string) (*token.AccessClaims, error)
}

type sessionValidator func(ctx context.Context, claims *token.AccessClaims) error

func NewAuthInterceptor(svcCtx *svc.ServiceContext) grpc.UnaryServerInterceptor {
	return newAuthInterceptor(
		svcCtx.Token,
		func(ctx context.Context, claims *token.AccessClaims) error {
			return validateSession(ctx, svcCtx.DB, claims)
		},
	)
}

func newAuthInterceptor(
	parser accessTokenParser,
	validateSession sessionValidator,
) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		if isPublicMethod(info.FullMethod, req) {
			return handler(ctx, req)
		}
		if info.FullMethod == pb.AuthService_Logout_FullMethodName {
			logoutReq, ok := req.(*pb.LogoutReq)
			if ok {
				rawToken, metadataErr := accessTokenFromContext(ctx)
				if metadataErr != nil {
					rawToken = strings.TrimSpace(logoutReq.AccessToken)
				}
				if rawToken != "" {
					claims, parseErr := parser.ParseAccessToken(rawToken)
					if parseErr == nil && validateSession(ctx, claims) == nil {
						return handler(authctx.WithClaims(ctx, claims), req)
					}
				}
				// Access Token 过期后仍允许使用 Refresh Token 撤销服务端 Session。
				if strings.TrimSpace(logoutReq.RefreshToken) != "" {
					return handler(ctx, req)
				}
			}
			return nil, status.Error(codes.Unauthenticated, "missing or invalid logout token")
		}

		rawToken, err := accessTokenFromContext(ctx)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "missing or invalid access token")
		}

		claims, err := parser.ParseAccessToken(rawToken)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "access token is invalid or expired")
		}

		if err := validateSession(ctx, claims); err != nil {
			if errors.Is(err, errSessionInvalid) {
				return nil, status.Error(codes.Unauthenticated, "login session is invalid or expired")
			}
			return nil, status.Error(codes.Unavailable, "authentication service is unavailable")
		}

		return handler(authctx.WithClaims(ctx, claims), req)
	}
}

func isPublicMethod(fullMethod string, req any) bool {
	switch fullMethod {
	case pb.AuthService_EmailRegister_FullMethodName,
		pb.AuthService_UsernameLogin_FullMethodName,
		pb.AuthService_EmailPasswordLogin_FullMethodName,
		pb.AuthService_EmailCodeLogin_FullMethodName,
		pb.AuthService_ValidateToken_FullMethodName,
		pb.AuthService_RefreshToken_FullMethodName,
		pb.AuthService_ResetPassword_FullMethodName:
		return true
	case pb.AuthService_SendEmailCode_FullMethodName:
		request, ok := req.(*pb.SendEmailCodeReq)
		return ok && request.Purpose != pb.EmailCodePurpose_EMAIL_CODE_PURPOSE_CHANGE_EMAIL
	default:
		return false
	}
}

func accessTokenFromContext(ctx context.Context) (string, error) {
	values := metadata.ValueFromIncomingContext(ctx, "authorization")
	if len(values) != 1 {
		return "", errors.New("authorization metadata must occur exactly once")
	}

	parts := strings.Fields(values[0])
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", errors.New("authorization metadata is not a bearer token")
	}

	return parts[1], nil
}

func validateSession(ctx context.Context, db *gorm.DB, claims *token.AccessClaims) error {
	if db == nil || claims == nil {
		return errors.New("authentication dependencies are unavailable")
	}

	var session struct {
		UserID    int64      `gorm:"column:user_id"`
		ExpiresAt time.Time  `gorm:"column:expires_at"`
		RevokedAt *time.Time `gorm:"column:revoked_at"`
	}

	err := db.WithContext(ctx).
		Model(&model.UserSessions{}).
		Select("user_id", "expires_at", "revoked_at").
		Where("id = ? AND user_id = ?", claims.SessionID, claims.UserID).
		Take(&session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errSessionInvalid
	}
	if err != nil {
		return fmt.Errorf("query login session: %w", err)
	}

	if session.UserID != claims.UserID || session.RevokedAt != nil || !session.ExpiresAt.After(time.Now().UTC()) {
		return errSessionInvalid
	}

	return nil
}
