package svc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	authservice "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/client/authservice"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/internal/config"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/model"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/zrpc"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type ServiceContext struct {
	Config config.Config

	ProblemModel    model.ProblemModel
	AuthClient      authservice.AuthService
	TestDataStorage TestDataStorage
	ContestAccess   ContestAccessChecker
	UploadPolicy    UploadPolicy

	InternalCredentials map[string]string
	InternalCallers     map[string][]string

	authRPC    zrpc.Client
	contestRPC zrpc.Client
	db         *gorm.DB
	redis      *redis.Client
	sqlDB      *sql.DB
}

func NewServiceContext(c config.Config) (*ServiceContext, error) {
	if c.Mysql.DSN == "" {
		return nil, errors.New("mysql DSN is required")
	}
	if c.RedisDB.Addr == "" {
		return nil, errors.New("redis address is required")
	}
	uploadPolicy, err := NewUploadPolicy(c.Upload)
	if err != nil {
		return nil, fmt.Errorf("configure test data uploads: %w", err)
	}
	storage, err := NewS3TestDataStorage(c.ObjectStorage, uploadPolicy)
	if err != nil {
		return nil, fmt.Errorf("configure test data storage: %w", err)
	}
	credentials, callers, err := internalAuthConfiguration(c.InternalAuth)
	if err != nil {
		return nil, fmt.Errorf("configure internal authentication: %w", err)
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

	authRPC, err := zrpc.NewClient(c.AuthRPC)
	if err != nil {
		_ = redisClient.Close()
		_ = sqlDB.Close()
		return nil, fmt.Errorf("create auth rpc client: %w", err)
	}
	contestRPC, err := zrpc.NewClient(c.ContestRPC)
	if err != nil {
		_ = authRPC.Conn().Close()
		_ = redisClient.Close()
		_ = sqlDB.Close()
		return nil, fmt.Errorf("create contest rpc client: %w", err)
	}

	return &ServiceContext{
		Config:              c,
		ProblemModel:        model.NewProblemModel(db),
		AuthClient:          authservice.NewAuthService(authRPC),
		ContestAccess:       NewContestAccessChecker(contestRPC.Conn()),
		TestDataStorage:     storage,
		UploadPolicy:        uploadPolicy,
		InternalCredentials: credentials,
		InternalCallers:     callers,
		authRPC:             authRPC,
		contestRPC:          contestRPC,
		db:                  db,
		redis:               redisClient,
		sqlDB:               sqlDB,
	}, nil
}

func internalAuthConfiguration(c config.InternalAuthConf) (map[string]string, map[string][]string, error) {
	if len(c.Callers) == 0 {
		return nil, nil, errors.New("at least one internal caller is required")
	}
	credentials := make(map[string]string, len(c.Callers))
	callers := make(map[string][]string)
	for _, caller := range c.Callers {
		name := strings.TrimSpace(caller.Name)
		if name == "" || caller.Token == "" || len(caller.Methods) == 0 {
			return nil, nil, errors.New("each internal caller requires name, token and methods")
		}
		if _, exists := credentials[name]; exists {
			return nil, nil, fmt.Errorf("duplicate internal caller %q", name)
		}
		credentials[name] = caller.Token
		for _, method := range caller.Methods {
			method = strings.TrimSpace(method)
			if method == "" {
				return nil, nil, fmt.Errorf("internal caller %q has an empty method", name)
			}
			callers[method] = append(callers[method], name)
		}
	}
	if len(callers["/problem.ProblemService/BatchGetProblems"]) == 0 ||
		len(callers["/problem.ProblemService/GetJudgeProblemSpec"]) == 0 {
		return nil, nil, errors.New("internal callers must cover BatchGetProblems and GetJudgeProblemSpec")
	}
	return credentials, callers, nil
}

func (s *ServiceContext) Close() error {
	var authErr error
	if s.authRPC != nil && s.authRPC.Conn() != nil {
		authErr = s.authRPC.Conn().Close()
	}
	var contestErr error
	if s.contestRPC != nil && s.contestRPC.Conn() != nil {
		contestErr = s.contestRPC.Conn().Close()
	}
	var redisErr error
	if s.redis != nil {
		redisErr = s.redis.Close()
	}
	var sqlErr error
	if s.sqlDB != nil {
		sqlErr = s.sqlDB.Close()
	}
	return errors.Join(authErr, contestErr, redisErr, sqlErr)
}
