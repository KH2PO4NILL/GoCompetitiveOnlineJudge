package logic

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	authpb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

type stubContestAccess struct {
	allowed          bool
	err              error
	calls            int
	userID           int64
	requesterIsAdmin bool
}

func (s *stubContestAccess) CanAccessProblem(_ context.Context, _, _, userID int64, requesterIsAdmin bool) (bool, error) {
	s.calls++
	s.userID = userID
	s.requesterIsAdmin = requesterIsAdmin
	return s.allowed, s.err
}

func TestGetProblemPublicAndContestVisibility(t *testing.T) {
	public := testProblemAggregate(1, 42, 3)
	public.Status = model.ProblemStatusPublished
	public.Editable.Visibility = model.ProblemVisibilityPublic
	contest := testProblemAggregate(2, 42, 4)
	contest.Status = model.ProblemStatusPublished
	contest.Editable.Visibility = model.ProblemVisibilityContestOnly
	repository := &logicProblemModel{
		getPublicFn: func(_ context.Context, id int64) (*model.ProblemAggregate, error) {
			if id != 1 {
				return nil, model.ErrProblemNotFound
			}
			return public, nil
		},
		getPublishedFn: func(context.Context, int64) (*model.ProblemAggregate, error) { return contest, nil },
	}
	publicLogic := NewGetProblemLogic(context.Background(), &svc.ServiceContext{ProblemModel: repository})
	response, err := publicLogic.GetProblem(&pb.GetProblemReq{ProblemId: 1})
	if err != nil || response.Problem.ProblemId != 1 {
		t.Fatalf("public GetProblem() = (%+v, %v)", response, err)
	}

	checker := &stubContestAccess{allowed: true}
	contestLogic := NewGetProblemLogic(principalContext(7, authpb.UserRole_USER_ROLE_USER), &svc.ServiceContext{
		ProblemModel: repository, ContestAccess: checker,
	})
	contestID := int64(10)
	response, err = contestLogic.GetProblem(&pb.GetProblemReq{ProblemId: 2, ContestId: &contestID})
	if err != nil || response.Problem.ProblemId != 2 || checker.calls != 1 || checker.userID != 7 || checker.requesterIsAdmin {
		t.Fatalf("contest GetProblem() = (%+v, %v), checker = %+v", response, err, checker)
	}
	adminLogic := NewGetProblemLogic(principalContext(8, authpb.UserRole_USER_ROLE_ADMIN), &svc.ServiceContext{
		ProblemModel: repository, ContestAccess: checker,
	})
	response, err = adminLogic.GetProblem(&pb.GetProblemReq{ProblemId: 2, ContestId: &contestID})
	if err != nil || response.Problem.ProblemId != 2 || checker.userID != 8 || !checker.requesterIsAdmin {
		t.Fatalf("admin contest GetProblem() = (%+v, %v), checker = %+v", response, err, checker)
	}

	checker.allowed = false
	_, err = contestLogic.GetProblem(&pb.GetProblemReq{ProblemId: 2, ContestId: &contestID})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("denied contest GetProblem() code = %s, want NotFound", status.Code(err))
	}
}

func TestGetProblemForEditReturnsCurrentTestCases(t *testing.T) {
	versionID := int64(3)
	repository := &logicProblemModel{
		getForEditFn: func(context.Context, int64) (*model.ProblemAggregate, error) {
			problem := testProblemAggregate(9, 42, 1)
			problem.CurrentTestDataVersionID = &versionID
			return problem, nil
		},
		getTestDataVersionFn: func(_ context.Context, problemID, gotVersionID int64) (string, error) {
			if problemID != 9 || gotVersionID != versionID {
				t.Fatalf("GetTestDataVersion(%d, %d)", problemID, gotVersionID)
			}
			return "upload-version", nil
		},
		getCurrentTestCasesFn: func(context.Context, int64) ([]model.TestCaseRecord, error) {
			return []model.TestCaseRecord{{ID: 8, TestCaseInput: model.TestCaseInput{Name: "case", Ordinal: 1, Enabled: true}}}, nil
		},
	}
	logic := NewGetProblemForEditLogic(principalContext(42, authpb.UserRole_USER_ROLE_USER), &svc.ServiceContext{ProblemModel: repository})
	response, err := logic.GetProblemForEdit(&pb.GetProblemForEditReq{ProblemId: 9})
	if err != nil || response.Problem.ProblemId != 9 || response.Problem.JudgeConfig.TestDataVersion != "upload-version" ||
		len(response.TestCases) != 1 || response.TestCases[0].TestCaseId != 8 {
		t.Fatalf("GetProblemForEdit() = (%+v, %v)", response, err)
	}
}

func TestFinalizeTestDataUploadIsIdempotentWithoutReinspection(t *testing.T) {
	checksum := strings.Repeat("b", 64)
	upload := &model.TestDataUpload{
		UploadID: "upload-final", ProblemID: 9, ContentLength: 128, SHA256: checksum,
		Status: model.TestDataUploadStatusFinalized, ExpiresAt: time.Now().Add(-time.Hour),
	}
	repository := &logicProblemModel{
		getForEditFn: func(context.Context, int64) (*model.ProblemAggregate, error) {
			return testProblemAggregate(9, 42, 3), nil
		},
		getUploadFn: func(context.Context, string) (*model.TestDataUpload, error) { return upload, nil },
		finalizeUploadFn: func(context.Context, model.FinalizeTestDataUploadInput) (*model.FinalizeTestDataUploadResult, error) {
			return &model.FinalizeTestDataUploadResult{TestDataVersion: "upload-final", Revision: 2}, nil
		},
	}
	storage := &stubTestDataStorage{}
	logic := NewFinalizeTestDataUploadLogic(principalContext(42, authpb.UserRole_USER_ROLE_USER), &svc.ServiceContext{
		ProblemModel: repository, TestDataStorage: storage,
	})
	response, err := logic.FinalizeTestDataUpload(&pb.FinalizeTestDataUploadReq{
		ProblemId: 9, UploadId: upload.UploadID, Sha256: checksum, ExpectedRevision: 1,
	})
	if err != nil || response.Revision != 2 || storage.inspectCalls != 0 {
		t.Fatalf("FinalizeTestDataUpload() = (%+v, %v), inspect calls = %d", response, err, storage.inspectCalls)
	}
}

func TestProblemStateTransitionSuccessPaths(t *testing.T) {
	base := testProblemAggregate(9, 42, 1)
	repository := &logicProblemModel{
		getForEditFn: func(context.Context, int64) (*model.ProblemAggregate, error) { return base, nil },
		publishFn: func(context.Context, int64, int64, int64) (*model.ProblemAggregate, error) {
			result := testProblemAggregate(9, 42, 2)
			result.Status = model.ProblemStatusPublished
			return result, nil
		},
		archiveFn: func(context.Context, int64, int64, int64) (*model.ProblemAggregate, error) {
			result := testProblemAggregate(9, 42, 2)
			result.Status = model.ProblemStatusArchived
			return result, nil
		},
		restoreFn: func(context.Context, int64, int64, int64) (*model.ProblemAggregate, error) {
			return testProblemAggregate(9, 42, 2), nil
		},
	}
	serviceContext := &svc.ServiceContext{ProblemModel: repository}
	authorCtx := principalContext(42, authpb.UserRole_USER_ROLE_USER)
	if response, err := NewPublishProblemLogic(authorCtx, serviceContext).PublishProblem(&pb.PublishProblemReq{ProblemId: 9, ExpectedRevision: 1}); err != nil || response.Problem.Revision != 2 {
		t.Fatalf("PublishProblem() = (%+v, %v)", response, err)
	}
	if response, err := NewArchiveProblemLogic(authorCtx, serviceContext).ArchiveProblem(&pb.ArchiveProblemReq{ProblemId: 9, ExpectedRevision: 1}); err != nil || response.Problem.Revision != 2 {
		t.Fatalf("ArchiveProblem() = (%+v, %v)", response, err)
	}
	adminCtx := principalContext(1, authpb.UserRole_USER_ROLE_ADMIN)
	if response, err := NewRestoreProblemLogic(adminCtx, serviceContext).RestoreProblem(&pb.RestoreProblemReq{ProblemId: 9, ExpectedRevision: 1}); err != nil || response.Problem.Revision != 2 {
		t.Fatalf("RestoreProblem() = (%+v, %v)", response, err)
	}
}

func TestGetJudgeProblemSpecRequiresServiceAndSupportsRevision(t *testing.T) {
	var revisions []int64
	repository := &logicProblemModel{judgeSpecFn: func(_ context.Context, problemID, revision int64) (*model.JudgeProblemSpec, error) {
		revisions = append(revisions, revision)
		return &model.JudgeProblemSpec{
			ProblemID: problemID, Revision: revision, TimeLimitMS: 1000, MemoryLimitMB: 256,
			AllowedLanguages: []string{"go"}, CheckerType: 1,
			TestDataVersion: "upload-1", TestDataPackageKey: "private/package.zip",
			TestDataChecksum: "checksum", TestCases: []model.TestCaseRecord{{ID: 1}},
		}, nil
	}}
	svcCtx := &svc.ServiceContext{ProblemModel: repository}
	if _, err := NewGetJudgeProblemSpecLogic(context.Background(), svcCtx).GetJudgeProblemSpec(&pb.GetJudgeProblemSpecReq{ProblemId: 9}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("anonymous GetJudgeProblemSpec() code = %s", status.Code(err))
	}
	response, err := NewGetJudgeProblemSpecLogic(serviceContext("judge.worker"), svcCtx).GetJudgeProblemSpec(&pb.GetJudgeProblemSpecReq{ProblemId: 9, Revision: 4})
	if err != nil || response.Spec.Revision != 4 || response.Spec.TestDataPackageKey != "private/package.zip" {
		t.Fatalf("GetJudgeProblemSpec() = (%+v, %v)", response, err)
	}
	if _, err := NewGetJudgeProblemSpecLogic(serviceContext("judge.worker"), svcCtx).GetJudgeProblemSpec(&pb.GetJudgeProblemSpecReq{ProblemId: 9}); err != nil {
		t.Fatalf("GetJudgeProblemSpec(current) error = %v", err)
	}
	if len(revisions) != 2 || revisions[0] != 4 || revisions[1] != 0 {
		t.Fatalf("GetJudgeSpec() revisions = %v", revisions)
	}
}

func TestAllProblemLogicRejectNilRequest(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	tests := []struct {
		name string
		call func() error
	}{
		{name: "get", call: func() error { _, err := NewGetProblemLogic(context.Background(), svcCtx).GetProblem(nil); return err }},
		{name: "list", call: func() error {
			_, err := NewListProblemsLogic(context.Background(), svcCtx).ListProblems(nil)
			return err
		}},
		{name: "create", call: func() error {
			_, err := NewCreateProblemLogic(context.Background(), svcCtx).CreateProblem(nil)
			return err
		}},
		{name: "get edit", call: func() error {
			_, err := NewGetProblemForEditLogic(context.Background(), svcCtx).GetProblemForEdit(nil)
			return err
		}},
		{name: "update", call: func() error {
			_, err := NewUpdateProblemLogic(context.Background(), svcCtx).UpdateProblem(nil)
			return err
		}},
		{name: "publish", call: func() error {
			_, err := NewPublishProblemLogic(context.Background(), svcCtx).PublishProblem(nil)
			return err
		}},
		{name: "archive", call: func() error {
			_, err := NewArchiveProblemLogic(context.Background(), svcCtx).ArchiveProblem(nil)
			return err
		}},
		{name: "delete", call: func() error {
			_, err := NewDeleteProblemLogic(context.Background(), svcCtx).DeleteProblem(nil)
			return err
		}},
		{name: "restore", call: func() error {
			_, err := NewRestoreProblemLogic(context.Background(), svcCtx).RestoreProblem(nil)
			return err
		}},
		{name: "create upload", call: func() error {
			_, err := NewCreateTestDataUploadLogic(context.Background(), svcCtx).CreateTestDataUpload(nil)
			return err
		}},
		{name: "finalize upload", call: func() error {
			_, err := NewFinalizeTestDataUploadLogic(context.Background(), svcCtx).FinalizeTestDataUpload(nil)
			return err
		}},
		{name: "batch", call: func() error {
			_, err := NewBatchGetProblemsLogic(context.Background(), svcCtx).BatchGetProblems(nil)
			return err
		}},
		{name: "judge spec", call: func() error {
			_, err := NewGetJudgeProblemSpecLogic(context.Background(), svcCtx).GetJudgeProblemSpec(nil)
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := status.Code(test.call()); got != codes.InvalidArgument {
				t.Fatalf("nil request code = %s, want InvalidArgument", got)
			}
		})
	}
}

func TestUploadValidationAndFinalizeFailures(t *testing.T) {
	checksum := strings.Repeat("a", 64)
	policy := svc.UploadPolicy{MaxUploadBytes: 10, AllowedExtensions: []string{".zip"}}
	createLogic := NewCreateTestDataUploadLogic(context.Background(), &svc.ServiceContext{UploadPolicy: policy})
	createTests := []struct {
		request *pb.CreateTestDataUploadReq
		code    codes.Code
	}{
		{request: &pb.CreateTestDataUploadReq{ProblemId: 1, FileName: "data.tar", ContentLength: 1, Sha256: checksum}, code: codes.InvalidArgument},
		{request: &pb.CreateTestDataUploadReq{ProblemId: 1, FileName: "data.zip", ContentLength: 11, Sha256: checksum}, code: codes.ResourceExhausted},
		{request: &pb.CreateTestDataUploadReq{ProblemId: 1, FileName: "data.zip", ContentLength: 1, Sha256: "bad"}, code: codes.InvalidArgument},
	}
	for _, test := range createTests {
		if got := status.Code(func() error { _, err := createLogic.CreateTestDataUpload(test.request); return err }()); got != test.code {
			t.Errorf("CreateTestDataUpload(%+v) code = %s, want %s", test.request, got, test.code)
		}
	}

	upload := &model.TestDataUpload{
		UploadID: "upload", ProblemID: 9, ContentLength: 1, SHA256: checksum,
		Status: model.TestDataUploadStatusPending, ExpiresAt: time.Now().Add(-time.Minute),
	}
	repository := &logicProblemModel{
		getForEditFn: func(context.Context, int64) (*model.ProblemAggregate, error) {
			return testProblemAggregate(9, 42, 1), nil
		},
		getUploadFn: func(context.Context, string) (*model.TestDataUpload, error) { return upload, nil },
	}
	finalizeLogic := NewFinalizeTestDataUploadLogic(principalContext(42, authpb.UserRole_USER_ROLE_USER), &svc.ServiceContext{
		ProblemModel: repository, TestDataStorage: &stubTestDataStorage{},
	})
	_, err := finalizeLogic.FinalizeTestDataUpload(&pb.FinalizeTestDataUploadReq{
		ProblemId: 9, UploadId: upload.UploadID, Sha256: checksum, ExpectedRevision: 1,
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expired FinalizeTestDataUpload() code = %s", status.Code(err))
	}
	upload.ExpiresAt = time.Now().Add(time.Minute)
	upload.ProblemID = 10
	_, err = finalizeLogic.FinalizeTestDataUpload(&pb.FinalizeTestDataUploadReq{
		ProblemId: 9, UploadId: upload.UploadID, Sha256: checksum, ExpectedRevision: 1,
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("cross-problem FinalizeTestDataUpload() code = %s", status.Code(err))
	}
}

func TestProblemPatchRejectsInvalidMasks(t *testing.T) {
	tests := []struct {
		name  string
		paths []string
	}{
		{name: "empty", paths: nil},
		{name: "unknown", paths: []string{"author_id"}},
		{name: "duplicate", paths: []string{"source", "source"}},
		{name: "overlap", paths: []string{"statement", "statement.title"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := problemPatch(validEditableFields(), &fieldmaskpb.FieldMask{Paths: test.paths})
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("problemPatch() code = %s, want InvalidArgument", status.Code(err))
			}
		})
	}
}

func TestMapModelErrorCodes(t *testing.T) {
	tests := []struct {
		err  error
		code codes.Code
	}{
		{model.ErrInvalidProblemInput, codes.InvalidArgument},
		{model.ErrProblemNotFound, codes.NotFound},
		{model.ErrProblemRevisionConflict, codes.Aborted},
		{model.ErrProblemStateConflict, codes.FailedPrecondition},
		{model.ErrProblemAlreadyDeleted, codes.FailedPrecondition},
		{model.ErrProblemNotDeleted, codes.FailedPrecondition},
		{model.ErrProblemNotPublishable, codes.FailedPrecondition},
		{model.ErrTestDataUploadNotFound, codes.NotFound},
		{model.ErrTestDataUploadExpired, codes.FailedPrecondition},
		{model.ErrTestDataUploadFinalized, codes.FailedPrecondition},
		{model.ErrTestDataUploadConflict, codes.FailedPrecondition},
		{errors.New("database detail"), codes.Internal},
	}
	for _, test := range tests {
		if got := status.Code(mapModelError(test.err)); got != test.code {
			t.Errorf("mapModelError(%v) = %s, want %s", test.err, got, test.code)
		}
	}
}

func TestProtoConversionHandlesNullablePublishedAt(t *testing.T) {
	aggregate := testProblemAggregate(1, 2, 3)
	if got := problemProto(aggregate, ""); got.PublishedAt != nil {
		t.Fatalf("PublishedAt = %v, want nil", got.PublishedAt)
	}
	now := time.Now().UTC()
	aggregate.PublishedAt = &now
	if got := problemProto(aggregate, ""); got.PublishedAt == nil || !got.PublishedAt.AsTime().Equal(now) {
		t.Fatalf("PublishedAt = %v, want %v", got.PublishedAt, now)
	}
}
