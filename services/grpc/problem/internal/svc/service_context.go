package svc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/internal/config"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/model"

	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type ServiceContext struct {
	Config config.Config

	ProblemModel model.ProblemModel

	db    *gorm.DB
	redis *redis.Client
	sqlDB *sql.DB
}

func NewServiceContext(c config.Config) (*ServiceContext, error) {
	if c.Mysql.DSN == "" {
		return nil, errors.New("mysql DSN is required")
	}
	if c.RedisDB.Addr == "" {
		return nil, errors.New("redis address is required")
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

	return &ServiceContext{
		Config:       c,
		ProblemModel: model.NewProblemModel(db),
		db:           db,
		redis:        redisClient,
		sqlDB:        sqlDB,
	}, nil
}

func (s *ServiceContext) Close() error {
	var redisErr error
	if s.redis != nil {
		redisErr = s.redis.Close()
	}
	var sqlErr error
	if s.sqlDB != nil {
		sqlErr = s.sqlDB.Close()
	}
	return errors.Join(redisErr, sqlErr)
}
