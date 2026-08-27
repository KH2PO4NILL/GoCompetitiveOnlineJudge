package logic

import (
	"context"
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

type logicProblemModel struct {
	model.ProblemModel

	createFn              func(context.Context, model.CreateProblemInput) (*model.ProblemAggregate, error)
	getForEditFn          func(context.Context, int64) (*model.ProblemAggregate, error)
	getPublicFn           func(context.Context, int64) (*model.ProblemAggregate, error)
	getPublishedFn        func(context.Context, int64) (*model.ProblemAggregate, error)
	getCurrentTestCasesFn func(context.Context, int64) ([]model.TestCaseRecord, error)
	updateFn              func(context.Context, model.UpdateProblemInput) (*model.ProblemAggregate, error)
	publishFn             func(context.Context, int64, int64, int64) (*model.ProblemAggregate, error)
	archiveFn             func(context.Context, int64, int64, int64) (*model.ProblemAggregate, error)
	restoreFn             func(context.Context, int64, int64, int64) (*model.ProblemAggregate, error)
	listPublicFn          func(context.Context, model.ProblemListFilter) ([]model.ProblemSummary, int64, error)
	listManagementFn      func(context.Context, model.ProblemListFilter) ([]model.ProblemSummary, int64, error)
	batchFn               func(context.Context, []int64) ([]*model.ProblemSummary, error)
	judgeSpecFn           func(context.Context, int64, int64) (*model.JudgeProblemSpec, error)
	getTestDataVersionFn  func(context.Context, int64, int64) (string, error)
	createUploadFn        func(context.Context, model.CreateTestDataUploadInput) (*model.TestDataUpload, error)
	getUploadFn           func(context.Context, string) (*model.TestDataUpload, error)
	finalizeUploadFn      func(context.Context, model.FinalizeTestDataUploadInput) (*model.FinalizeTestDataUploadResult, error)
}

func (m *logicProblemModel) Create(ctx context.Context, input model.CreateProblemInput) (*model.ProblemAggregate, error) {
	return m.createFn(ctx, input)
}

func (m *logicProblemModel) GetForEdit(ctx context.Context, problemID int64) (*model.ProblemAggregate, error) {
	return m.getForEditFn(ctx, problemID)
}

func (m *logicProblemModel) GetPublic(ctx context.Context, problemID int64) (*model.ProblemAggregate, error) {
	return m.getPublicFn(ctx, problemID)
}

func (m *logicProblemModel) GetPublished(ctx context.Context, problemID int64) (*model.ProblemAggregate, error) {
	return m.getPublishedFn(ctx, problemID)
}

func (m *logicProblemModel) GetCurrentTestCases(ctx context.Context, problemID int64) ([]model.TestCaseRecord, error) {
	return m.getCurrentTestCasesFn(ctx, problemID)
}

func (m *logicProblemModel) Update(ctx context.Context, input model.UpdateProblemInput) (*model.ProblemAggregate, error) {
	return m.updateFn(ctx, input)
}

func (m *logicProblemModel) Publish(ctx context.Context, problemID, revision, actorID int64) (*model.ProblemAggregate, error) {
	return m.publishFn(ctx, problemID, revision, actorID)
}

func (m *logicProblemModel) Archive(ctx context.Context, problemID, revision, actorID int64) (*model.ProblemAggregate, error) {
	return m.archiveFn(ctx, problemID, revision, actorID)
}

func (m *logicProblemModel) Restore(ctx context.Context, problemID, revision, actorID int64) (*model.ProblemAggregate, error) {
	return m.restoreFn(ctx, problemID, revision, actorID)
}

func (m *logicProblemModel) ListPublic(ctx context.Context, filter model.ProblemListFilter) ([]model.ProblemSummary, int64, error) {
	return m.listPublicFn(ctx, filter)
}

func (m *logicProblemModel) ListForManagement(ctx context.Context, filter model.ProblemListFilter) ([]model.ProblemSummary, int64, error) {
	return m.listManagementFn(ctx, filter)
}

func (m *logicProblemModel) BatchGetPublicSummaries(ctx context.Context, ids []int64) ([]*model.ProblemSummary, error) {
	return m.batchFn(ctx, ids)
}

func (m *logicProblemModel) GetJudgeSpec(ctx context.Context, problemID, revision int64) (*model.JudgeProblemSpec, error) {
	return m.judgeSpecFn(ctx, problemID, revision)
}

func (m *logicProblemModel) GetTestDataVersion(ctx context.Context, problemID, versionID int64) (string, error) {
	return m.getTestDataVersionFn(ctx, problemID, versionID)
}

func (m *logicProblemModel) CreateTestDataUpload(ctx context.Context, input model.CreateTestDataUploadInput) (*model.TestDataUpload, error) {
	return m.createUploadFn(ctx, input)
}

func (m *logicProblemModel) GetTestDataUpload(ctx context.Context, uploadID string) (*model.TestDataUpload, error) {
	return m.getUploadFn(ctx, uploadID)
}

func (m *logicProblemModel) FinalizeTestDataUpload(ctx context.Context, input model.FinalizeTestDataUploadInput) (*model.FinalizeTestDataUploadResult, error) {
	return m.finalizeUploadFn(ctx, input)
}

type stubTestDataStorage struct {
	url          string
	object       *svc.TestDataObject
	createErr    error
	inspectErr   error
	inspectCalls int
}

func (s *stubTestDataStorage) CreateUploadURL(context.Context, *model.TestDataUpload) (string, error) {
	return s.url, s.createErr
}

func (s *stubTestDataStorage) InspectUpload(context.Context, *model.TestDataUpload) (*svc.TestDataObject, error) {
	s.inspectCalls++
	return s.object, s.inspectErr
}

func validEditableFields() *pb.ProblemEditableFields {
	return &pb.ProblemEditableFields{
		Statement:  &pb.ProblemStatement{Title: "A + B", Description: "description"},
		Difficulty: pb.ProblemDifficulty_PROBLEM_DIFFICULTY_EASY,
		Tags:       []string{"math"}, Source: "local",
		JudgeSettings: &pb.JudgeSettings{
			TimeLimitMs: 1000, MemoryLimitMb: 256,
			AllowedLanguages: []string{"go"}, CheckerType: pb.CheckerType_CHECKER_TYPE_STANDARD,
		},
		Visibility: pb.ProblemVisibility_PROBLEM_VISIBILITY_PRIVATE,
	}
}

func TestCreateProblemUsesAuthenticatedAuthor(t *testing.T) {
	var captured model.CreateProblemInput
	repository := &logicProblemModel{createFn: func(_ context.Context, input model.CreateProblemInput) (*model.ProblemAggregate, error) {
		captured = input
		return testProblemAggregate(9, input.AuthorID, 1), nil
	}}
	logic := NewCreateProblemLogic(principalContext(42, authpb.UserRole_USER_ROLE_USER), &svc.ServiceContext{ProblemModel: repository})
	response, err := logic.CreateProblem(&pb.CreateProblemReq{Problem: validEditableFields()})
	if err != nil {
		t.Fatalf("CreateProblem() error = %v", err)
	}
	if captured.AuthorID != 42 || response.Problem.Metadata.AuthorId != 42 {
		t.Fatalf("author IDs = (%d, %d), want 42", captured.AuthorID, response.Problem.Metadata.AuthorId)
	}
}

func TestUpdateProblemAppliesFieldMaskForAuthor(t *testing.T) {
	var captured model.UpdateProblemInput
	repository := &logicProblemModel{
		getForEditFn: func(context.Context, int64) (*model.ProblemAggregate, error) {
			return testProblemAggregate(9, 42, 1), nil
		},
		updateFn: func(_ context.Context, input model.UpdateProblemInput) (*model.ProblemAggregate, error) {
			captured = input
			result := testProblemAggregate(9, 42, 2)
			result.Editable.Title = *input.Patch.Title
			return result, nil
		},
	}
	logic := NewUpdateProblemLogic(principalContext(42, authpb.UserRole_USER_ROLE_USER), &svc.ServiceContext{ProblemModel: repository})
	response, err := logic.UpdateProblem(&pb.UpdateProblemReq{
		ProblemId: 9, ExpectedRevision: 1,
		Problem:    &pb.ProblemEditableFields{Statement: &pb.ProblemStatement{Title: "New title"}},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"statement.title"}},
	})
	if err != nil {
		t.Fatalf("UpdateProblem() error = %v", err)
	}
	if captured.ActorID != 42 || captured.Patch.Title == nil || *captured.Patch.Title != "New title" {
		t.Fatalf("Update() input = %+v", captured)
	}
	if response.Problem.Revision != 2 || response.Problem.Statement.Title != "New title" {
		t.Fatalf("UpdateProblem() response = %+v", response.Problem)
	}
}

func TestUpdateProblemRejectsNonAuthor(t *testing.T) {
	repository := &logicProblemModel{getForEditFn: func(context.Context, int64) (*model.ProblemAggregate, error) {
		return testProblemAggregate(9, 42, 1), nil
	}}
	logic := NewUpdateProblemLogic(principalContext(7, authpb.UserRole_USER_ROLE_USER), &svc.ServiceContext{ProblemModel: repository})
	_, err := logic.UpdateProblem(&pb.UpdateProblemReq{
		ProblemId: 9, ExpectedRevision: 1,
		Problem:    &pb.ProblemEditableFields{Statement: &pb.ProblemStatement{Title: "New title"}},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"statement.title"}},
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("UpdateProblem() code = %s, want %s", status.Code(err), codes.PermissionDenied)
	}
}

func TestListProblemsSeparatesPublicAndManagementQueries(t *testing.T) {
	now := time.Now().UTC()
	repository := &logicProblemModel{
		listPublicFn: func(_ context.Context, filter model.ProblemListFilter) ([]model.ProblemSummary, int64, error) {
			if filter.Status != nil || filter.AuthorID != nil {
				t.Fatalf("public filter leaked management fields: %+v", filter)
			}
			return []model.ProblemSummary{{ProblemID: 1, Title: "Public", UpdatedAt: now}}, 1, nil
		},
		listManagementFn: func(_ context.Context, filter model.ProblemListFilter) ([]model.ProblemSummary, int64, error) {
			if filter.AuthorID == nil || *filter.AuthorID != 42 {
				t.Fatalf("management author filter = %v, want 42", filter.AuthorID)
			}
			return []model.ProblemSummary{}, 0, nil
		},
	}
	publicLogic := NewListProblemsLogic(context.Background(), &svc.ServiceContext{ProblemModel: repository})
	response, err := publicLogic.ListProblems(&pb.ListProblemsReq{})
	if err != nil || response.Total != 1 {
		t.Fatalf("public ListProblems() = (%+v, %v)", response, err)
	}

	managementLogic := NewListProblemsLogic(principalContext(42, authpb.UserRole_USER_ROLE_USER), &svc.ServiceContext{ProblemModel: repository})
	_, err = managementLogic.ListProblems(&pb.ListProblemsReq{Status: pb.ProblemStatus_PROBLEM_STATUS_DRAFT})
	if err != nil {
		t.Fatalf("management ListProblems() error = %v", err)
	}
}

func TestBatchGetProblemsRequiresTrustedServiceAndOmitsMissing(t *testing.T) {
	summary := &model.ProblemSummary{ProblemID: 2, Title: "Two", UpdatedAt: time.Now().UTC()}
	repository := &logicProblemModel{batchFn: func(context.Context, []int64) ([]*model.ProblemSummary, error) {
		return []*model.ProblemSummary{nil, summary}, nil
	}}
	userLogic := NewBatchGetProblemsLogic(principalContext(1, authpb.UserRole_USER_ROLE_ADMIN), &svc.ServiceContext{ProblemModel: repository})
	if _, err := userLogic.BatchGetProblems(&pb.BatchGetProblemsReq{ProblemIds: []int64{1, 2}}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("user BatchGetProblems() code = %s", status.Code(err))
	}
	internalLogic := NewBatchGetProblemsLogic(serviceContext("submit.rpc"), &svc.ServiceContext{ProblemModel: repository})
	response, err := internalLogic.BatchGetProblems(&pb.BatchGetProblemsReq{ProblemIds: []int64{1, 2}})
	if err != nil || len(response.Problems) != 1 || response.Problems[0].ProblemId != 2 {
		t.Fatalf("internal BatchGetProblems() = (%+v, %v)", response, err)
	}
}

func TestBatchGetProblemsPreservesDuplicatesAndEnforcesLimit(t *testing.T) {
	summary := &model.ProblemSummary{ProblemID: 2, Title: "Two", UpdatedAt: time.Now().UTC()}
	repository := &logicProblemModel{batchFn: func(_ context.Context, ids []int64) ([]*model.ProblemSummary, error) {
		if len(ids) != 3 || ids[1] != 2 || ids[2] != 2 {
			t.Fatalf("BatchGetPublicSummaries() IDs = %v", ids)
		}
		return []*model.ProblemSummary{nil, summary, summary}, nil
	}}
	logic := NewBatchGetProblemsLogic(serviceContext("submit.rpc"), &svc.ServiceContext{ProblemModel: repository})
	response, err := logic.BatchGetProblems(&pb.BatchGetProblemsReq{ProblemIds: []int64{1, 2, 2}})
	if err != nil || len(response.Problems) != 2 || response.Problems[0].ProblemId != 2 || response.Problems[1].ProblemId != 2 {
		t.Fatalf("BatchGetProblems() = (%+v, %v)", response, err)
	}
	tooMany := make([]int64, maxBatchSize+1)
	for index := range tooMany {
		tooMany[index] = int64(index + 1)
	}
	if _, err := logic.BatchGetProblems(&pb.BatchGetProblemsReq{ProblemIds: tooMany}); status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("oversized BatchGetProblems() code = %s", status.Code(err))
	}
}

func TestTestDataUploadBusinessFlow(t *testing.T) {
	checksum := strings.Repeat("a", 64)
	upload := &model.TestDataUpload{
		UploadID: "upload-1", ProblemID: 9, ObjectKey: "problem-test-data/9/upload-1/data.zip",
		ContentLength: 128, SHA256: checksum, Status: model.TestDataUploadStatusPending,
		ExpiresAt: time.Now().Add(time.Minute),
	}
	var finalized model.FinalizeTestDataUploadInput
	repository := &logicProblemModel{
		getForEditFn: func(context.Context, int64) (*model.ProblemAggregate, error) {
			return testProblemAggregate(9, 42, 1), nil
		},
		createUploadFn: func(context.Context, model.CreateTestDataUploadInput) (*model.TestDataUpload, error) {
			return upload, nil
		},
		getUploadFn: func(context.Context, string) (*model.TestDataUpload, error) {
			return upload, nil
		},
		finalizeUploadFn: func(_ context.Context, input model.FinalizeTestDataUploadInput) (*model.FinalizeTestDataUploadResult, error) {
			finalized = input
			return &model.FinalizeTestDataUploadResult{
				TestDataVersion: "upload-1", Revision: 2,
				TestCases: []model.TestCaseRecord{{ID: 10, TestCaseInput: input.TestCases[0]}},
			}, nil
		},
	}
	storage := &stubTestDataStorage{
		url: "https://storage.invalid/upload-1",
		object: &svc.TestDataObject{ContentLength: 128, SHA256: checksum, PackageKey: "problem-test-data/9/versions/upload-1.zip", TestCases: []model.TestCaseInput{{
			Name: "case-1", Score: 100, Ordinal: 1, Enabled: true,
		}}},
	}
	serviceContext := &svc.ServiceContext{ProblemModel: repository, TestDataStorage: storage}
	ctx := principalContext(42, authpb.UserRole_USER_ROLE_USER)
	createLogic := NewCreateTestDataUploadLogic(ctx, serviceContext)
	createResponse, err := createLogic.CreateTestDataUpload(&pb.CreateTestDataUploadReq{
		ProblemId: 9, FileName: "data.zip", ContentLength: 128, Sha256: checksum,
	})
	if err != nil || createResponse.UploadUrl != storage.url {
		t.Fatalf("CreateTestDataUpload() = (%+v, %v)", createResponse, err)
	}

	finalizeLogic := NewFinalizeTestDataUploadLogic(ctx, serviceContext)
	finalizeResponse, err := finalizeLogic.FinalizeTestDataUpload(&pb.FinalizeTestDataUploadReq{
		ProblemId: 9, UploadId: "upload-1", Sha256: checksum, ExpectedRevision: 1,
	})
	if err != nil {
		t.Fatalf("FinalizeTestDataUpload() error = %v", err)
	}
	if finalized.ActorID != 42 || finalized.PackageKey != storage.object.PackageKey || len(finalized.TestCases) != 1 || finalizeResponse.Revision != 2 {
		t.Fatalf("FinalizeTestDataUpload() input/response = (%+v, %+v)", finalized, finalizeResponse)
	}
}
