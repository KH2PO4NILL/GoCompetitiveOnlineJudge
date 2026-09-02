package logic

import (
	"context"
	"testing"
	"time"

	infraauth "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"
	authpb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type logicContestModel struct {
	model.ContestModel

	contest          *model.ContestAggregate
	detail           *model.ContestDetailRecord
	problems         []model.ContestProblem
	registration     *model.ContestRegistration
	participant      *model.ContestParticipant
	myStatus         *model.MyContestStatus
	scoreboard       *model.ScoreboardPage
	access           *model.ContestAccessSnapshot
	lastUpdate       model.UpdateContestInput
	lastSetProblems  model.SetContestProblemsInput
	lastRegister     model.RegisterContestInput
	lastReview       model.ReviewRegistrationInput
	lastScoreboard   model.ScoreboardFilter
	lastAccessAt     time.Time
	softDeleteCalled bool
	operationError   error
}

func (m *logicContestModel) Create(context.Context, model.CreateContestInput) (*model.ContestAggregate, error) {
	return m.contest, m.operationError
}
func (m *logicContestModel) Update(_ context.Context, input model.UpdateContestInput) (*model.ContestAggregate, error) {
	m.lastUpdate = input
	return m.contest, m.operationError
}
func (m *logicContestModel) Publish(context.Context, int64, int64) (*model.ContestAggregate, error) {
	return m.contest, m.operationError
}
func (m *logicContestModel) Cancel(context.Context, int64, int64, string) (*model.ContestAggregate, error) {
	return m.contest, m.operationError
}
func (m *logicContestModel) Archive(context.Context, int64, int64) (*model.ContestAggregate, error) {
	return m.contest, m.operationError
}
func (m *logicContestModel) SoftDelete(context.Context, int64, int64) (int64, error) {
	m.softDeleteCalled = true
	return m.contest.Revision + 1, m.operationError
}
func (m *logicContestModel) Restore(context.Context, int64, int64) (*model.ContestAggregate, error) {
	return m.contest, m.operationError
}
func (m *logicContestModel) GetForEdit(context.Context, int64) (*model.ContestAggregate, error) {
	return m.contest, m.operationError
}
func (m *logicContestModel) GetDetail(context.Context, int64, time.Time) (*model.ContestDetailRecord, error) {
	return m.detail, m.operationError
}
func (m *logicContestModel) ListPublic(context.Context, model.ContestListFilter) ([]model.ContestSummary, int64, error) {
	return []model.ContestSummary{{
		ContestID: m.detail.ContestID, Title: m.detail.Title, Mode: m.detail.Mode, Status: m.detail.Status,
		StartAt: m.detail.StartAt, EndAt: m.detail.EndAt, RegistrationPolicy: m.detail.RegistrationPolicy,
		Visibility: m.detail.Visibility, RegistrationOpen: m.detail.RegistrationOpen,
	}}, 1, m.operationError
}
func (m *logicContestModel) SetProblems(_ context.Context, input model.SetContestProblemsInput) (*model.ContestAggregate, []model.ContestProblem, error) {
	m.lastSetProblems = input
	return m.contest, m.problems, m.operationError
}
func (m *logicContestModel) ListProblems(context.Context, int64) ([]model.ContestProblem, error) {
	return append([]model.ContestProblem(nil), m.problems...), m.operationError
}
func (m *logicContestModel) Register(_ context.Context, input model.RegisterContestInput) (*model.ContestRegistration, *model.ContestParticipant, error) {
	m.lastRegister = input
	return m.registration, m.participant, m.operationError
}
func (m *logicContestModel) CancelRegistration(context.Context, int64, int64, int64) (*model.ContestRegistration, error) {
	return m.registration, m.operationError
}
func (m *logicContestModel) ReviewRegistration(_ context.Context, input model.ReviewRegistrationInput) (*model.ContestRegistration, *model.ContestParticipant, error) {
	m.lastReview = input
	return m.registration, m.participant, m.operationError
}
func (m *logicContestModel) GetMyStatus(context.Context, int64, int64) (*model.MyContestStatus, error) {
	return m.myStatus, m.operationError
}
func (m *logicContestModel) ListRegistrations(context.Context, model.RegistrationListFilter) ([]model.ContestRegistration, int64, error) {
	return []model.ContestRegistration{*m.registration}, 1, m.operationError
}
func (m *logicContestModel) ListParticipants(context.Context, model.ParticipantListFilter) ([]model.ContestParticipant, int64, error) {
	return []model.ContestParticipant{*m.participant}, 1, m.operationError
}
func (m *logicContestModel) GetAccessSnapshot(_ context.Context, _, _, _ int64, at time.Time) (*model.ContestAccessSnapshot, error) {
	m.lastAccessAt = at
	return m.access, m.operationError
}
func (m *logicContestModel) GetScoreboard(_ context.Context, filter model.ScoreboardFilter) (*model.ScoreboardPage, error) {
	m.lastScoreboard = filter
	result := *m.scoreboard
	result.View = filter.View
	result.Frozen = filter.View == model.ScoreboardViewFrozenPublic
	return &result, m.operationError
}
func (m *logicContestModel) FreezeScoreboard(context.Context, int64, int64) (*model.ContestAggregate, error) {
	return m.contest, m.operationError
}
func (m *logicContestModel) UnfreezeScoreboard(context.Context, int64, int64) (*model.ContestAggregate, error) {
	return m.contest, m.operationError
}

type fakeProblemCatalog struct {
	ids []int64
	err error
}

func (f fakeProblemCatalog) ListPublishedForContest(context.Context, []int64) ([]int64, error) {
	return append([]int64(nil), f.ids...), f.err
}

func contestPrincipal(userID int64, role authpb.UserRole) context.Context {
	return infraauth.WithPrincipal(context.Background(), infraauth.Principal{
		UserID: userID, Role: role, Status: authpb.UserStatus_USER_STATUS_NORMAL,
	})
}

func contestService(name string) context.Context {
	return infraauth.WithServicePrincipal(context.Background(), infraauth.ServicePrincipal{Name: name})
}

func testContestFixture(now time.Time) (*logicContestModel, *svc.ServiceContext) {
	contest := &model.ContestAggregate{
		ContestID: 10, CreatorID: 7, Status: model.ContestStatusPublished,
		EffectiveStatus: model.ContestStatusRunning, Revision: 3,
		Editable: model.EditableContest{
			Title: "Round", Description: "contest", Mode: model.ContestModeICPC,
			StartAt: now.Add(-time.Hour), EndAt: now.Add(time.Hour),
			RegistrationPolicy:   model.RegistrationPolicyOpen,
			Visibility:           model.ContestVisibilityPublic,
			ScoreboardVisibility: model.ScoreboardVisibilityPublic,
		},
		CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now,
	}
	detail := &model.ContestDetailRecord{
		ContestID: contest.ContestID, Title: contest.Editable.Title, Description: contest.Editable.Description,
		Mode: contest.Editable.Mode, Status: model.ContestStatusRunning,
		StartAt: contest.Editable.StartAt, EndAt: contest.Editable.EndAt,
		RegistrationPolicy: contest.Editable.RegistrationPolicy,
		Visibility:         contest.Editable.Visibility, ScoreboardVisibility: contest.Editable.ScoreboardVisibility,
		RegistrationOpen: false,
	}
	registration := &model.ContestRegistration{
		RegistrationID: 20, ContestID: contest.ContestID, UserID: contest.CreatorID,
		Status: model.RegistrationStatusApproved, Revision: 2, CreatedAt: now.Add(-time.Hour), UpdatedAt: now,
	}
	participant := &model.ContestParticipant{
		ContestID: contest.ContestID, UserID: contest.CreatorID, RegistrationID: registration.RegistrationID,
		Status: model.ParticipantStatusNormal, QualifiedAt: now.Add(-time.Hour), UpdatedAt: now,
	}
	problem := model.ContestProblem{ContestID: contest.ContestID, ProblemID: 100, Label: "A", DisplayOrder: 0}
	repository := &logicContestModel{
		contest: contest, detail: detail, problems: []model.ContestProblem{problem},
		registration: registration, participant: participant,
		myStatus: &model.MyContestStatus{ContestID: contest.ContestID, Registration: registration, Participant: participant},
		access: &model.ContestAccessSnapshot{
			ContestID: contest.ContestID, Mode: model.ContestModeICPC,
			LifecycleStatus: model.ContestStatusPublished, EffectiveStatus: model.ContestStatusRunning,
			ProblemAssigned: true, RegistrationStatus: int64TestPointer(model.RegistrationStatusApproved),
			ParticipantStatus: int64TestPointer(model.ParticipantStatusNormal),
		},
		scoreboard: &model.ScoreboardPage{
			ContestID: contest.ContestID, Mode: model.ContestModeICPC, GeneratedAt: now,
			View: model.ScoreboardViewLive, Total: 1,
			Rows: []model.ScoreboardRow{{
				Rank: 1, Ordinal: 1, UserID: contest.CreatorID, DisplayName: "user",
				ICPC: &model.IcpcScore{}, ProblemResults: []model.ScoreboardProblemResult{{
					ProblemID: problem.ProblemID, Label: problem.Label, ICPC: &model.IcpcProblemResult{},
				}},
			}},
		},
	}
	serviceContext := &svc.ServiceContext{
		ContestModel: repository, ProblemCatalog: fakeProblemCatalog{ids: []int64{problem.ProblemID}},
		Now: func() time.Time { return now },
	}
	return repository, serviceContext
}

func int64TestPointer(value int64) *int64 { return &value }

func editableRequest(now time.Time) *pb.ContestEditableFields {
	return &pb.ContestEditableFields{
		Title: "Round", Description: "contest", Mode: pb.ContestMode_CONTEST_MODE_ICPC,
		StartAt: timestamppb.New(now.Add(time.Hour)), EndAt: timestamppb.New(now.Add(2 * time.Hour)),
		RegistrationPolicy:   pb.RegistrationPolicy_REGISTRATION_POLICY_OPEN,
		Visibility:           pb.ContestVisibility_CONTEST_VISIBILITY_PUBLIC,
		ScoreboardVisibility: pb.ScoreboardVisibility_SCOREBOARD_VISIBILITY_PUBLIC,
	}
}

func TestAllContestRPCLogicHappyPaths(t *testing.T) {
	now := time.Date(2026, 8, 30, 4, 0, 0, 0, time.UTC)
	repository, svcCtx := testContestFixture(now)
	userCtx := contestPrincipal(7, authpb.UserRole_USER_ROLE_USER)

	if response, err := NewListContestsLogic(context.Background(), svcCtx).ListContests(&pb.ListContestsReq{}); err != nil || len(response.Contests) != 1 {
		t.Fatalf("ListContests() = (%+v, %v)", response, err)
	}
	if response, err := NewGetContestLogic(context.Background(), svcCtx).GetContest(&pb.GetContestReq{ContestId: 10}); err != nil || response.Contest == nil {
		t.Fatalf("GetContest() = (%+v, %v)", response, err)
	}
	if response, err := NewRegisterContestLogic(userCtx, svcCtx).RegisterContest(&pb.RegisterContestReq{ContestId: 10}); err != nil || response.Registration == nil {
		t.Fatalf("RegisterContest() = (%+v, %v)", response, err)
	}
	if repository.lastRegister.UserID != 7 || !repository.lastRegister.At.Equal(now) {
		t.Fatalf("Register input = %+v", repository.lastRegister)
	}
	if response, err := NewCancelRegistrationLogic(userCtx, svcCtx).CancelRegistration(&pb.CancelRegistrationReq{ContestId: 10, ExpectedRevision: 2}); err != nil || response.Registration == nil {
		t.Fatalf("CancelRegistration() = (%+v, %v)", response, err)
	}
	if response, err := NewGetMyContestStatusLogic(userCtx, svcCtx).GetMyContestStatus(&pb.GetMyContestStatusReq{ContestId: 10}); err != nil || response.Status == nil || !response.Status.CanSubmit {
		t.Fatalf("GetMyContestStatus() = (%+v, %v)", response, err)
	}
	if response, err := NewListContestProblemsLogic(userCtx, svcCtx).ListContestProblems(&pb.ListContestProblemsReq{ContestId: 10}); err != nil || len(response.Problems) != 1 {
		t.Fatalf("ListContestProblems() = (%+v, %v)", response, err)
	}
	if response, err := NewGetScoreboardLogic(userCtx, svcCtx).GetScoreboard(&pb.GetScoreboardReq{ContestId: 10}); err != nil || response.Scoreboard == nil {
		t.Fatalf("GetScoreboard() = (%+v, %v)", response, err)
	}
	if repository.lastScoreboard.View != model.ScoreboardViewLive {
		t.Fatalf("scoreboard view = %d", repository.lastScoreboard.View)
	}

	if response, err := NewCreateContestLogic(userCtx, svcCtx).CreateContest(&pb.CreateContestReq{Contest: editableRequest(now)}); err != nil || response.Contest == nil {
		t.Fatalf("CreateContest() = (%+v, %v)", response, err)
	}
	if response, err := NewGetContestForEditLogic(userCtx, svcCtx).GetContestForEdit(&pb.GetContestForEditReq{ContestId: 10}); err != nil || response.Contest == nil || len(response.Problems) != 1 {
		t.Fatalf("GetContestForEdit() = (%+v, %v)", response, err)
	}
	if response, err := NewUpdateContestLogic(userCtx, svcCtx).UpdateContest(&pb.UpdateContestReq{
		ContestId: 10, Contest: &pb.ContestEditableFields{Title: "Updated"},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"title"}}, ExpectedRevision: 3,
	}); err != nil || response.Contest == nil || repository.lastUpdate.Patch.Title == nil {
		t.Fatalf("UpdateContest() = (%+v, %v), input=%+v", response, err, repository.lastUpdate)
	}
	if response, err := NewPublishContestLogic(userCtx, svcCtx).PublishContest(&pb.PublishContestReq{ContestId: 10, ExpectedRevision: 3}); err != nil || response.Contest == nil {
		t.Fatalf("PublishContest() = (%+v, %v)", response, err)
	}
	if response, err := NewCancelContestLogic(userCtx, svcCtx).CancelContest(&pb.CancelContestReq{ContestId: 10, ExpectedRevision: 3}); err != nil || response.Contest == nil {
		t.Fatalf("CancelContest() = (%+v, %v)", response, err)
	}
	if response, err := NewArchiveContestLogic(userCtx, svcCtx).ArchiveContest(&pb.ArchiveContestReq{ContestId: 10, ExpectedRevision: 3}); err != nil || response.Contest == nil {
		t.Fatalf("ArchiveContest() = (%+v, %v)", response, err)
	}
	if response, err := NewDeleteContestLogic(userCtx, svcCtx).DeleteContest(&pb.DeleteContestReq{ContestId: 10, ExpectedRevision: 3}); err != nil || response.Revision != 4 {
		t.Fatalf("DeleteContest() = (%+v, %v)", response, err)
	}
	if response, err := NewRestoreContestLogic(userCtx, svcCtx).RestoreContest(&pb.RestoreContestReq{ContestId: 10, ExpectedRevision: 3}); err != nil || response.Contest == nil {
		t.Fatalf("RestoreContest() = (%+v, %v)", response, err)
	}
	if response, err := NewSetContestProblemsLogic(userCtx, svcCtx).SetContestProblems(&pb.SetContestProblemsReq{
		ContestId: 10, ExpectedRevision: 3,
		Problems: []*pb.ContestProblem{{ProblemId: 100, Label: "A", DisplayOrder: 0}},
	}); err != nil || response.Contest == nil || len(response.Problems) != 1 {
		t.Fatalf("SetContestProblems() = (%+v, %v)", response, err)
	}
	if response, err := NewListRegistrationsLogic(userCtx, svcCtx).ListRegistrations(&pb.ListRegistrationsReq{ContestId: 10}); err != nil || len(response.Registrations) != 1 {
		t.Fatalf("ListRegistrations() = (%+v, %v)", response, err)
	}
	if response, err := NewReviewRegistrationLogic(userCtx, svcCtx).ReviewRegistration(&pb.ReviewRegistrationReq{
		ContestId: 10, RegistrationId: 20, Decision: pb.RegistrationReviewDecision_REGISTRATION_REVIEW_DECISION_APPROVE,
		ExpectedRevision: 2,
	}); err != nil || response.Registration == nil || response.Participant == nil || repository.lastReview.ReviewerID != 7 {
		t.Fatalf("ReviewRegistration() = (%+v, %v), input=%+v", response, err, repository.lastReview)
	}
	if response, err := NewListParticipantsLogic(userCtx, svcCtx).ListParticipants(&pb.ListParticipantsReq{ContestId: 10}); err != nil || len(response.Participants) != 1 {
		t.Fatalf("ListParticipants() = (%+v, %v)", response, err)
	}
	if response, err := NewFreezeScoreboardLogic(userCtx, svcCtx).FreezeScoreboard(&pb.FreezeScoreboardReq{ContestId: 10, ExpectedRevision: 3}); err != nil || response.Contest == nil {
		t.Fatalf("FreezeScoreboard() = (%+v, %v)", response, err)
	}
	if response, err := NewUnfreezeScoreboardLogic(userCtx, svcCtx).UnfreezeScoreboard(&pb.UnfreezeScoreboardReq{ContestId: 10, ExpectedRevision: 3}); err != nil || response.Contest == nil {
		t.Fatalf("UnfreezeScoreboard() = (%+v, %v)", response, err)
	}

	if response, err := NewCanAccessProblemLogic(contestService("problem.rpc"), svcCtx).CanAccessProblem(&pb.CanAccessProblemReq{
		ContestId: 10, ProblemId: 100, UserId: 7,
	}); err != nil || !response.Allowed || response.DenyReason != pb.SubmissionDenyReason_SUBMISSION_DENY_REASON_UNSPECIFIED {
		t.Fatalf("CanAccessProblem() = (%+v, %v)", response, err)
	}
	submittedAt := timestamppb.New(now)
	if response, err := NewValidateContestSubmissionLogic(contestService("submit.rpc"), svcCtx).ValidateContestSubmission(&pb.ValidateContestSubmissionReq{
		ContestId: 10, ProblemId: 100, UserId: 7, SubmittedAt: submittedAt,
	}); err != nil || !response.Allowed || response.Mode != pb.ContestMode_CONTEST_MODE_ICPC {
		t.Fatalf("ValidateContestSubmission() = (%+v, %v)", response, err)
	}
	if !repository.lastAccessAt.Equal(now) {
		t.Fatalf("submission access time = %v, want %v", repository.lastAccessAt, now)
	}
	if response, err := NewResolveSubmissionResultVisibilityLogic(contestService("submit.rpc"), svcCtx).
		ResolveSubmissionResultVisibility(&pb.ResolveSubmissionResultVisibilityReq{
			ContestId: 10, UserId: 7, ObservedAt: timestamppb.New(now),
		}); err != nil || response.Visibility != pb.SubmissionResultVisibility_SUBMISSION_RESULT_VISIBILITY_FULL {
		t.Fatalf("ResolveSubmissionResultVisibility() = (%+v, %v)", response, err)
	}
}

func TestResolveSubmissionResultVisibilityKeepsOIHistoryUnlocked(t *testing.T) {
	now := time.Date(2026, 8, 30, 4, 0, 0, 0, time.UTC)
	repository, svcCtx := testContestFixture(now)
	repository.contest.Editable.Mode = model.ContestModeOI
	repository.contest.Editable.EndAt = now.Add(time.Hour)

	request := &pb.ResolveSubmissionResultVisibilityReq{
		ContestId: repository.contest.ContestID, UserId: 7, ObservedAt: timestamppb.New(now),
	}
	response, err := NewResolveSubmissionResultVisibilityLogic(contestService("submit.rpc"), svcCtx).
		ResolveSubmissionResultVisibility(request)
	if err != nil || response.Visibility != pb.SubmissionResultVisibility_SUBMISSION_RESULT_VISIBILITY_STATUS_ONLY {
		t.Fatalf("running OI visibility = (%+v, %v)", response, err)
	}

	for _, statusValue := range []int64{
		model.ContestStatusCancelled,
		model.ContestStatusArchived,
		model.ContestStatusDeleted,
	} {
		repository.contest.Status = statusValue
		response, err = NewResolveSubmissionResultVisibilityLogic(contestService("submit.rpc"), svcCtx).
			ResolveSubmissionResultVisibility(request)
		if err != nil || response.Visibility != pb.SubmissionResultVisibility_SUBMISSION_RESULT_VISIBILITY_FULL {
			t.Fatalf("OI visibility for lifecycle %d = (%+v, %v)", statusValue, response, err)
		}
	}

	repository.contest.Status = model.ContestStatusPublished
	request.ObservedAt = timestamppb.New(repository.contest.Editable.EndAt)
	response, err = NewResolveSubmissionResultVisibilityLogic(contestService("submit.rpc"), svcCtx).
		ResolveSubmissionResultVisibility(request)
	if err != nil || response.Visibility != pb.SubmissionResultVisibility_SUBMISSION_RESULT_VISIBILITY_FULL {
		t.Fatalf("finished OI visibility = (%+v, %v)", response, err)
	}
}

func TestContestAuthorizationAndVisibilityBoundaries(t *testing.T) {
	now := time.Date(2026, 8, 30, 4, 0, 0, 0, time.UTC)
	repository, svcCtx := testContestFixture(now)

	otherUser := contestPrincipal(8, authpb.UserRole_USER_ROLE_USER)
	_, err := NewDeleteContestLogic(otherUser, svcCtx).DeleteContest(&pb.DeleteContestReq{ContestId: 10, ExpectedRevision: 3})
	if status.Code(err) != codes.PermissionDenied || repository.softDeleteCalled {
		t.Fatalf("non-owner DeleteContest() error = %v, called=%t", err, repository.softDeleteCalled)
	}
	admin := contestPrincipal(8, authpb.UserRole_USER_ROLE_ADMIN)
	if _, err := NewDeleteContestLogic(admin, svcCtx).DeleteContest(&pb.DeleteContestReq{ContestId: 10, ExpectedRevision: 3}); err != nil {
		t.Fatalf("admin DeleteContest() error = %v", err)
	}

	repository.detail.Visibility = model.ContestVisibilityPrivate
	_, err = NewGetContestLogic(context.Background(), svcCtx).GetContest(&pb.GetContestReq{ContestId: 10})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("anonymous private GetContest() code = %s", status.Code(err))
	}
	if _, err := NewGetContestLogic(contestPrincipal(7, authpb.UserRole_USER_ROLE_USER), svcCtx).GetContest(&pb.GetContestReq{ContestId: 10}); err != nil {
		t.Fatalf("creator private GetContest() error = %v", err)
	}
}

func TestContestLogicRejectsOversizedPaginationAndWrongInternalCaller(t *testing.T) {
	_, svcCtx := testContestFixture(time.Now().UTC())
	_, err := NewListContestsLogic(context.Background(), svcCtx).ListContests(&pb.ListContestsReq{PageSize: 101})
	if status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("oversized ListContests() code = %s", status.Code(err))
	}
	_, err = NewCanAccessProblemLogic(contestService("submit.rpc"), svcCtx).CanAccessProblem(&pb.CanAccessProblemReq{
		ContestId: 10, ProblemId: 100, UserId: 7,
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("wrong caller CanAccessProblem() code = %s", status.Code(err))
	}
}

func TestContestManagerProblemAccessIsConsistent(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	repository, svcCtx := testContestFixture(now)
	repository.myStatus = &model.MyContestStatus{ContestID: repository.contest.ContestID}
	repository.access = &model.ContestAccessSnapshot{
		ContestID: repository.contest.ContestID, Mode: model.ContestModeICPC,
		LifecycleStatus: model.ContestStatusPublished, EffectiveStatus: model.ContestStatusRunning,
		ProblemAssigned: true, Manager: true,
	}
	creatorCtx := contestPrincipal(repository.contest.CreatorID, authpb.UserRole_USER_ROLE_USER)

	statusResponse, err := NewGetMyContestStatusLogic(creatorCtx, svcCtx).GetMyContestStatus(
		&pb.GetMyContestStatusReq{ContestId: repository.contest.ContestID},
	)
	if err != nil || statusResponse.Status == nil || !statusResponse.Status.CanViewProblems || statusResponse.Status.CanSubmit {
		t.Fatalf("manager GetMyContestStatus() = (%+v, %v)", statusResponse, err)
	}
	if _, err := NewListContestProblemsLogic(creatorCtx, svcCtx).ListContestProblems(
		&pb.ListContestProblemsReq{ContestId: repository.contest.ContestID},
	); err != nil {
		t.Fatalf("manager ListContestProblems() error = %v", err)
	}
	accessResponse, err := NewCanAccessProblemLogic(contestService("problem.rpc"), svcCtx).CanAccessProblem(
		&pb.CanAccessProblemReq{
			ContestId: repository.contest.ContestID, ProblemId: repository.problems[0].ProblemID,
			UserId: repository.contest.CreatorID,
		},
	)
	if err != nil || !accessResponse.Allowed {
		t.Fatalf("manager CanAccessProblem() = (%+v, %v)", accessResponse, err)
	}

	repository.access.Manager = false
	adminAccess, err := NewCanAccessProblemLogic(contestService("problem.rpc"), svcCtx).CanAccessProblem(
		&pb.CanAccessProblemReq{
			ContestId: repository.contest.ContestID, ProblemId: repository.problems[0].ProblemID,
			UserId: 8, RequesterIsAdmin: true,
		},
	)
	if err != nil || !adminAccess.Allowed {
		t.Fatalf("admin CanAccessProblem() = (%+v, %v)", adminAccess, err)
	}
}

func TestValidateContestSubmissionRejectsEmptySnapshot(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	repository, svcCtx := testContestFixture(now)
	repository.access = nil
	response, err := NewValidateContestSubmissionLogic(contestService("submit.rpc"), svcCtx).ValidateContestSubmission(
		&pb.ValidateContestSubmissionReq{
			ContestId: repository.contest.ContestID, ProblemId: repository.problems[0].ProblemID,
			UserId: repository.contest.CreatorID, SubmittedAt: timestamppb.New(now),
		},
	)
	if response != nil || status.Code(err) != codes.Internal {
		t.Fatalf("ValidateContestSubmission() = (%+v, %v), code = %s", response, err, status.Code(err))
	}
	accessResponse, accessErr := NewCanAccessProblemLogic(contestService("problem.rpc"), svcCtx).CanAccessProblem(
		&pb.CanAccessProblemReq{
			ContestId: repository.contest.ContestID, ProblemId: repository.problems[0].ProblemID,
			UserId: repository.contest.CreatorID,
		},
	)
	if accessResponse != nil || status.Code(accessErr) != codes.Internal {
		t.Fatalf("CanAccessProblem() = (%+v, %v), code = %s", accessResponse, accessErr, status.Code(accessErr))
	}
}

func TestAllContestRPCLogicRejectNilRequests(t *testing.T) {
	_, svcCtx := testContestFixture(time.Now().UTC())
	userCtx := contestPrincipal(7, authpb.UserRole_USER_ROLE_USER)
	tests := []struct {
		name string
		call func() error
	}{
		{"ListContests", func() error {
			_, err := NewListContestsLogic(context.Background(), svcCtx).ListContests(nil)
			return err
		}},
		{"GetContest", func() error { _, err := NewGetContestLogic(context.Background(), svcCtx).GetContest(nil); return err }},
		{"RegisterContest", func() error { _, err := NewRegisterContestLogic(userCtx, svcCtx).RegisterContest(nil); return err }},
		{"CancelRegistration", func() error {
			_, err := NewCancelRegistrationLogic(userCtx, svcCtx).CancelRegistration(nil)
			return err
		}},
		{"GetMyContestStatus", func() error {
			_, err := NewGetMyContestStatusLogic(userCtx, svcCtx).GetMyContestStatus(nil)
			return err
		}},
		{"ListContestProblems", func() error {
			_, err := NewListContestProblemsLogic(userCtx, svcCtx).ListContestProblems(nil)
			return err
		}},
		{"GetScoreboard", func() error { _, err := NewGetScoreboardLogic(userCtx, svcCtx).GetScoreboard(nil); return err }},
		{"CreateContest", func() error { _, err := NewCreateContestLogic(userCtx, svcCtx).CreateContest(nil); return err }},
		{"GetContestForEdit", func() error { _, err := NewGetContestForEditLogic(userCtx, svcCtx).GetContestForEdit(nil); return err }},
		{"UpdateContest", func() error { _, err := NewUpdateContestLogic(userCtx, svcCtx).UpdateContest(nil); return err }},
		{"PublishContest", func() error { _, err := NewPublishContestLogic(userCtx, svcCtx).PublishContest(nil); return err }},
		{"CancelContest", func() error { _, err := NewCancelContestLogic(userCtx, svcCtx).CancelContest(nil); return err }},
		{"ArchiveContest", func() error { _, err := NewArchiveContestLogic(userCtx, svcCtx).ArchiveContest(nil); return err }},
		{"DeleteContest", func() error { _, err := NewDeleteContestLogic(userCtx, svcCtx).DeleteContest(nil); return err }},
		{"RestoreContest", func() error { _, err := NewRestoreContestLogic(userCtx, svcCtx).RestoreContest(nil); return err }},
		{"SetContestProblems", func() error {
			_, err := NewSetContestProblemsLogic(userCtx, svcCtx).SetContestProblems(nil)
			return err
		}},
		{"ListRegistrations", func() error { _, err := NewListRegistrationsLogic(userCtx, svcCtx).ListRegistrations(nil); return err }},
		{"ReviewRegistration", func() error {
			_, err := NewReviewRegistrationLogic(userCtx, svcCtx).ReviewRegistration(nil)
			return err
		}},
		{"ListParticipants", func() error { _, err := NewListParticipantsLogic(userCtx, svcCtx).ListParticipants(nil); return err }},
		{"FreezeScoreboard", func() error { _, err := NewFreezeScoreboardLogic(userCtx, svcCtx).FreezeScoreboard(nil); return err }},
		{"UnfreezeScoreboard", func() error {
			_, err := NewUnfreezeScoreboardLogic(userCtx, svcCtx).UnfreezeScoreboard(nil)
			return err
		}},
		{"CanAccessProblem", func() error {
			_, err := NewCanAccessProblemLogic(contestService("problem.rpc"), svcCtx).CanAccessProblem(nil)
			return err
		}},
		{"ValidateContestSubmission", func() error {
			_, err := NewValidateContestSubmissionLogic(contestService("submit.rpc"), svcCtx).ValidateContestSubmission(nil)
			return err
		}},
		{"ResolveSubmissionResultVisibility", func() error {
			_, err := NewResolveSubmissionResultVisibilityLogic(contestService("submit.rpc"), svcCtx).
				ResolveSubmissionResultVisibility(nil)
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if code := status.Code(test.call()); code != codes.InvalidArgument {
				t.Fatalf("nil request code = %s", code)
			}
		})
	}
}

func TestGetScoreboardUsesFrozenView(t *testing.T) {
	now := time.Now().UTC()
	repository, svcCtx := testContestFixture(now)
	repository.detail.ScoreboardFrozen = true
	response, err := NewGetScoreboardLogic(contestPrincipal(7, authpb.UserRole_USER_ROLE_USER), svcCtx).
		GetScoreboard(&pb.GetScoreboardReq{ContestId: 10})
	if err != nil || response.Scoreboard == nil || repository.lastScoreboard.View != model.ScoreboardViewFrozenPublic || !response.Scoreboard.Frozen {
		t.Fatalf("frozen GetScoreboard() = (%+v, %v), filter=%+v", response, err, repository.lastScoreboard)
	}
}
