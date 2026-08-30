package svc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	sharedauth "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"
	authservice "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/client/authservice"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/config"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"github.com/zeromicro/go-zero/zrpc"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type ServiceContext struct {
	Config config.Config

	ContestModel model.ContestModel
	AuthClient   authservice.AuthService

	InternalCredentials map[string]string
	InternalCallers     map[string][]string

	authRPC zrpc.Client
	db      *gorm.DB
	sqlDB   *sql.DB
}

func NewServiceContext(c config.Config) (*ServiceContext, error) {
	if c.Mysql.DSN == "" {
		return nil, errors.New("mysql DSN is required")
	}
	credentials, callers, err := sharedauth.BuildServiceAuthentication(
		c.InternalAuth,
		pb.ContestService_CanAccessProblem_FullMethodName,
		pb.ContestService_ValidateContestSubmission_FullMethodName,
	)
	if err != nil {
		return nil, fmt.Errorf("configure internal authentication: %w", err)
	}
	if err := c.AuthRPCAuth.Validate(); err != nil {
		return nil, fmt.Errorf("configure auth RPC authentication: %w", err)
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
	authRPC, err := zrpc.NewClient(c.AuthRPC)
	if err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("create auth rpc client: %w", err)
	}
	return &ServiceContext{
		Config:              c,
		ContestModel:        model.NewContestModel(db),
		AuthClient:          authservice.NewAuthService(authRPC),
		InternalCredentials: credentials,
		InternalCallers:     callers,
		authRPC:             authRPC,
		db:                  db,
		sqlDB:               sqlDB,
	}, nil
}

func (s *ServiceContext) Close() error {
	var authErr error
	if s.authRPC != nil && s.authRPC.Conn() != nil {
		authErr = s.authRPC.Conn().Close()
	}
	var sqlErr error
	if s.sqlDB != nil {
		sqlErr = s.sqlDB.Close()
	}
	return errors.Join(authErr, sqlErr)
}
