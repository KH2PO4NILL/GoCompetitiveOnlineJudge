package model

import "time"

type submissionRow struct {
	ID                 int64      `gorm:"column:id;primaryKey"`
	UserID             int64      `gorm:"column:user_id"`
	ProblemID          int64      `gorm:"column:problem_id"`
	ContestID          *int64     `gorm:"column:contest_id"`
	ProblemRevision    int64      `gorm:"column:problem_revision"`
	Language           string     `gorm:"column:language"`
	SourceCode         string     `gorm:"column:source_code" json:"-"`
	ClientRequestID    string     `gorm:"column:client_request_id"`
	RequestFingerprint []byte     `gorm:"column:request_fingerprint" json:"-"`
	Status             int64      `gorm:"column:status"`
	CurrentAttemptID   *int64     `gorm:"column:current_attempt_id"`
	AttemptCount       int32      `gorm:"column:attempt_count"`
	Verdict            *int64     `gorm:"column:verdict"`
	Score              *int64     `gorm:"column:score"`
	MaxScore           *int64     `gorm:"column:max_score"`
	JudgingStartedAt   *time.Time `gorm:"column:judging_started_at"`
	FinishedAt         *time.Time `gorm:"column:finished_at"`
	SubmittedAt        time.Time  `gorm:"column:submitted_at"`
	UpdatedAt          time.Time  `gorm:"column:updated_at"`
	Version            int64      `gorm:"column:version"`
}

func (submissionRow) TableName() string { return "submissions" }

type attemptRow struct {
	ID                 int64      `gorm:"column:id;primaryKey"`
	SubmissionID       int64      `gorm:"column:submission_id"`
	AttemptNo          int32      `gorm:"column:attempt_no"`
	Kind               int64      `gorm:"column:kind"`
	Status             int64      `gorm:"column:status"`
	RequestedBy        *int64     `gorm:"column:requested_by"`
	Reason             string     `gorm:"column:reason"`
	WorkerID           string     `gorm:"column:worker_id"`
	DeliveryID         string     `gorm:"column:delivery_id"`
	LeaseTokenHash     []byte     `gorm:"column:lease_token_hash" json:"-"`
	LeaseExpiresAt     *time.Time `gorm:"column:lease_expires_at"`
	LastHeartbeatAt    *time.Time `gorm:"column:last_heartbeat_at"`
	CompletedTestCases int32      `gorm:"column:completed_test_cases"`
	ReportID           *string    `gorm:"column:report_id" json:"-"`
	ReportFingerprint  []byte     `gorm:"column:report_fingerprint" json:"-"`
	Verdict            *int64     `gorm:"column:verdict"`
	Score              *int64     `gorm:"column:score"`
	MaxScore           *int64     `gorm:"column:max_score"`
	CompileOutput      string     `gorm:"column:compile_output" json:"-"`
	EnqueuedAt         time.Time  `gorm:"column:enqueued_at"`
	StartedAt          *time.Time `gorm:"column:started_at"`
	FinishedAt         *time.Time `gorm:"column:finished_at"`
	SupersededAt       *time.Time `gorm:"column:superseded_at"`
	CreatedAt          time.Time  `gorm:"column:created_at"`
	UpdatedAt          time.Time  `gorm:"column:updated_at"`
}

func (attemptRow) TableName() string { return "submission_attempts" }

type caseResultRow struct {
	ID           int64     `gorm:"column:id;primaryKey"`
	AttemptID    int64     `gorm:"column:attempt_id"`
	TestCaseID   int64     `gorm:"column:test_case_id"`
	Ordinal      int32     `gorm:"column:ordinal"`
	GroupName    string    `gorm:"column:group_name"`
	Verdict      int64     `gorm:"column:verdict"`
	Score        int64     `gorm:"column:score"`
	TimeUsedMS   int64     `gorm:"column:time_used_ms"`
	MemoryUsedKB int64     `gorm:"column:memory_used_kb"`
	JudgeMessage string    `gorm:"column:judge_message" json:"-"`
	CreatedAt    time.Time `gorm:"column:created_at"`
}

func (caseResultRow) TableName() string { return "submission_case_results" }

type outboxRow struct {
	ID              int64      `gorm:"column:id;primaryKey"`
	EventID         string     `gorm:"column:event_id"`
	EventType       int64      `gorm:"column:event_type"`
	AggregateID     int64      `gorm:"column:aggregate_id"`
	AttemptID       int64      `gorm:"column:attempt_id"`
	EventKey        string     `gorm:"column:event_key"`
	Payload         []byte     `gorm:"column:payload" json:"-"`
	Status          int64      `gorm:"column:status"`
	AvailableAt     time.Time  `gorm:"column:available_at"`
	PublishAttempts int32      `gorm:"column:publish_attempts"`
	LockedBy        *string    `gorm:"column:locked_by"`
	LockedUntil     *time.Time `gorm:"column:locked_until"`
	LastError       string     `gorm:"column:last_error"`
	CreatedAt       time.Time  `gorm:"column:created_at"`
	PublishedAt     *time.Time `gorm:"column:published_at"`
}

func (outboxRow) TableName() string { return "submit_outbox_events" }
