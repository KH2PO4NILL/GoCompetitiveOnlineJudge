package svc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/config"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/mailer"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/token"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/model"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type ServiceContext struct {
	Config config.Config

	DB           *gorm.DB
	Redis        *redis.Client
	Token        *token.Manager
	Mailer       mailer.Sender
	UsersModel   model.UsersModel
	SchoolsModel model.SchoolsModel

	sqlDB         *sql.DB
	cleanupCancel context.CancelFunc
	cleanupDone   chan struct{}
}

func NewServiceContext(c config.Config) (*ServiceContext, error) {
	if c.Token.RefreshExpireSeconds <= 0 {
		return nil, fmt.Errorf("refresh token expiration must be positive")
	}

	gormLogger := logger.New(
		log.New(os.Stdout, "", log.LstdFlags),
		logger.Config{
			SlowThreshold:             500 * time.Millisecond,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
			Colorful:                  false,
		},
	)
	db, err := gorm.Open(mysql.Open(c.Mysql.DSN), &gorm.Config{Logger: gormLogger})
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql database: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(pingCtx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping mysql: %w", err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     c.RedisDB.Addr,
		Password: c.RedisDB.Password,
		DB:       c.RedisDB.DB,
	})

	if err := redisClient.Ping(pingCtx).Err(); err != nil {
		_ = redisClient.Close()
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	tokenManager, err := token.NewManager(token.ManagerConfig{
		Secret:    c.Token.Secret,
		Issuer:    c.Token.Issuer,
		Audience:  c.Token.Audience,
		AccessTTL: time.Duration(c.Token.AccessExpireSeconds) * time.Second,
		Leeway:    time.Duration(c.Token.LeewaySeconds) * time.Second,
	})
	if err != nil {
		_ = redisClient.Close()
		_ = sqlDB.Close()
		return nil, fmt.Errorf("initialize token manager: %w", err)
	}

	mailSender, err := mailer.NewSMTP(c.Mail)
	if err != nil {
		_ = redisClient.Close()
		_ = sqlDB.Close()
		return nil, fmt.Errorf("initialize mail sender: %w", err)
	}
	serviceContext := &ServiceContext{
		Config: c,
		DB:     db,
		Redis:  redisClient,
		Token:  tokenManager,
		Mailer: mailSender,

		UsersModel:   model.NewUsersModel(db),
		SchoolsModel: model.NewSchoolsModel(db),
		sqlDB:        sqlDB,
	}
	serviceContext.startSessionCleanup()
	return serviceContext, nil
}

func (s *ServiceContext) Close() error {
	if s.cleanupCancel != nil {
		s.cleanupCancel()
		<-s.cleanupDone
	}
	return errors.Join(
		s.Redis.Close(),
		s.sqlDB.Close(),
	)
}

func (s *ServiceContext) startSessionCleanup() {
	interval := time.Duration(s.Config.Session.CleanupIntervalSeconds) * time.Second
	retention := time.Duration(s.Config.Session.RevokedRetentionSeconds) * time.Second
	if interval <= 0 || retention <= 0 {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cleanupCancel = cancel
	s.cleanupDone = make(chan struct{})
	go func() {
		defer close(s.cleanupDone)
		if err := s.cleanupSessions(ctx, retention); err != nil && !errors.Is(err, context.Canceled) {
			logx.Errorf("initial user session cleanup: %v", err)
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.cleanupSessions(ctx, retention); err != nil && !errors.Is(err, context.Canceled) {
					logx.Errorf("periodic user session cleanup: %v", err)
				}
			}
		}
	}()
}

func (s *ServiceContext) cleanupSessions(ctx context.Context, revokedRetention time.Duration) error {
	now := time.Now().UTC()
	revokedBefore := now.Add(-revokedRetention)
	return s.DB.WithContext(ctx).
		Where("expires_at <= ? OR (revoked_at IS NOT NULL AND revoked_at <= ?)", now, revokedBefore).
		Delete(&model.UserSessions{}).Error
}
