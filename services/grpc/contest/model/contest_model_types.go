package model

import "time"

// EditableContest contains values controlled by a contest creator. InviteCode
// is accepted on writes only; persisted aggregates expose InviteCodeConfigured
// instead of returning the secret or its hash.
type EditableContest struct {
	Title                     string
	Description               string
	Mode                      int64
	StartAt                   time.Time
	EndAt                     time.Time
	RegistrationStartAt       *time.Time
	RegistrationEndAt         *time.Time
	RegistrationPolicy        int64
	Visibility                int64
	ScoreboardVisibility      int64
	ScoreboardFreezeAt        *time.Time
	ScoreboardFreezeBeforeEnd *time.Duration
	InviteCode                *string
}

type CreateContestInput struct {
	CreatorID int64
	Editable  EditableContest
}

// ContestPatch uses pointers to distinguish an omitted field from an explicit
// zero/empty/null value. Pointer-to-pointer fields support clearing nullable
// values.
type ContestPatch struct {
	Title                     *string
	Description               *string
	Mode                      *int64
	StartAt                   *time.Time
	EndAt                     *time.Time
	RegistrationStartAt       **time.Time
	RegistrationEndAt         **time.Time
	RegistrationPolicy        *int64
	Visibility                *int64
	ScoreboardVisibility      *int64
	ScoreboardFreezeAt        **time.Time
	ScoreboardFreezeBeforeEnd **time.Duration
	InviteCode                **string
}

type UpdateContestInput struct {
	ContestID        int64
	ExpectedRevision int64
	Patch            ContestPatch
}

type ContestAggregate struct {
	ContestID            int64
	CreatorID            int64
	Editable             EditableContest
	InviteCodeConfigured bool
	Status               int64
	EffectiveStatus      int64
	Revision             int64
	ParticipantCount     int64
	CancelReason         string
	PublishedAt          *time.Time
	CancelledAt          *time.Time
	ArchivedAt           *time.Time
	DeletedAt            *time.Time
	ScoreboardFrozen     bool
	ScoreboardFrozenAt   *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// ContestDetailRecord is safe to pass to user-facing DTO conversion. It never
// contains invite-code material or soft-delete restoration metadata.
type ContestDetailRecord struct {
	ContestID            int64
	Title                string
	Description          string
	Mode                 int64
	Status               int64
	StartAt              time.Time
	EndAt                time.Time
	RegistrationStartAt  *time.Time
	RegistrationEndAt    *time.Time
	RegistrationPolicy   int64
	Visibility           int64
	ScoreboardVisibility int64
	RegistrationOpen     bool
	ScoreboardFrozen     bool
	ParticipantCount     int64
}

type ContestSummary struct {
	ContestID          int64
	Title              string
	Mode               int64
	Status             int64
	StartAt            time.Time
	EndAt              time.Time
	RegistrationPolicy int64
	Visibility         int64
	RegistrationOpen   bool
	ParticipantCount   int64
}

type ContestListFilter struct {
	Page      int
	PageSize  int
	Keyword   string
	Status    *int64
	Mode      *int64
	CreatorID *int64
	At        time.Time
}

type ContestProblem struct {
	ContestID    int64
	ProblemID    int64
	Label        string
	DisplayOrder int64
	MaxScore     int64
}

type SetContestProblemsInput struct {
	ContestID        int64
	ExpectedRevision int64
	Problems         []ContestProblem
}

type ContestRegistration struct {
	RegistrationID  int64
	ContestID       int64
	UserID          int64
	Status          int64
	ApplicationNote string
	ReviewerID      *int64
	ReviewNote      string
	Revision        int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
	ReviewedAt      *time.Time
	CancelledAt     *time.Time
}

type ContestParticipant struct {
	ContestID      int64
	UserID         int64
	RegistrationID int64
	Status         int64
	QualifiedAt    time.Time
	UpdatedAt      time.Time
}

type RegisterContestInput struct {
	ContestID       int64
	UserID          int64
	InviteCode      string
	ApplicationNote string
	At              time.Time
}

type ReviewRegistrationInput struct {
	ContestID        int64
	RegistrationID   int64
	ReviewerID       int64
	Decision         int64
	ReviewNote       string
	ExpectedRevision int64
}

type SetParticipantStatusInput struct {
	ContestID int64
	UserID    int64
	Status    int64
}

type MyContestStatus struct {
	ContestID    int64
	Registration *ContestRegistration
	Participant  *ContestParticipant
}

type RegistrationListFilter struct {
	ContestID int64
	Page      int
	PageSize  int
	Status    *int64
}

type ParticipantListFilter struct {
	ContestID int64
	Page      int
	PageSize  int
	Status    *int64
}

// ContestAccessSnapshot intentionally contains only facts needed by access and
// submission policy. It is populated with one joined query.
type ContestAccessSnapshot struct {
	ContestID          int64
	Mode               int64
	LifecycleStatus    int64
	EffectiveStatus    int64
	Visibility         int64
	StartAt            time.Time
	EndAt              time.Time
	ProblemAssigned    bool
	RegistrationStatus *int64
	ParticipantStatus  *int64
}

type IcpcScore struct {
	SolvedCount  int64
	TotalPenalty time.Duration
	LastSolvedAt *time.Time
}

type PointsScore struct {
	TotalScore                int64
	LastEffectiveSubmissionAt *time.Time
}

type IcpcProblemResult struct {
	Solved        bool
	WrongAttempts int64
	Penalty       time.Duration
	AcceptedAt    *time.Time
	FirstToSolve  bool
}

type PointsProblemResult struct {
	Score                     int64
	MaxScore                  int64
	SubmissionCount           int64
	LastEffectiveSubmissionAt *time.Time
}

type ScoreboardProblemResult struct {
	ProblemID int64
	Label     string
	ICPC      *IcpcProblemResult
	Points    *PointsProblemResult
}

type ScoreboardRow struct {
	Rank           int64
	Ordinal        int64
	UserID         int64
	DisplayName    string
	ICPC           *IcpcScore
	Points         *PointsScore
	ProblemResults []ScoreboardProblemResult
}

type ReplaceScoreboardInput struct {
	ContestID     int64
	Mode          int64
	SourceVersion int64
	GeneratedAt   time.Time
	Rows          []ScoreboardRow
}

type ScoreboardFilter struct {
	ContestID int64
	View      int64
	Page      int
	PageSize  int
}

type ScoreboardFreezeCandidate struct {
	ContestID int64
	Revision  int64
	FreezeAt  time.Time
}

type ScoreboardPage struct {
	ContestID     int64
	Mode          int64
	GeneratedAt   time.Time
	Frozen        bool
	View          int64
	SourceVersion int64
	Total         int64
	Rows          []ScoreboardRow
}

type contestRow struct {
	ID                          int64      `gorm:"column:id;primaryKey"`
	CreatorID                   int64      `gorm:"column:creator_id"`
	Title                       string     `gorm:"column:title"`
	Description                 string     `gorm:"column:description"`
	Mode                        int64      `gorm:"column:mode"`
	StartAt                     time.Time  `gorm:"column:start_at"`
	EndAt                       time.Time  `gorm:"column:end_at"`
	RegistrationStartAt         *time.Time `gorm:"column:registration_start_at"`
	RegistrationEndAt           *time.Time `gorm:"column:registration_end_at"`
	RegistrationPolicy          int64      `gorm:"column:registration_policy"`
	Visibility                  int64      `gorm:"column:visibility"`
	ScoreboardVisibility        int64      `gorm:"column:scoreboard_visibility"`
	ScoreboardFreezeAt          *time.Time `gorm:"column:scoreboard_freeze_at"`
	ScoreboardFreezeBeforeEndMS *int64     `gorm:"column:scoreboard_freeze_before_end_ms"`
	InviteCodeHash              *string    `gorm:"column:invite_code_hash"`
	Status                      int64      `gorm:"column:status"`
	Revision                    int64      `gorm:"column:revision"`
	ParticipantCount            int64      `gorm:"column:participant_count"`
	CancelReason                string     `gorm:"column:cancel_reason"`
	PublishedAt                 *time.Time `gorm:"column:published_at"`
	CancelledAt                 *time.Time `gorm:"column:cancelled_at"`
	ArchivedAt                  *time.Time `gorm:"column:archived_at"`
	DeletedAt                   *time.Time `gorm:"column:deleted_at"`
	DeletedFromStatus           *int64     `gorm:"column:deleted_from_status"`
	ScoreboardFrozen            bool       `gorm:"column:scoreboard_frozen"`
	ScoreboardFrozenAt          *time.Time `gorm:"column:scoreboard_frozen_at"`
	CreatedAt                   time.Time  `gorm:"column:created_at"`
	UpdatedAt                   time.Time  `gorm:"column:updated_at"`
}

func (contestRow) TableName() string { return "contests" }

type contestProblemRow struct {
	ContestID    int64     `gorm:"column:contest_id;primaryKey"`
	ProblemID    int64     `gorm:"column:problem_id;primaryKey"`
	Label        string    `gorm:"column:label"`
	DisplayOrder int64     `gorm:"column:display_order"`
	MaxScore     int64     `gorm:"column:max_score"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

func (contestProblemRow) TableName() string { return "contest_problems" }

type contestRegistrationRow struct {
	ID              int64      `gorm:"column:id;primaryKey"`
	ContestID       int64      `gorm:"column:contest_id"`
	UserID          int64      `gorm:"column:user_id"`
	Status          int64      `gorm:"column:status"`
	ApplicationNote string     `gorm:"column:application_note"`
	ReviewerID      *int64     `gorm:"column:reviewer_id"`
	ReviewNote      string     `gorm:"column:review_note"`
	Revision        int64      `gorm:"column:revision"`
	CreatedAt       time.Time  `gorm:"column:created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at"`
	ReviewedAt      *time.Time `gorm:"column:reviewed_at"`
	CancelledAt     *time.Time `gorm:"column:cancelled_at"`
}

func (contestRegistrationRow) TableName() string { return "contest_registrations" }

type contestParticipantRow struct {
	ContestID      int64     `gorm:"column:contest_id;primaryKey"`
	UserID         int64     `gorm:"column:user_id;primaryKey"`
	RegistrationID int64     `gorm:"column:registration_id"`
	Status         int64     `gorm:"column:status"`
	QualifiedAt    time.Time `gorm:"column:qualified_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at"`
}

func (contestParticipantRow) TableName() string { return "contest_participants" }

type scoreboardStateRow struct {
	ContestID              int64      `gorm:"column:contest_id;primaryKey"`
	Mode                   int64      `gorm:"column:mode"`
	SourceVersion          int64      `gorm:"column:source_version"`
	GeneratedAt            *time.Time `gorm:"column:generated_at"`
	ActiveFrozenSnapshotID *int64     `gorm:"column:active_frozen_snapshot_id"`
	UpdatedAt              time.Time  `gorm:"column:updated_at"`
}

func (scoreboardStateRow) TableName() string { return "contest_scoreboard_state" }

type scoreboardRowRecord struct {
	ContestID                 int64      `gorm:"column:contest_id;primaryKey"`
	UserID                    int64      `gorm:"column:user_id;primaryKey"`
	DisplayName               string     `gorm:"column:display_name"`
	Mode                      int64      `gorm:"column:mode"`
	Rank                      int64      `gorm:"column:rank_value"`
	Ordinal                   int64      `gorm:"column:ordinal"`
	SolvedCount               *int64     `gorm:"column:solved_count"`
	TotalPenaltyMS            *int64     `gorm:"column:total_penalty_ms"`
	LastSolvedAt              *time.Time `gorm:"column:last_solved_at"`
	TotalScore                *int64     `gorm:"column:total_score"`
	LastEffectiveSubmissionAt *time.Time `gorm:"column:last_effective_submission_at"`
}

func (scoreboardRowRecord) TableName() string { return "contest_scoreboard_rows" }

type scoreboardProblemResultRow struct {
	ContestID                 int64      `gorm:"column:contest_id;primaryKey"`
	UserID                    int64      `gorm:"column:user_id;primaryKey"`
	ProblemID                 int64      `gorm:"column:problem_id;primaryKey"`
	Label                     string     `gorm:"column:label"`
	Mode                      int64      `gorm:"column:mode"`
	Solved                    *bool      `gorm:"column:solved"`
	WrongAttempts             *int64     `gorm:"column:wrong_attempts"`
	PenaltyMS                 *int64     `gorm:"column:penalty_ms"`
	AcceptedAt                *time.Time `gorm:"column:accepted_at"`
	FirstToSolve              *bool      `gorm:"column:first_to_solve"`
	Score                     *int64     `gorm:"column:score"`
	MaxScore                  *int64     `gorm:"column:max_score"`
	SubmissionCount           *int64     `gorm:"column:submission_count"`
	LastEffectiveSubmissionAt *time.Time `gorm:"column:last_effective_submission_at"`
}

func (scoreboardProblemResultRow) TableName() string {
	return "contest_scoreboard_problem_results"
}

type scoreboardSnapshotRow struct {
	ID            int64     `gorm:"column:id;primaryKey"`
	ContestID     int64     `gorm:"column:contest_id"`
	SourceVersion int64     `gorm:"column:source_version"`
	Mode          int64     `gorm:"column:mode"`
	GeneratedAt   time.Time `gorm:"column:generated_at"`
	FrozenAt      time.Time `gorm:"column:frozen_at"`
	CreatedAt     time.Time `gorm:"column:created_at"`
}

func (scoreboardSnapshotRow) TableName() string { return "contest_scoreboard_snapshots" }

type scoreboardSnapshotRowRecord struct {
	SnapshotID                int64      `gorm:"column:snapshot_id;primaryKey"`
	UserID                    int64      `gorm:"column:user_id;primaryKey"`
	DisplayName               string     `gorm:"column:display_name"`
	Mode                      int64      `gorm:"column:mode"`
	Rank                      int64      `gorm:"column:rank_value"`
	Ordinal                   int64      `gorm:"column:ordinal"`
	SolvedCount               *int64     `gorm:"column:solved_count"`
	TotalPenaltyMS            *int64     `gorm:"column:total_penalty_ms"`
	LastSolvedAt              *time.Time `gorm:"column:last_solved_at"`
	TotalScore                *int64     `gorm:"column:total_score"`
	LastEffectiveSubmissionAt *time.Time `gorm:"column:last_effective_submission_at"`
}

func (scoreboardSnapshotRowRecord) TableName() string {
	return "contest_scoreboard_snapshot_rows"
}

type scoreboardSnapshotProblemResultRow struct {
	SnapshotID                int64      `gorm:"column:snapshot_id;primaryKey"`
	UserID                    int64      `gorm:"column:user_id;primaryKey"`
	ProblemID                 int64      `gorm:"column:problem_id;primaryKey"`
	Label                     string     `gorm:"column:label"`
	Mode                      int64      `gorm:"column:mode"`
	Solved                    *bool      `gorm:"column:solved"`
	WrongAttempts             *int64     `gorm:"column:wrong_attempts"`
	PenaltyMS                 *int64     `gorm:"column:penalty_ms"`
	AcceptedAt                *time.Time `gorm:"column:accepted_at"`
	FirstToSolve              *bool      `gorm:"column:first_to_solve"`
	Score                     *int64     `gorm:"column:score"`
	MaxScore                  *int64     `gorm:"column:max_score"`
	SubmissionCount           *int64     `gorm:"column:submission_count"`
	LastEffectiveSubmissionAt *time.Time `gorm:"column:last_effective_submission_at"`
}

func (scoreboardSnapshotProblemResultRow) TableName() string {
	return "contest_scoreboard_snapshot_problem_results"
}
