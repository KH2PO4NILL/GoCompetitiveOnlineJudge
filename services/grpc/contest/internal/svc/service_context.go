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
	problemservice "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/problemservice"

	"github.com/zeromicro/go-zero/zrpc"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type ServiceContext struct {
	Config config.Config

	ContestModel   model.ContestModel
	AuthClient     authservice.AuthService
	ProblemCatalog ProblemCatalog
	Now            func() time.Time

	InternalCredentials map[string]string
	InternalCallers     map[string][]string

	authRPC      zrpc.Client
	problemRPC   zrpc.Client
	freezeWorker *ScoreboardFreezeWorker
	db           *gorm.DB
	sqlDB        *sql.DB
}

func NewServiceContext(c config.Config) (*ServiceContext, error) {
	if c.Mysql.DSN == "" {
		return nil, errors.New("mysql DSN is required")
	}
	credentials, callers, err := sharedauth.BuildServiceAuthentication(
		c.InternalAuth,
		pb.ContestService_CanAccessProblem_FullMethodName,
		pb.ContestService_ValidateContestSubmission_FullMethodName,
		pb.ContestService_ResolveSubmissionResultVisibility_FullMethodName,
	)
	if err != nil {
		return nil, fmt.Errorf("configure internal authentication: %w", err)
	}
	if err := c.AuthRPCAuth.Validate(); err != nil {
		return nil, fmt.Errorf("configure auth RPC authentication: %w", err)
	}
	if err := c.ProblemRPCAuth.Validate(); err != nil {
		return nil, fmt.Errorf("configure problem RPC authentication: %w", err)
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
	problemRPC, err := zrpc.NewClient(c.ProblemRPC)
	if err != nil {
		_ = authRPC.Conn().Close()
		_ = sqlDB.Close()
		return nil, fmt.Errorf("create problem rpc client: %w", err)
	}
	clock := func() time.Time { return time.Now().UTC() }
	contestModel := model.NewContestModel(db, model.WithClock(clock))
	ctx := &ServiceContext{
		Config:              c,
		ContestModel:        contestModel,
		AuthClient:          authservice.NewAuthService(authRPC),
		ProblemCatalog:      NewRPCProblemCatalog(problemservice.NewProblemService(problemRPC), c.ProblemRPCAuth),
		Now:                 clock,
		InternalCredentials: credentials,
		InternalCallers:     callers,
		authRPC:             authRPC,
		problemRPC:          problemRPC,
		db:                  db,
		sqlDB:               sqlDB,
	}
	worker, err := NewScoreboardFreezeWorker(contestModel, c.ScoreboardFreeze, ctx.Now)
	if err != nil {
		_ = problemRPC.Conn().Close()
		_ = authRPC.Conn().Close()
		_ = sqlDB.Close()
		return nil, fmt.Errorf("configure scoreboard freeze worker: %w", err)
	}
	ctx.freezeWorker = worker
	worker.Start()
	return ctx, nil
}

func (s *ServiceContext) Close() error {
	var workerErr error
	if s.freezeWorker != nil {
		workerErr = s.freezeWorker.Stop()
	}
	var authErr error
	if s.authRPC != nil && s.authRPC.Conn() != nil {
		authErr = s.authRPC.Conn().Close()
	}
	var problemErr error
	if s.problemRPC != nil && s.problemRPC.Conn() != nil {
		problemErr = s.problemRPC.Conn().Close()
	}
	var sqlErr error
	if s.sqlDB != nil {
		sqlErr = s.sqlDB.Close()
	}
	return errors.Join(workerErr, authErr, problemErr, sqlErr)
}
