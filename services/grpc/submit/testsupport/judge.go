package testsupport

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	sharedauth "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/judgemq"
	problempb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/internal/outbox"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/internal/server"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"
	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type JudgeHarness struct {
	Model         model.SubmitModel
	DB            *gorm.DB
	Target        string
	Applied       atomic.Int32
	DropReport    atomic.Bool
	FailAcquire   atomic.Bool
	FailHeartbeat atomic.Bool
	LoseMark      atomic.Bool
}

type catalog struct{ spec *problempb.JudgeProblemSpec }

func (catalog catalog) GetJudgeProblemSpec(context.Context, int64, int64) (*problempb.JudgeProblemSpec, error) {
	return proto.Clone(catalog.spec).(*problempb.JudgeProblemSpec), nil
}

func NewJudgeHarness(t *testing.T, dsn string, spec *problempb.JudgeProblemSpec, token string) *JudgeHarness {
	t.Helper()
	settings, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		t.Fatal("invalid isolated test DSN")
	}
	settings.DBName, settings.ParseTime, settings.Loc = "", true, time.UTC
	settings.Timeout, settings.ReadTimeout, settings.WriteTimeout = 5*time.Second, 10*time.Second, 10*time.Second
	admin, err := sql.Open("mysql", settings.FormatDSN())
	if err != nil {
		t.Fatal("open test MySQL failed")
	}
	name := "judge_e2e_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.Exec("CREATE DATABASE `" + name + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"); err != nil {
		_ = admin.Close()
		t.Fatal("create isolated database failed")
	}
	t.Cleanup(func() {
		if _, err := admin.Exec("DROP DATABASE `" + name + "`"); err != nil {
			t.Error("isolated database cleanup failed")
		}
		_ = admin.Close()
	})
	settings.DBName = name
	database, err := sql.Open("mysql", settings.FormatDSN())
	if err != nil {
		t.Fatal("open isolated database failed")
	}
	t.Cleanup(func() { _ = database.Close() })
	_, file, _, _ := runtime.Caller(0)
	for _, migration := range []string{"001_create_submit.sql", "002_add_current_attempt_foreign_key.sql"} {
		body, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../migrations", migration))
		if err != nil {
			t.Fatal(err)
		}
		var lines []string
		for _, line := range strings.Split(string(body), "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "--") {
				lines = append(lines, line)
			}
		}
		for _, statement := range strings.Split(strings.Join(lines, "\n"), ";") {
			if strings.TrimSpace(statement) != "" {
				if _, err := database.Exec(statement); err != nil {
					t.Fatal("apply isolated migration failed", migration)
				}
			}
		}
	}
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: database}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal("open test model failed")
	}
	harness := &JudgeHarness{Model: model.NewSubmitModel(db), DB: db}
	faults := func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info.FullMethod == pb.SubmitService_AcquireJudgeTask_FullMethodName && harness.FailAcquire.Swap(false) {
			return nil, status.Error(codes.Unavailable, "injected temporary submit fault")
		}
		if info.FullMethod == pb.SubmitService_HeartbeatJudgeTask_FullMethodName && harness.FailHeartbeat.Swap(false) {
			return nil, status.Error(codes.Unavailable, "injected temporary heartbeat fault")
		}
		response, err := handler(ctx, request)
		if result, ok := response.(*pb.ReportJudgeResultResp); ok && err == nil && result.Disposition == pb.ReportDisposition_REPORT_DISPOSITION_APPLIED {
			harness.Applied.Add(1)
			if harness.DropReport.Swap(false) {
				return nil, status.Error(codes.Unavailable, "injected report response loss")
			}
		}
		return response, err
	}
	auth := sharedauth.NewServiceAuthInterceptor(map[string]string{"judge.worker": token}, func(string) []string { return []string{"judge.worker"} })
	rpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(auth, faults))
	pb.RegisterSubmitServiceServer(rpcServer, server.NewSubmitServiceServer(&svc.ServiceContext{SubmitModel: harness.Model, ProblemCatalog: catalog{spec}, LeaseDuration: 2 * time.Second}))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	harness.Target = listener.Addr().String()
	go func() { _ = rpcServer.Serve(listener) }()
	t.Cleanup(rpcServer.Stop)
	recovery, err := svc.NewLeaseRecoveryWorker(harness.Model, 100*time.Millisecond, 16, time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	recovery.Start()
	t.Cleanup(func() { _ = recovery.Stop() })
	return harness
}

type markFault struct {
	model.SubmitModel
	fail *atomic.Bool
}

func (repository markFault) MarkOutboxPublished(ctx context.Context, input model.MarkOutboxPublishedInput) error {
	if repository.fail.Swap(false) {
		return errors.New("injected mark response loss")
	}
	return repository.SubmitModel.MarkOutboxPublished(ctx, input)
}

func (harness *JudgeHarness) Publisher(t *testing.T, sender judgemq.Sender, logger *slog.Logger) *outbox.Publisher {
	t.Helper()
	publisher, err := outbox.New(markFault{harness.Model, &harness.LoseMark}, sender, outbox.Options{ID: uuid.NewString(), BatchSize: 4, Concurrency: 4, MaxAttempts: 8, Interval: 100 * time.Millisecond, ClaimDuration: 7 * time.Second, Timeout: 2 * time.Second, Backoff: 100 * time.Millisecond, MaxBackoff: time.Second}, logger)
	if err != nil {
		t.Fatal(err)
	}
	return publisher
}

func (harness *JudgeHarness) AssertAppliedOnce(t *testing.T, submissionID int64) {
	t.Helper()
	var count int64
	if err := harness.DB.Table("submission_attempts").Where("submission_id = ? AND status = ?", submissionID, model.AttemptStatusFinished).Count(&count).Error; err != nil {
		t.Fatal(fmt.Errorf("count finished attempts: %w", err))
	}
	if count != 1 {
		t.Fatalf("finished attempt count = %d, want 1", count)
	}
}
