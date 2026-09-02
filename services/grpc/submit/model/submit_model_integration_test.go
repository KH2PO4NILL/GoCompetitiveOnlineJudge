package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSubmitModelCreateIsConcurrentAndPayloadSafe(t *testing.T) {
	repository, db, _ := newSubmitModelTestDatabase(t)
	ctx := context.Background()
	input := testCreateSubmissionInput("concurrent-create", nil)
	results := make([]*CreateSubmissionResult, 2)
	errorsFound := make([]error, 2)
	var group sync.WaitGroup
	start := make(chan struct{})
	for index := range results {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			<-start
			results[index], errorsFound[index] = repository.CreateSubmission(ctx, input)
		}(index)
	}
	close(start)
	group.Wait()
	for index, err := range errorsFound {
		if err != nil {
			t.Fatalf("CreateSubmission(%d) error = %v", index, err)
		}
	}
	if results[0].Submission.SubmissionID != results[1].Submission.SubmissionID ||
		results[0].Deduplicated == results[1].Deduplicated {
		t.Fatalf("concurrent results = %+v %+v", results[0], results[1])
	}
	assertTableCount(t, db, "submissions", 1)
	assertTableCount(t, db, "submission_attempts", 1)
	assertTableCount(t, db, "submit_outbox_events", 1)

	retry := input
	retry.ProblemRevision = 99
	retry.SubmittedAt = input.SubmittedAt.Add(time.Hour)
	deduplicated, err := repository.CreateSubmission(ctx, retry)
	if err != nil || !deduplicated.Deduplicated || deduplicated.Submission.ProblemRevision != input.ProblemRevision {
		t.Fatalf("stable retry = %+v err=%v", deduplicated, err)
	}
	conflict := input
	conflict.SourceCode += "\n// different"
	if _, err := repository.CreateSubmission(ctx, conflict); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("payload conflict error = %v", err)
	}
}

func TestSubmitModelWorkerLifecycleAndReportIdempotency(t *testing.T) {
	repository, db, clock := newSubmitModelTestDatabase(t)
	ctx := context.Background()
	created, err := repository.CreateSubmission(ctx, testCreateSubmissionInput("worker-lifecycle", nil))
	if err != nil {
		t.Fatalf("CreateSubmission() error = %v", err)
	}
	attemptID := created.Submission.CurrentAttemptID

	type acquireResult struct {
		result *AcquireAttemptResult
		err    error
	}
	acquired := make([]acquireResult, 2)
	var group sync.WaitGroup
	start := make(chan struct{})
	for index := range acquired {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			<-start
			result, acquireErr := repository.AcquireAttempt(ctx, AcquireAttemptInput{
				WorkerID: fmt.Sprintf("worker-%d", index), SubmissionID: created.Submission.SubmissionID,
				AttemptID: attemptID, DeliveryID: fmt.Sprintf("delivery-%d", index), LeaseDuration: 5 * time.Minute,
			})
			acquired[index] = acquireResult{result, acquireErr}
		}(index)
	}
	close(start)
	group.Wait()
	var winner *AcquireAttemptResult
	denied := 0
	for _, item := range acquired {
		if item.err != nil {
			t.Fatalf("AcquireAttempt() error = %v", item.err)
		}
		if item.result.Acquired {
			winner = item.result
		} else if item.result.DenyReason == AcquireDenyLeasedByOtherWorker {
			denied++
		}
	}
	if winner == nil || denied != 1 {
		t.Fatalf("acquire race = %+v", acquired)
	}

	heartbeat, err := repository.HeartbeatAttempt(ctx, HeartbeatAttemptInput{
		WorkerID: strings.TrimPrefix(winner.LeaseToken, "impossible"),
	})
	if err == nil || heartbeat != nil {
		t.Fatal("invalid heartbeat unexpectedly succeeded")
	}
	workerID := "worker-0"
	if acquired[1].result == winner {
		workerID = "worker-1"
	}
	heartbeat, err = repository.HeartbeatAttempt(ctx, HeartbeatAttemptInput{
		WorkerID: workerID, SubmissionID: winner.SubmissionID, AttemptID: winner.AttemptID,
		LeaseToken: winner.LeaseToken, CompletedTestCases: 1, TotalTestCases: 2, LeaseDuration: 5 * time.Minute,
	})
	if err != nil || heartbeat.CompletedTestCases != 1 {
		t.Fatalf("HeartbeatAttempt() = %+v err=%v", heartbeat, err)
	}
	clock.Advance(time.Minute)

	report := ApplyJudgeResultInput{
		WorkerID: workerID, SubmissionID: winner.SubmissionID, AttemptID: winner.AttemptID,
		LeaseToken: winner.LeaseToken, ReportID: "report-1", Verdict: VerdictAccepted,
		Score: 100, MaxScore: 100, CompileOutput: "compiled",
		CaseResults: []CaseResult{
			{TestCaseID: 101, Ordinal: 0, GroupName: "sample", Verdict: VerdictAccepted, Score: 40, TimeUsedMS: 2, MemoryUsedKB: 10, JudgeMessage: "internal-1"},
			{TestCaseID: 102, Ordinal: 1, GroupName: "main", Verdict: VerdictAccepted, Score: 60, TimeUsedMS: 3, MemoryUsedKB: 11, JudgeMessage: "internal-2"},
		},
	}
	applied, err := repository.ApplyJudgeResult(ctx, report)
	if err != nil || applied.Disposition != ReportDispositionApplied || applied.Submission.Status != SubmissionStatusFinished {
		t.Fatalf("ApplyJudgeResult() = %+v err=%v", applied, err)
	}
	duplicate, err := repository.ApplyJudgeResult(ctx, report)
	if err != nil || duplicate.Disposition != ReportDispositionDuplicate {
		t.Fatalf("duplicate ApplyJudgeResult() = %+v err=%v", duplicate, err)
	}
	changed := report
	changed.CaseResults = append([]CaseResult(nil), report.CaseResults...)
	changed.CaseResults[0].JudgeMessage = "changed"
	if _, err := repository.ApplyJudgeResult(ctx, changed); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("changed report error = %v", err)
	}
	assertTableCount(t, db, "submission_case_results", 2)
	assertTableCount(t, db, "submit_outbox_events", 1)
	var persisted attemptRow
	if err := db.Where("id = ?", winner.AttemptID).Take(&persisted).Error; err != nil {
		t.Fatal(err)
	}
	if string(persisted.LeaseTokenHash) == winner.LeaseToken || len(persisted.LeaseTokenHash) != 32 {
		t.Fatal("lease token was not stored as a digest")
	}
	if persisted.CompletedTestCases != int32(len(report.CaseResults)) {
		t.Fatalf("completed test cases = %d, want %d", persisted.CompletedTestCases, len(report.CaseResults))
	}
	clock.Advance(time.Minute)
	if _, err := repository.RejudgeSubmission(ctx, RejudgeSubmissionInput{
		SubmissionID: winner.SubmissionID, RequestedBy: 99, Reason: "verify stale report",
	}); err != nil {
		t.Fatalf("RejudgeSubmission() error = %v", err)
	}
	differentReport := report
	differentReport.ReportID = "another-report"
	if _, err := repository.ApplyJudgeResult(ctx, differentReport); !errors.Is(err, ErrStaleAttempt) {
		t.Fatalf("old attempt with another report error = %v", err)
	}
	if duplicate, err := repository.ApplyJudgeResult(ctx, report); err != nil || duplicate.Disposition != ReportDispositionDuplicate {
		t.Fatalf("old attempt exact retry = %+v err=%v", duplicate, err)
	}
}

func TestSubmitModelRejudgeAndExpiredLeaseProtectCurrentAttempt(t *testing.T) {
	repository, db, clock := newSubmitModelTestDatabase(t)
	ctx := context.Background()
	created, err := repository.CreateSubmission(ctx, testCreateSubmissionInput("lease-retry", nil))
	if err != nil {
		t.Fatal(err)
	}
	firstAttempt := created.Submission.CurrentAttemptID
	lease, err := repository.AcquireAttempt(ctx, AcquireAttemptInput{
		WorkerID: "worker", SubmissionID: created.Submission.SubmissionID, AttemptID: firstAttempt,
		DeliveryID: "delivery", LeaseDuration: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	clock.Advance(2 * time.Minute)
	if _, err := repository.ApplyJudgeResult(ctx, ApplyJudgeResultInput{
		WorkerID: "worker", SubmissionID: created.Submission.SubmissionID, AttemptID: firstAttempt,
		LeaseToken: lease.LeaseToken, ReportID: "expired-report", Verdict: VerdictSystemError,
		Score: 0, MaxScore: 100,
	}); !errors.Is(err, ErrLeaseExpired) {
		t.Fatalf("expired ApplyJudgeResult() error = %v", err)
	}
	retried, err := repository.SupersedeExpiredAttempt(ctx, SupersedeAttemptInput{
		SubmissionID: created.Submission.SubmissionID, AttemptID: firstAttempt,
	})
	if err != nil {
		t.Fatalf("SupersedeExpiredAttempt() error = %v", err)
	}
	if retried.AttemptID == firstAttempt || retried.Kind != AttemptKindLeaseRetry {
		t.Fatalf("retry attempt = %+v", retried)
	}
	if _, err := repository.HeartbeatAttempt(ctx, HeartbeatAttemptInput{
		WorkerID: "worker", SubmissionID: created.Submission.SubmissionID, AttemptID: firstAttempt,
		LeaseToken: lease.LeaseToken, CompletedTestCases: 0, TotalTestCases: 1, LeaseDuration: time.Minute,
	}); !errors.Is(err, ErrStaleAttempt) {
		t.Fatalf("old heartbeat error = %v", err)
	}
	rejudged, err := repository.RejudgeSubmission(ctx, RejudgeSubmissionInput{
		SubmissionID: created.Submission.SubmissionID, RequestedBy: 99, Reason: "admin rejudge",
	})
	if err != nil {
		t.Fatalf("RejudgeSubmission() error = %v", err)
	}
	if rejudged.Submission.ProblemRevision != created.Submission.ProblemRevision || rejudged.AttemptNo != 3 {
		t.Fatalf("rejudge result = %+v", rejudged)
	}
	var old attemptRow
	if err := db.Where("id = ?", firstAttempt).Take(&old).Error; err != nil {
		t.Fatal(err)
	}
	if old.Status != AttemptStatusSuperseded {
		t.Fatalf("old attempt status = %d", old.Status)
	}
	assertTableCount(t, db, "submission_attempts", 3)
	assertTableCount(t, db, "submit_outbox_events", 3)
	if _, err := repository.BatchRejudgeSubmissions(ctx, BatchRejudgeInput{
		SubmissionIDs: []int64{created.Submission.SubmissionID, 999999}, RequestedBy: 99,
		Reason: "must roll back",
	}); !errors.Is(err, ErrSubmissionNotFound) {
		t.Fatalf("BatchRejudgeSubmissions() error = %v", err)
	}
	assertTableCount(t, db, "submission_attempts", 3)
	assertTableCount(t, db, "submit_outbox_events", 3)
}

func TestSubmitModelContestResultAndOutboxClaim(t *testing.T) {
	repository, db, clock := newSubmitModelTestDatabase(t)
	ctx := context.Background()
	contestID := int64(88)
	created, err := repository.CreateSubmission(ctx, testCreateSubmissionInput("contest-outbox", &contestID))
	if err != nil {
		t.Fatal(err)
	}
	lease, err := repository.AcquireAttempt(ctx, AcquireAttemptInput{
		WorkerID: "worker", SubmissionID: created.Submission.SubmissionID, AttemptID: created.Submission.CurrentAttemptID,
		DeliveryID: "delivery", LeaseDuration: 5 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	clock.Advance(time.Minute)
	_, err = repository.ApplyJudgeResult(ctx, ApplyJudgeResultInput{
		WorkerID: "worker", SubmissionID: created.Submission.SubmissionID, AttemptID: created.Submission.CurrentAttemptID,
		LeaseToken: lease.LeaseToken, ReportID: "report", Verdict: VerdictWrongAnswer, Score: 0, MaxScore: 100,
		CaseResults: []CaseResult{{TestCaseID: 1, Ordinal: 0, Verdict: VerdictWrongAnswer}},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertTableCount(t, db, "submit_outbox_events", 2)
	var firstPending outboxRow
	if err := db.Order("id ASC").Take(&firstPending).Error; err != nil {
		t.Fatal(err)
	}
	if err := repository.MarkOutboxPublished(ctx, MarkOutboxPublishedInput{
		EventID: firstPending.EventID, WorkerID: "publisher-a",
	}); !errors.Is(err, ErrOutboxStateConflict) {
		t.Fatalf("unclaimed MarkOutboxPublished() error = %v", err)
	}

	claimed, err := repository.ClaimOutboxEvents(ctx, ClaimOutboxInput{
		WorkerID: "publisher-a", Limit: 10, LockDuration: time.Minute,
	})
	if err != nil || len(claimed) != 2 {
		t.Fatalf("ClaimOutboxEvents() = %+v err=%v", claimed, err)
	}
	blocked, err := repository.ClaimOutboxEvents(ctx, ClaimOutboxInput{
		WorkerID: "publisher-b", Limit: 10, LockDuration: time.Minute,
	})
	if err != nil || len(blocked) != 0 {
		t.Fatalf("second claim = %+v err=%v", blocked, err)
	}
	if err := repository.MarkOutboxPublished(ctx, MarkOutboxPublishedInput{
		EventID: claimed[1].EventID, WorkerID: "publisher-b",
	}); !errors.Is(err, ErrOutboxStateConflict) {
		t.Fatalf("wrong-owner MarkOutboxPublished() error = %v", err)
	}
	if err := repository.MarkOutboxFailed(ctx, MarkOutboxFailedInput{
		EventID: claimed[0].EventID, WorkerID: "publisher-a", ErrorCode: "mq.temporary",
		RetryAt: clock.Now().Add(4 * time.Minute), MaxAttempts: 3,
	}); err != nil {
		t.Fatalf("MarkOutboxFailed() error = %v", err)
	}
	if err := repository.MarkOutboxPublished(ctx, MarkOutboxPublishedInput{
		EventID: claimed[1].EventID, WorkerID: "publisher-a",
	}); err != nil {
		t.Fatalf("MarkOutboxPublished() error = %v", err)
	}
	clock.Advance(5 * time.Minute)
	reclaimed, err := repository.ClaimOutboxEvents(ctx, ClaimOutboxInput{
		WorkerID: "publisher-b", Limit: 10, LockDuration: time.Minute,
	})
	if err != nil || len(reclaimed) != 1 || reclaimed[0].EventID != claimed[0].EventID {
		t.Fatalf("retry claim = %+v err=%v", reclaimed, err)
	}
}

func TestSubmitModelListUsesStableOrderAndFilters(t *testing.T) {
	repository, _, clock := newSubmitModelTestDatabase(t)
	ctx := context.Background()
	for index := 0; index < 3; index++ {
		input := testCreateSubmissionInput(fmt.Sprintf("list-%d", index), nil)
		input.UserID = int64(index%2 + 1)
		input.SubmittedAt = clock.Now().Add(time.Duration(index) * time.Second)
		if _, err := repository.CreateSubmission(ctx, input); err != nil {
			t.Fatal(err)
		}
	}
	userID := int64(1)
	rows, total, err := repository.ListSubmissions(ctx, SubmissionListFilter{Page: 1, PageSize: 10, UserID: &userID})
	if err != nil || total != 2 || len(rows) != 2 {
		t.Fatalf("ListSubmissions() = %+v total=%d err=%v", rows, total, err)
	}
	if !rows[0].SubmittedAt.After(rows[1].SubmittedAt) {
		t.Fatalf("list is not descending: %+v", rows)
	}
	page, total, err := repository.ListSubmissions(ctx, SubmissionListFilter{Page: 2, PageSize: 2})
	if err != nil || total != 3 || len(page) != 1 {
		t.Fatalf("paged list = %+v total=%d err=%v", page, total, err)
	}
}

type submitModelTestClock struct {
	mu sync.Mutex
	at time.Time
}

func (clock *submitModelTestClock) Now() time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	return clock.at
}

func (clock *submitModelTestClock) Advance(delta time.Duration) {
	clock.mu.Lock()
	clock.at = clock.at.Add(delta)
	clock.mu.Unlock()
}

func testCreateSubmissionInput(requestID string, contestID *int64) CreateSubmissionInput {
	return CreateSubmissionInput{
		UserID: 1, ProblemID: 2, ContestID: cloneInt64Pointer(contestID), ProblemRevision: 3,
		Language: "go1.26", SourceCode: "package main\nfunc main() {}", ClientRequestID: requestID,
		SubmittedAt: time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC),
	}
}

func assertTableCount(t *testing.T, db *gorm.DB, table string, want int64) {
	t.Helper()
	var count int64
	if err := db.Table(table).Count(&count).Error; err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if count != want {
		t.Fatalf("%s count = %d, want %d", table, count, want)
	}
}

func newSubmitModelTestDatabase(t *testing.T) (*defaultSubmitModel, *gorm.DB, *submitModelTestClock) {
	t.Helper()
	dsn := os.Getenv("SUBMIT_MODEL_TEST_DSN")
	if dsn == "" {
		t.Skip("set SUBMIT_MODEL_TEST_DSN to run isolated MySQL integration tests")
	}
	config, err := mysqlDriver.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("parse SUBMIT_MODEL_TEST_DSN: %v", err)
	}
	config.DBName = ""
	config.ParseTime = true
	config.Loc = time.UTC
	admin, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatalf("open MySQL admin connection: %v", err)
	}
	databaseName := fmt.Sprintf("submit_model_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec("CREATE DATABASE `" + databaseName + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"); err != nil {
		_ = admin.Close()
		t.Fatalf("create isolated database: %v", err)
	}
	var testSQLDB *sql.DB
	t.Cleanup(func() {
		if testSQLDB != nil {
			_ = testSQLDB.Close()
		}
		_, _ = admin.Exec("DROP DATABASE IF EXISTS `" + databaseName + "`")
		_ = admin.Close()
	})
	config.DBName = databaseName
	testSQLDB, err = sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatalf("open isolated database: %v", err)
	}
	applySubmitMigration(t, testSQLDB, filepath.Join("..", "migrations", "001_create_submit.sql"))
	// The table migration must be safe to replay after a partially completed
	// MySQL DDL run. The separately tracked FK step has its own baseline check.
	applySubmitMigration(t, testSQLDB, filepath.Join("..", "migrations", "001_create_submit.sql"))
	applySubmitMigration(t, testSQLDB, filepath.Join("..", "migrations", "002_add_current_attempt_foreign_key.sql"))
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: testSQLDB}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open GORM test database: %v", err)
	}
	clock := &submitModelTestClock{at: time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)}
	return &defaultSubmitModel{db: db, now: clock.Now}, db, clock
}

func applySubmitMigration(t *testing.T, db *sql.DB, fileName string) {
	t.Helper()
	contents, err := os.ReadFile(fileName)
	if err != nil {
		t.Fatalf("read migration %s: %v", fileName, err)
	}
	lines := strings.Split(string(contents), "\n")
	filtered := lines[:0]
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		filtered = append(filtered, line)
	}
	for _, statement := range strings.Split(strings.Join(filtered, "\n"), ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("apply migration %s: %v\nSQL: %s", fileName, err, statement)
		}
	}
}
