package model

import (
	"database/sql"
	"time"
)

type ProblemSample struct {
	Input       string `json:"input"`
	Output      string `json:"output"`
	Explanation string `json:"explanation,omitempty"`
}

type EditableProblem struct {
	Title             string
	Description       string
	InputDescription  string
	OutputDescription string
	Samples           []ProblemSample
	Hint              string
	Difficulty        int64
	Tags              []string
	Source            string
	TimeLimitMS       int64
	MemoryLimitMB     int64
	AllowedLanguages  []string
	CheckerType       int64
	CheckerFileKey    *string
	InteractorFileKey *string
	Visibility        int64
}

type CreateProblemInput struct {
	AuthorID int64
	Editable EditableProblem
}

// ProblemPatch uses pointers so a caller can distinguish "not supplied" from
// an explicit zero/empty value. Validation still rejects values outside the
// domain; empty strings and empty slices remain valid for nullable content.
type ProblemPatch struct {
	Title             *string
	Description       *string
	InputDescription  *string
	OutputDescription *string
	Samples           *[]ProblemSample
	Hint              *string
	Difficulty        *int64
	Tags              *[]string
	Source            *string
	TimeLimitMS       *int64
	MemoryLimitMB     *int64
	AllowedLanguages  *[]string
	CheckerType       *int64
	CheckerFileKey    **string
	InteractorFileKey **string
	Visibility        *int64
}

type UpdateProblemInput struct {
	ProblemID        int64
	ExpectedRevision int64
	ActorID          int64
	Patch            ProblemPatch
}

type ProblemAggregate struct {
	ProblemID                int64
	AuthorID                 int64
	Editable                 EditableProblem
	CurrentTestDataVersionID *int64
	LatestPublishedRevision  *int64
	Status                   int64
	Revision                 int64
	PublishedAt              *time.Time
	CreatedAt                time.Time
	UpdatedAt                time.Time
	DeletedAt                *time.Time
}

type ProblemRevision struct {
	ID                int64
	ProblemID         int64
	Revision          int64
	Editable          EditableProblem
	TestDataVersionID *int64
	Status            int64
	CreatedBy         int64
	CreatedAt         time.Time
}

type ProblemSummary struct {
	ProblemID  int64
	Title      string
	Difficulty int64
	Tags       []string
	Status     int64
	Visibility int64
	AuthorID   int64
	UpdatedAt  time.Time
	Revision   int64
}

type ProblemListFilter struct {
	Page       int
	PageSize   int
	Keyword    string
	Tags       []string
	Difficulty *int64
	Status     *int64
	Visibility *int64
	AuthorID   *int64
}

type TestCaseInput struct {
	Name      string
	Score     int64
	Ordinal   int64
	Enabled   bool
	GroupName string
}

type TestCaseRecord struct {
	ID int64
	TestCaseInput
}

type JudgeProblemSpec struct {
	ProblemID          int64
	Revision           int64
	TimeLimitMS        int64
	MemoryLimitMB      int64
	AllowedLanguages   []string
	CheckerType        int64
	CheckerFileKey     *string
	InteractorFileKey  *string
	TestDataVersion    string
	TestDataPackageKey string
	TestDataChecksum   string
	TestCases          []TestCaseRecord
}

type CreateTestDataUploadInput struct {
	ProblemID     int64
	FileName      string
	ContentLength int64
	SHA256        string
	TTL           time.Duration
}

type TestDataUpload struct {
	UploadID      string
	ProblemID     int64
	ObjectKey     string
	FileName      string
	ContentLength int64
	SHA256        string
	Status        int64
	ExpiresAt     time.Time
	CreatedAt     time.Time
	FinalizedAt   *time.Time
}

type FinalizeTestDataUploadInput struct {
	ProblemID        int64
	UploadID         string
	SHA256           string
	ContentLength    int64
	ExpectedRevision int64
	ActorID          int64
	TestCases        []TestCaseInput
}

type FinalizeTestDataUploadResult struct {
	TestDataVersion string
	Revision        int64
	TestCases       []TestCaseRecord
}

type problemRevisionRow struct {
	ID                   int64          `gorm:"column:id;primaryKey"`
	ProblemID            int64          `gorm:"column:problem_id"`
	Revision             int64          `gorm:"column:revision"`
	Title                string         `gorm:"column:title"`
	Description          string         `gorm:"column:description"`
	InputDescription     string         `gorm:"column:input_description"`
	OutputDescription    string         `gorm:"column:output_description"`
	SamplesJSON          string         `gorm:"column:samples_json"`
	Hint                 string         `gorm:"column:hint"`
	Difficulty           int64          `gorm:"column:difficulty"`
	Source               string         `gorm:"column:source"`
	TimeLimitMS          int64          `gorm:"column:time_limit_ms"`
	MemoryLimitMB        int64          `gorm:"column:memory_limit_mb"`
	AllowedLanguagesJSON string         `gorm:"column:allowed_languages_json"`
	CheckerType          int64          `gorm:"column:checker_type"`
	CheckerFileKey       sql.NullString `gorm:"column:checker_file_key"`
	InteractorFileKey    sql.NullString `gorm:"column:interactor_file_key"`
	TestDataVersionID    sql.NullInt64  `gorm:"column:test_data_version_id"`
	Status               int64          `gorm:"column:status"`
	Visibility           int64          `gorm:"column:visibility"`
	CreatedBy            int64          `gorm:"column:created_by"`
	CreatedAt            time.Time      `gorm:"column:created_at"`
}

func (problemRevisionRow) TableName() string { return "problem_revisions" }

type testDataUploadRow struct {
	UploadID                   string        `gorm:"column:upload_id;primaryKey"`
	ProblemID                  int64         `gorm:"column:problem_id"`
	ObjectKey                  string        `gorm:"column:object_key"`
	FileName                   string        `gorm:"column:file_name"`
	ContentLength              int64         `gorm:"column:content_length"`
	Checksum                   string        `gorm:"column:checksum"`
	Status                     int64         `gorm:"column:status"`
	ExpiresAt                  time.Time     `gorm:"column:expires_at"`
	CreatedAt                  time.Time     `gorm:"column:created_at"`
	FinalizedAt                sql.NullTime  `gorm:"column:finalized_at"`
	FinalizedTestDataVersionID sql.NullInt64 `gorm:"column:finalized_test_data_version_id"`
	FinalizedRevision          sql.NullInt64 `gorm:"column:finalized_revision"`
}

func (testDataUploadRow) TableName() string { return "test_data_uploads" }
