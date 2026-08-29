package model

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

var (
	ErrInvalidContestInput          = errors.New("invalid contest input")
	ErrContestNotFound              = errors.New("contest not found")
	ErrContestRevisionConflict      = errors.New("contest revision conflict")
	ErrContestStateConflict         = errors.New("contest state conflict")
	ErrContestAlreadyDeleted        = errors.New("contest already deleted")
	ErrContestNotDeleted            = errors.New("contest is not deleted")
	ErrContestNotPublishable        = errors.New("contest is not publishable")
	ErrContestProblemConflict       = errors.New("contest problem conflict")
	ErrRegistrationNotFound         = errors.New("contest registration not found")
	ErrRegistrationAlreadyExists    = errors.New("contest registration already exists")
	ErrRegistrationRevisionConflict = errors.New("contest registration revision conflict")
	ErrRegistrationStateConflict    = errors.New("contest registration state conflict")
	ErrRegistrationClosed           = errors.New("contest registration is closed")
	ErrParticipantNotFound          = errors.New("contest participant not found")
	ErrParticipantStateConflict     = errors.New("contest participant state conflict")
	ErrInvalidInviteCode            = errors.New("invalid contest invite code")
	ErrScoreboardNotReady           = errors.New("contest scoreboard is not ready")
	ErrScoreboardVersionConflict    = errors.New("contest scoreboard version conflict")
	ErrScoreboardAlreadyFrozen      = errors.New("contest scoreboard is already frozen")
	ErrScoreboardNotFrozen          = errors.New("contest scoreboard is not frozen")
	ErrScoreboardSnapshotConflict   = errors.New("contest scoreboard snapshot conflict")
)

const (
	ContestModeICPC int64 = 1
	ContestModeIOI  int64 = 2
	ContestModeOI   int64 = 3

	ContestStatusDraft     int64 = 1
	ContestStatusPublished int64 = 2
	ContestStatusRunning   int64 = 3 // derived, never persisted
	ContestStatusFinished  int64 = 4 // derived, never persisted
	ContestStatusCancelled int64 = 5
	ContestStatusArchived  int64 = 6
	ContestStatusDeleted   int64 = 7

	ContestVisibilityPublic   int64 = 1
	ContestVisibilityUnlisted int64 = 2
	ContestVisibilityPrivate  int64 = 3

	RegistrationPolicyOpen             int64 = 1
	RegistrationPolicyApprovalRequired int64 = 2
	RegistrationPolicyInviteOnly       int64 = 3

	RegistrationStatusPending   int64 = 1
	RegistrationStatusApproved  int64 = 2
	RegistrationStatusRejected  int64 = 3
	RegistrationStatusCancelled int64 = 4

	RegistrationDecisionApprove int64 = 1
	RegistrationDecisionReject  int64 = 2

	ParticipantStatusNormal       int64 = 1
	ParticipantStatusDisqualified int64 = 2
	ParticipantStatusWithdrawn    int64 = 3

	ScoreboardVisibilityPublic           int64 = 1
	ScoreboardVisibilityParticipantsOnly int64 = 2
	ScoreboardVisibilityHidden           int64 = 3

	ScoreboardViewLive         int64 = 1
	ScoreboardViewFrozenPublic int64 = 2
	ScoreboardViewFinal        int64 = 3
)

// ContestModel is the only persistence boundary exposed to Contest RPC logic.
// Implementations preserve contest revision, registration, participant and
// scoreboard snapshot invariants across tables.
type ContestModel interface {
	Create(ctx context.Context, input CreateContestInput) (*ContestAggregate, error)
	Update(ctx context.Context, input UpdateContestInput) (*ContestAggregate, error)
	Publish(ctx context.Context, contestID, expectedRevision int64) (*ContestAggregate, error)
	Cancel(ctx context.Context, contestID, expectedRevision int64, reason string) (*ContestAggregate, error)
	Archive(ctx context.Context, contestID, expectedRevision int64) (*ContestAggregate, error)
	SoftDelete(ctx context.Context, contestID, expectedRevision int64) (newRevision int64, err error)
	Restore(ctx context.Context, contestID, expectedRevision int64) (*ContestAggregate, error)

	GetForEdit(ctx context.Context, contestID int64) (*ContestAggregate, error)
	GetDetail(ctx context.Context, contestID int64, at time.Time) (*ContestDetailRecord, error)
	ListPublic(ctx context.Context, filter ContestListFilter) ([]ContestSummary, int64, error)
	ListForManagement(ctx context.Context, filter ContestListFilter) ([]ContestSummary, int64, error)

	SetProblems(ctx context.Context, input SetContestProblemsInput) (*ContestAggregate, []ContestProblem, error)
	ListProblems(ctx context.Context, contestID int64) ([]ContestProblem, error)

	Register(ctx context.Context, input RegisterContestInput) (*ContestRegistration, *ContestParticipant, error)
	CancelRegistration(ctx context.Context, contestID, userID, expectedRevision int64) (*ContestRegistration, error)
	ReviewRegistration(ctx context.Context, input ReviewRegistrationInput) (*ContestRegistration, *ContestParticipant, error)
	SetParticipantStatus(ctx context.Context, input SetParticipantStatusInput) (*ContestParticipant, error)
	GetMyStatus(ctx context.Context, contestID, userID int64) (*MyContestStatus, error)
	ListRegistrations(ctx context.Context, filter RegistrationListFilter) ([]ContestRegistration, int64, error)
	ListParticipants(ctx context.Context, filter ParticipantListFilter) ([]ContestParticipant, int64, error)
	GetAccessSnapshot(ctx context.Context, contestID, problemID, userID int64, at time.Time) (*ContestAccessSnapshot, error)
	VerifyInviteCode(ctx context.Context, contestID int64, candidate string) error

	ReplaceScoreboard(ctx context.Context, input ReplaceScoreboardInput) error
	GetScoreboard(ctx context.Context, filter ScoreboardFilter) (*ScoreboardPage, error)
	ListDueScoreboardFreezes(ctx context.Context, at time.Time, limit int) ([]ScoreboardFreezeCandidate, error)
	FreezeScoreboard(ctx context.Context, contestID, expectedRevision int64) (*ContestAggregate, error)
	UnfreezeScoreboard(ctx context.Context, contestID, expectedRevision int64) (*ContestAggregate, error)
}

type defaultContestModel struct {
	db  *gorm.DB
	now func() time.Time
}

type ContestModelOption func(*defaultContestModel)

// WithClock supplies the service clock used for lifecycle and registration
// windows. Production callers normally omit it; deterministic tests can inject
// a fixed clock without relying on package internals.
func WithClock(now func() time.Time) ContestModelOption {
	return func(model *defaultContestModel) {
		if now != nil {
			model.now = now
		}
	}
}

func NewContestModel(db *gorm.DB, options ...ContestModelOption) ContestModel {
	model := &defaultContestModel{
		db: db,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
	for _, option := range options {
		if option != nil {
			option(model)
		}
	}
	return model
}
