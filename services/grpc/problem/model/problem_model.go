package model

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

var (
	ErrInvalidProblemInput     = errors.New("invalid problem input")
	ErrProblemNotFound         = errors.New("problem not found")
	ErrProblemRevisionConflict = errors.New("problem revision conflict")
	ErrProblemAlreadyDeleted   = errors.New("problem already deleted")
	ErrProblemNotDeleted       = errors.New("problem is not deleted")
	ErrProblemStateConflict    = errors.New("problem state conflict")
	ErrProblemNotPublishable   = errors.New("problem is not publishable")
	ErrTestDataUploadNotFound  = errors.New("test data upload not found")
	ErrTestDataUploadConflict  = errors.New("test data upload conflict")
	ErrTestDataUploadExpired   = errors.New("test data upload expired")
	ErrTestDataUploadFinalized = errors.New("test data upload already finalized with different input")
)

const (
	ProblemStatusDraft     int64 = 1
	ProblemStatusPublished int64 = 2
	ProblemStatusArchived  int64 = 3
	ProblemStatusDeleted   int64 = 4

	ProblemVisibilityPrivate     int64 = 1
	ProblemVisibilityPublic      int64 = 2
	ProblemVisibilityContestOnly int64 = 3

	TestDataUploadStatusPending   int64 = 1
	TestDataUploadStatusFinalized int64 = 2
	TestDataUploadStatusExpired   int64 = 3
	TestDataUploadStatusFailed    int64 = 4
)

// ProblemModel is the only persistence boundary exposed to Problem RPC logic.
// Every method preserves revision, soft-delete and cross-table transaction
// invariants; callers should not compose writes from generated table CRUDs.
type ProblemModel interface {
	Create(ctx context.Context, input CreateProblemInput) (*ProblemAggregate, error)
	Update(ctx context.Context, input UpdateProblemInput) (*ProblemAggregate, error)
	Publish(ctx context.Context, problemID, expectedRevision, actorID int64) (*ProblemAggregate, error)
	Archive(ctx context.Context, problemID, expectedRevision, actorID int64) (*ProblemAggregate, error)
	SoftDelete(ctx context.Context, problemID, expectedRevision, actorID int64) (newRevision int64, err error)
	Restore(ctx context.Context, problemID, expectedRevision, actorID int64) (*ProblemAggregate, error)

	GetForEdit(ctx context.Context, problemID int64) (*ProblemAggregate, error)
	GetPublic(ctx context.Context, problemID int64) (*ProblemAggregate, error)
	GetPublished(ctx context.Context, problemID int64) (*ProblemAggregate, error)
	GetRevision(ctx context.Context, problemID, revision int64) (*ProblemRevision, error)
	GetJudgeSpec(ctx context.Context, problemID, revision int64) (*JudgeProblemSpec, error)
	GetTestDataVersion(ctx context.Context, problemID, versionID int64) (string, error)
	GetCurrentTestCases(ctx context.Context, problemID int64) ([]TestCaseRecord, error)
	ListPublic(ctx context.Context, filter ProblemListFilter) ([]ProblemSummary, int64, error)
	ListForManagement(ctx context.Context, filter ProblemListFilter) ([]ProblemSummary, int64, error)
	BatchGetPublicSummaries(ctx context.Context, problemIDs []int64) ([]*ProblemSummary, error)

	CreateTestDataUpload(ctx context.Context, input CreateTestDataUploadInput) (*TestDataUpload, error)
	GetTestDataUpload(ctx context.Context, uploadID string) (*TestDataUpload, error)
	FinalizeTestDataUpload(ctx context.Context, input FinalizeTestDataUploadInput) (*FinalizeTestDataUploadResult, error)
}

type defaultProblemModel struct {
	db  *gorm.DB
	now func() time.Time
}

func NewProblemModel(db *gorm.DB) ProblemModel {
	return &defaultProblemModel{
		db: db,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}
