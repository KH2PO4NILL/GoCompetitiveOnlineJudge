package logic

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type FinalizeTestDataUploadLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewFinalizeTestDataUploadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FinalizeTestDataUploadLogic {
	return &FinalizeTestDataUploadLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *FinalizeTestDataUploadLogic) FinalizeTestDataUpload(in *pb.FinalizeTestDataUploadReq) (*pb.FinalizeTestDataUploadResp, error) {
	if in == nil || strings.TrimSpace(in.UploadId) == "" {
		return nil, status.Error(codes.InvalidArgument, "upload_id is required")
	}
	if err := validatePositiveProblemRevision(in.ProblemId, in.ExpectedRevision); err != nil {
		return nil, err
	}
	if !validSHA256String(in.Sha256) {
		return nil, status.Error(codes.InvalidArgument, "sha256 must be 64 hexadecimal characters")
	}
	principal, _, err := authorizeProblem(l.ctx, l.svcCtx, in.ProblemId)
	if err != nil {
		return nil, err
	}
	if l.svcCtx.TestDataStorage == nil {
		return nil, invalidDependency("test data storage")
	}
	upload, err := l.svcCtx.ProblemModel.GetTestDataUpload(l.ctx, in.UploadId)
	if err != nil {
		return nil, mapModelError(err)
	}
	if upload.ProblemID != in.ProblemId {
		return nil, status.Error(codes.FailedPrecondition, "test data upload belongs to another problem")
	}
	if !strings.EqualFold(strings.TrimSpace(upload.SHA256), strings.TrimSpace(in.Sha256)) {
		return nil, status.Error(codes.FailedPrecondition, "test data upload checksum does not match")
	}
	if upload.Status == model.TestDataUploadStatusFinalized {
		result, err := l.svcCtx.ProblemModel.FinalizeTestDataUpload(l.ctx, model.FinalizeTestDataUploadInput{
			ProblemID: in.ProblemId, UploadID: in.UploadId,
			SHA256: in.Sha256, ContentLength: upload.ContentLength,
			ExpectedRevision: in.ExpectedRevision, ActorID: principal.UserID,
		})
		if err != nil {
			return nil, mapModelError(err)
		}
		return finalizeUploadResponse(result), nil
	}
	if upload.Status != model.TestDataUploadStatusPending {
		return nil, status.Error(codes.FailedPrecondition, "test data upload is not pending")
	}
	if !time.Now().UTC().Before(upload.ExpiresAt) {
		return nil, status.Error(codes.FailedPrecondition, "test data upload expired")
	}
	object, err := l.svcCtx.TestDataStorage.InspectUpload(l.ctx, upload)
	if err != nil {
		switch {
		case errors.Is(err, svc.ErrTestDataObjectNotFound):
			return nil, status.Error(codes.FailedPrecondition, "uploaded test data object was not found")
		case errors.Is(err, svc.ErrTestDataObjectInvalid):
			return nil, status.Error(codes.InvalidArgument, "uploaded test data package is invalid")
		default:
			l.Errorf("inspect upload %s: %v", upload.UploadID, err)
			return nil, invalidDependency("test data storage")
		}
	}
	if object == nil || strings.TrimSpace(object.PackageKey) == "" || object.ContentLength != upload.ContentLength ||
		!strings.EqualFold(strings.TrimSpace(object.SHA256), strings.TrimSpace(upload.SHA256)) ||
		!strings.EqualFold(strings.TrimSpace(object.SHA256), strings.TrimSpace(in.Sha256)) {
		return nil, status.Error(codes.FailedPrecondition, "uploaded test data size or checksum does not match")
	}
	if err := validateTestCaseMetadata(object.TestCases); err != nil {
		return nil, err
	}
	result, err := l.svcCtx.ProblemModel.FinalizeTestDataUpload(l.ctx, model.FinalizeTestDataUploadInput{
		ProblemID: in.ProblemId, UploadID: in.UploadId,
		SHA256: in.Sha256, ContentLength: object.ContentLength,
		PackageKey:       object.PackageKey,
		ExpectedRevision: in.ExpectedRevision, ActorID: principal.UserID,
		TestCases: append([]model.TestCaseInput(nil), object.TestCases...),
	})
	if err != nil {
		return nil, mapModelError(err)
	}
	return finalizeUploadResponse(result), nil
}

func finalizeUploadResponse(result *model.FinalizeTestDataUploadResult) *pb.FinalizeTestDataUploadResp {
	testCases := make([]*pb.TestCaseInfo, 0, len(result.TestCases))
	for _, testCase := range result.TestCases {
		testCases = append(testCases, testCaseProto(testCase))
	}
	return &pb.FinalizeTestDataUploadResp{
		TestDataVersion: result.TestDataVersion,
		Revision:        result.Revision,
		TestCases:       testCases,
	}
}
