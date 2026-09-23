package svc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/judgemetrics"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/judgemq"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/judgepolicy"
	"github.com/google/uuid"

	sharedauth "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"
	authservice "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/client/authservice"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/internal/config"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/internal/outbox"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/zrpc"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type ServiceContext struct {
	Config config.Config

	SubmitModel    model.SubmitModel
	AuthClient     authservice.AuthService
	ProblemCatalog ProblemSpecCatalog
	ContestPolicy  ContestPolicy
	RateLimiter    SubmissionRateLimiter
	Now            func() time.Time
	LeaseDuration  time.Duration

	InternalCredentials map[string]string
	InternalCallers     map[string][]string

	authRPC     zrpc.Client
	problemRPC  zrpc.Client
	contestRPC  zrpc.Client
	redis       *redis.Client
	leaseWorker *LeaseRecoveryWorker
	db          *gorm.DB
	sqlDB       *sql.DB
	publisher   *outbox.Publisher
	producer    *judgemq.Producer
	metrics     *http.Server
}

func NewServiceContext(c config.Config) (*ServiceContext, error) {
	if err := c.ApplyEnvironment(); err != nil {
		return nil, errors.New("SUBMIT_OUTBOX_ENABLED must be a boolean")
	}
	if c.Mysql.DSN == "" {
		return nil, errors.New("mysql DSN is required")
	}
	if err := validateRuntimeConfig(c); err != nil {
		return nil, err
	}
	credentials, callers, err := sharedauth.BuildServiceAuthentication(
		c.InternalAuth,
		pb.SubmitService_AcquireJudgeTask_FullMethodName,
		pb.SubmitService_HeartbeatJudgeTask_FullMethodName,
		pb.SubmitService_ReportJudgeResult_FullMethodName,
	)
	if err != nil {
		return nil, fmt.Errorf("configure internal authentication: %w", err)
	}
	for name, credential := range map[string]sharedauth.ServiceClientAuthConf{
		"auth": c.AuthRPCAuth, "problem": c.ProblemRPCAuth, "contest": c.ContestRPCAuth,
	} {
		if err := credential.Validate(); err != nil {
			return nil, fmt.Errorf("configure %s RPC authentication: %w", name, err)
		}
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
		return nil, fmt.Errorf("create auth RPC client: %w", err)
	}
	problemRPC, err := zrpc.NewClient(c.ProblemRPC)
	if err != nil {
		_ = authRPC.Conn().Close()
		_ = sqlDB.Close()
		return nil, fmt.Errorf("create problem RPC client: %w", err)
	}
	contestRPC, err := zrpc.NewClient(c.ContestRPC)
	if err != nil {
		_ = problemRPC.Conn().Close()
		_ = authRPC.Conn().Close()
		_ = sqlDB.Close()
		return nil, fmt.Errorf("create contest RPC client: %w", err)
	}

	var redisClient *redis.Client
	var limiter SubmissionRateLimiter = NoopSubmissionRateLimiter{}
	if c.RateLimit.Enabled {
		redisClient = redis.NewClient(&redis.Options{
			Addr: c.RedisDB.Addr, Password: c.RedisDB.Password, DB: c.RedisDB.DB,
		})
		redisPingCtx, redisPingCancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := redisClient.Ping(redisPingCtx).Err()
		redisPingCancel()
		if err != nil {
			_ = redisClient.Close()
			_ = contestRPC.Conn().Close()
			_ = problemRPC.Conn().Close()
			_ = authRPC.Conn().Close()
			_ = sqlDB.Close()
			return nil, fmt.Errorf("ping redis: %w", err)
		}
		limiter, err = NewRedisSubmissionRateLimiter(
			redisClient,
			time.Duration(c.RateLimit.WindowSeconds)*time.Second,
			c.RateLimit.MaxSubmissions,
		)
		if err != nil {
			_ = redisClient.Close()
			_ = contestRPC.Conn().Close()
			_ = problemRPC.Conn().Close()
			_ = authRPC.Conn().Close()
			_ = sqlDB.Close()
			return nil, fmt.Errorf("configure submission rate limiter: %w", err)
		}
	}
	clock := func() time.Time { return time.Now().UTC() }
	repository := model.NewSubmitModel(db, model.WithClock(clock))
	worker, err := NewLeaseRecoveryWorker(
		repository,
		time.Duration(c.Lease.RecoveryIntervalSeconds)*time.Second,
		c.Lease.RecoveryBatchSize,
		time.Duration(c.Lease.RecoveryTimeoutSeconds)*time.Second,
		clock,
	)
	if err != nil {
		if redisClient != nil {
			_ = redisClient.Close()
		}
		_ = contestRPC.Conn().Close()
		_ = problemRPC.Conn().Close()
		_ = authRPC.Conn().Close()
		_ = sqlDB.Close()
		return nil, fmt.Errorf("configure lease recovery: %w", err)
	}
	result := &ServiceContext{
		Config: c, SubmitModel: repository,
		AuthClient:     authservice.NewAuthService(authRPC),
		ProblemCatalog: NewRPCProblemSpecCatalog(problemRPC.Conn(), c.ProblemRPCAuth),
		ContestPolicy:  NewRPCContestPolicy(contestRPC.Conn(), c.ContestRPCAuth),
		RateLimiter:    limiter, Now: clock,
		LeaseDuration:       time.Duration(c.Lease.DurationSeconds) * time.Second,
		InternalCredentials: credentials, InternalCallers: callers,
		authRPC: authRPC, problemRPC: problemRPC, contestRPC: contestRPC,
		redis: redisClient, leaseWorker: worker, db: db, sqlDB: sqlDB,
	}
	worker.Start()
	if c.Outbox.Enabled {
		sender, err := judgemq.NewProducer(judgemq.Config{Endpoint: c.Outbox.Endpoint, TLS: c.Outbox.TLS, AccessKey: c.Outbox.AccessKey, SecretKey: c.Outbox.SecretKey, Timeout: time.Duration(c.Outbox.TimeoutSeconds) * time.Second})
		if err != nil {
			_ = result.Close()
			return nil, fmt.Errorf("configure outbox transport: %w", err)
		}
		result.producer = sender
		publisher, err := outbox.New(repository, sender, c.Outbox.Options(c.Outbox.ID+"-"+uuid.NewString()), slog.New(slog.NewJSONHandler(os.Stdout, nil)))
		if err != nil {
			_ = result.Close()
			return nil, err
		}
		result.publisher = publisher
		if c.Outbox.MetricsListenOn != "" {
			result.metrics, err = publisher.Metrics.Listen(c.Outbox.MetricsListenOn, "/metrics")
			if err != nil {
				_ = result.Close()
				return nil, err
			}
		}
		publisher.Start()
	}
	return result, nil
}

func (s *ServiceContext) Close() error {
	if s == nil {
		return nil
	}
	var workerErr error
	var publisherErr, producerErr, metricsErr error
	if s.publisher != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.Config.Outbox.ShutdownSeconds)*time.Second)
		publisherErr = s.publisher.Stop(ctx)
		cancel()
	}
	if s.producer != nil {
		producerErr = s.producer.Close()
	}
	if s.metrics != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		metricsErr = judgemetrics.Close(ctx, s.metrics)
		cancel()
	}
	if s.leaseWorker != nil {
		workerErr = s.leaseWorker.Stop()
	}
	var authErr error
	if s.authRPC != nil && s.authRPC.Conn() != nil {
		authErr = s.authRPC.Conn().Close()
	}
	var problemErr error
	if s.problemRPC != nil && s.problemRPC.Conn() != nil {
		problemErr = s.problemRPC.Conn().Close()
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
	return errors.Join(workerErr, publisherErr, producerErr, metricsErr, authErr, problemErr, contestErr, redisErr, sqlErr)
}

func validateRuntimeConfig(c config.Config) error {
	if c.Outbox.Enabled {
		if c.Outbox.Endpoint == "" || len(c.Outbox.ID) > 64 || c.Outbox.ShutdownSeconds < 1 || c.Outbox.ShutdownSeconds > 300 {
			return errors.New("invalid outbox endpoint, instance ID or shutdown timeout")
		}
		if err := c.Outbox.Options(c.Outbox.ID + "-instance").Validate(); err != nil {
			return err
		}
	}
	if c.Limits.MaxSourceCodeBytes <= 0 || c.Limits.MaxSourceCodeBytes > judgepolicy.MaxSourceCodeBytes ||
		c.Limits.MaxClientRequestBytes <= 0 || c.Limits.MaxClientRequestBytes > 128 ||
		c.Limits.MaxPageSize <= 0 || c.Limits.MaxPageSize > 100 ||
		c.Limits.MaxBatchRejudge <= 0 || c.Limits.MaxBatchRejudge > 100 ||
		c.Limits.MaxCaseResults <= 0 || c.Limits.MaxCaseResults > 100000 || c.Limits.MaxCompileOutputBytes <= 0 ||
		c.Limits.MaxCompileOutputBytes > 4*1024*1024 || c.Limits.MaxJudgeMessageBytes <= 0 ||
		c.Limits.MaxJudgeMessageBytes > 1024*1024 {
		return errors.New("invalid submit limits configuration")
	}
	if c.Lease.DurationSeconds <= 0 || c.Lease.DurationSeconds > int64((24*time.Hour)/time.Second) ||
		c.Lease.RecoveryIntervalSeconds <= 0 || c.Lease.RecoveryBatchSize <= 0 ||
		c.Lease.RecoveryBatchSize > 1000 || c.Lease.RecoveryTimeoutSeconds <= 0 {
		return errors.New("invalid submit lease configuration")
	}
	if c.RateLimit.Enabled && (c.RedisDB.Addr == "" || c.RateLimit.WindowSeconds <= 0 || c.RateLimit.MaxSubmissions <= 0) {
		return errors.New("invalid submit rate limit configuration")
	}
	return nil
}
