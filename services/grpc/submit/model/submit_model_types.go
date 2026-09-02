package model

import (
	"encoding/json"
	"time"
)

type SubmissionSummary struct {
	SubmissionID     int64      `json:"submission_id"`
	UserID           int64      `json:"user_id"`
	ProblemID        int64      `json:"problem_id"`
	ContestID        *int64     `json:"contest_id,omitempty"`
	ProblemRevision  int64      `json:"problem_revision"`
	Language         string     `json:"language"`
	Status           int64      `json:"status"`
	Verdict          *int64     `json:"verdict,omitempty"`
	Score            *int64     `json:"score,omitempty"`
	MaxScore         *int64     `json:"max_score,omitempty"`
	AttemptCount     int32      `json:"attempt_count"`
	CurrentAttemptID int64      `json:"current_attempt_id"`
	SubmittedAt      time.Time  `json:"submitted_at"`
	JudgingStartedAt *time.Time `json:"judging_started_at,omitempty"`
	FinishedAt       *time.Time `json:"finished_at,omitempty"`
	Version          int64      `json:"version"`
}

type SubmissionAggregate struct {
	SubmissionSummary
	SourceCode     string       `json:"-"`
	CurrentAttempt *Attempt     `json:"-"`
	CaseResults    []CaseResult `json:"case_results,omitempty"`
}

type Attempt struct {
	AttemptID          int64      `json:"attempt_id"`
	SubmissionID       int64      `json:"submission_id"`
	AttemptNo          int32      `json:"attempt_no"`
	Kind               int64      `json:"kind"`
	Status             int64      `json:"status"`
	RequestedBy        *int64     `json:"requested_by,omitempty"`
	Reason             string     `json:"reason,omitempty"`
	WorkerID           string     `json:"worker_id,omitempty"`
	DeliveryID         string     `json:"delivery_id,omitempty"`
	LeaseExpiresAt     *time.Time `json:"lease_expires_at,omitempty"`
	LastHeartbeatAt    *time.Time `json:"last_heartbeat_at,omitempty"`
	CompletedTestCases int32      `json:"completed_test_cases"`
	ReportID           *string    `json:"-"`
	Verdict            *int64     `json:"verdict,omitempty"`
	Score              *int64     `json:"score,omitempty"`
	MaxScore           *int64     `json:"max_score,omitempty"`
	CompileOutput      string     `json:"-"`
	EnqueuedAt         time.Time  `json:"enqueued_at"`
	StartedAt          *time.Time `json:"started_at,omitempty"`
	FinishedAt         *time.Time `json:"finished_at,omitempty"`
	SupersededAt       *time.Time `json:"superseded_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type CaseResult struct {
	TestCaseID   int64  `json:"test_case_id"`
	Ordinal      int32  `json:"ordinal"`
	GroupName    string `json:"group_name"`
	Verdict      int64  `json:"verdict"`
	Score        int64  `json:"score"`
	TimeUsedMS   int64  `json:"time_used_ms"`
	MemoryUsedKB int64  `json:"memory_used_kb"`
	JudgeMessage string `json:"-"`
}

type CreateSubmissionInput struct {
	UserID          int64
	ProblemID       int64
	ContestID       *int64
	ProblemRevision int64
	Language        string
	SourceCode      string
	ClientRequestID string
	SubmittedAt     time.Time
}

type CreateSubmissionResult struct {
	Submission   *SubmissionAggregate
	Deduplicated bool
}

type SubmissionListFilter struct {
	Page      int
	PageSize  int
	UserID    *int64
	ProblemID *int64
	ContestID *int64
	Status    *int64
	Verdict   *int64
	Language  string
}

type RejudgeSubmissionInput struct {
	SubmissionID int64
	RequestedBy  int64
	Reason       string
}

type BatchRejudgeInput struct {
	SubmissionIDs []int64
	RequestedBy   int64
	Reason        string
}

type RejudgeResult struct {
	SubmissionID int64
	AttemptID    int64
	AttemptNo    int32
	Submission   *SubmissionAggregate
}

type AcquireAttemptInput struct {
	WorkerID      string
	SubmissionID  int64
	AttemptID     int64
	DeliveryID    string
	LeaseDuration time.Duration
}

type AcquireAttemptResult struct {
	Acquired        bool
	DenyReason      int64
	SubmissionID    int64
	AttemptID       int64
	ProblemID       int64
	ProblemRevision int64
	Language        string
	SourceCode      string `json:"-"`
	LeaseToken      string `json:"-"`
	LeaseExpiresAt  *time.Time
}

type HeartbeatAttemptInput struct {
	WorkerID           string
	SubmissionID       int64
	AttemptID          int64
	LeaseToken         string
	CompletedTestCases int32
	TotalTestCases     int32
	LeaseDuration      time.Duration
}

type HeartbeatResult struct {
	LeaseExpiresAt     time.Time
	CompletedTestCases int32
}

type ApplyJudgeResultInput struct {
	WorkerID      string
	SubmissionID  int64
	AttemptID     int64
	LeaseToken    string
	ReportID      string
	Verdict       int64
	Score         int64
	MaxScore      int64
	CompileOutput string
	CaseResults   []CaseResult
}

type ApplyJudgeResultResult struct {
	Disposition int64
	Submission  *SubmissionAggregate
}

type ExpiredLease struct {
	SubmissionID   int64
	AttemptID      int64
	AttemptNo      int32
	LeaseExpiresAt time.Time
}

type SupersedeAttemptInput struct {
	SubmissionID int64
	AttemptID    int64
	Reason       string
}

type OutboxEvent struct {
	ID              int64
	EventID         string
	EventType       int64
	AggregateID     int64
	AttemptID       int64
	EventKey        string
	Payload         json.RawMessage `json:"-"`
	Status          int64
	AvailableAt     time.Time
	PublishAttempts int32
	LockedBy        *string
	LockedUntil     *time.Time
	LastError       string
	CreatedAt       time.Time
	PublishedAt     *time.Time
}

type ClaimOutboxInput struct {
	WorkerID     string
	Limit        int
	LockDuration time.Duration
}

type MarkOutboxPublishedInput struct {
	EventID  string
	WorkerID string
}

type MarkOutboxFailedInput struct {
	EventID     string
	WorkerID    string
	ErrorCode   string
	RetryAt     time.Time
	MaxAttempts int32
}
