package model

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

var (
	ErrSubmissionNotFound      = errors.New("submission not found")
	ErrInvalidSubmissionInput  = errors.New("invalid submission input")
	ErrIdempotencyConflict     = errors.New("idempotency conflict")
	ErrSubmissionStateConflict = errors.New("submission state conflict")
	ErrAttemptNotFound         = errors.New("submission attempt not found")
	ErrAttemptAlreadyFinished  = errors.New("submission attempt already finished")
	ErrStaleAttempt            = errors.New("stale submission attempt")
	ErrLeaseHeld               = errors.New("submission attempt lease is held")
	ErrLeaseMismatch           = errors.New("submission attempt lease does not match")
	ErrLeaseExpired            = errors.New("submission attempt lease expired")
	ErrReportConflict          = errors.New("judge report conflict")
	ErrOutboxEventNotFound     = errors.New("submit outbox event not found")
	ErrOutboxStateConflict     = errors.New("submit outbox state conflict")
	ErrDatabaseConflict        = errors.New("submit database conflict")
)

const (
	SubmissionStatusQueued   int64 = 1
	SubmissionStatusJudging  int64 = 2
	SubmissionStatusFinished int64 = 3

	VerdictAccepted            int64 = 1
	VerdictPartialAccepted     int64 = 2
	VerdictWrongAnswer         int64 = 3
	VerdictTimeLimitExceeded   int64 = 4
	VerdictMemoryLimitExceeded int64 = 5
	VerdictRuntimeError        int64 = 6
	VerdictCompileError        int64 = 7
	VerdictOutputLimitExceeded int64 = 8
	VerdictSystemError         int64 = 9

	AttemptKindInitial    int64 = 1
	AttemptKindLeaseRetry int64 = 2
	AttemptKindRejudge    int64 = 3

	AttemptStatusQueued     int64 = 1
	AttemptStatusJudging    int64 = 2
	AttemptStatusFinished   int64 = 3
	AttemptStatusSuperseded int64 = 4

	OutboxEventJudgeTask        int64 = 1
	OutboxEventSubmissionJudged int64 = 2
	OutboxStatusPending         int64 = 1
	OutboxStatusPublished       int64 = 2
	OutboxStatusDead            int64 = 3

	AcquireDenyNone                int64 = 0
	AcquireDenyAlreadyFinished     int64 = 1
	AcquireDenyStaleAttempt        int64 = 2
	AcquireDenyLeasedByOtherWorker int64 = 3

	ReportDispositionApplied   int64 = 1
	ReportDispositionDuplicate int64 = 2
)

// SubmitModel is the only persistence boundary exposed to Submit RPC logic.
// Its methods preserve submission, attempt, result and outbox invariants.
type SubmitModel interface {
	CreateSubmission(ctx context.Context, input CreateSubmissionInput) (*CreateSubmissionResult, error)
	GetSubmissionByClientRequestID(ctx context.Context, userID int64, clientRequestID string) (*SubmissionAggregate, error)
	GetSubmission(ctx context.Context, submissionID int64) (*SubmissionAggregate, error)
	ListSubmissions(ctx context.Context, filter SubmissionListFilter) ([]SubmissionSummary, int64, error)

	RejudgeSubmission(ctx context.Context, input RejudgeSubmissionInput) (*RejudgeResult, error)
	BatchRejudgeSubmissions(ctx context.Context, input BatchRejudgeInput) ([]RejudgeResult, error)

	AcquireAttempt(ctx context.Context, input AcquireAttemptInput) (*AcquireAttemptResult, error)
	HeartbeatAttempt(ctx context.Context, input HeartbeatAttemptInput) (*HeartbeatResult, error)
	ApplyJudgeResult(ctx context.Context, input ApplyJudgeResultInput) (*ApplyJudgeResultResult, error)

	ListExpiredLeases(ctx context.Context, before time.Time, limit int) ([]ExpiredLease, error)
	SupersedeExpiredAttempt(ctx context.Context, input SupersedeAttemptInput) (*Attempt, error)

	ClaimOutboxEvents(ctx context.Context, input ClaimOutboxInput) ([]OutboxEvent, error)
	MarkOutboxPublished(ctx context.Context, input MarkOutboxPublishedInput) error
	MarkOutboxFailed(ctx context.Context, input MarkOutboxFailedInput) error
}

type defaultSubmitModel struct {
	db  *gorm.DB
	now func() time.Time
}

type SubmitModelOption func(*defaultSubmitModel)

// WithClock provides the service clock for deterministic tests.
func WithClock(now func() time.Time) SubmitModelOption {
	return func(model *defaultSubmitModel) {
		if now != nil {
			model.now = now
		}
	}
}

func NewSubmitModel(db *gorm.DB, options ...SubmitModelOption) SubmitModel {
	result := &defaultSubmitModel{
		db: db,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
	for _, option := range options {
		if option != nil {
			option(result)
		}
	}
	return result
}
