package logic

import (
	"context"
	"strings"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type CreateTestDataUploadLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateTestDataUploadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateTestDataUploadLogic {
	return &CreateTestDataUploadLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateTestDataUploadLogic) CreateTestDataUpload(in *pb.CreateTestDataUploadReq) (*pb.CreateTestDataUploadResp, error) {
	if in == nil || in.ProblemId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "problem_id must be positive")
	}
	policy := l.svcCtx.EffectiveUploadPolicy()
	if err := policy.ValidateFileName(in.FileName); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if in.ContentLength <= 0 {
		return nil, status.Error(codes.InvalidArgument, "content_length must be positive")
	}
	if in.ContentLength > policy.MaxUploadBytes {
		return nil, status.Error(codes.ResourceExhausted, "test data package is too large")
	}
	if !validSHA256String(in.Sha256) {
		return nil, status.Error(codes.InvalidArgument, "sha256 must be 64 hexadecimal characters")
	}
	if _, _, err := authorizeProblem(l.ctx, l.svcCtx, in.ProblemId); err != nil {
		return nil, err
	}
	if l.svcCtx.TestDataStorage == nil {
		return nil, invalidDependency("test data storage")
	}
	upload, err := l.svcCtx.ProblemModel.CreateTestDataUpload(l.ctx, model.CreateTestDataUploadInput{
		ProblemID: in.ProblemId, FileName: in.FileName,
		ContentLength: in.ContentLength, SHA256: in.Sha256, TTL: policy.URLTTL,
	})
	if err != nil {
		return nil, mapModelError(err)
	}
	uploadURL, err := l.svcCtx.TestDataStorage.CreateUploadURL(l.ctx, upload)
	if err != nil || strings.TrimSpace(uploadURL) == "" {
		l.Errorf("create upload URL for problem %d upload %s: %v", in.ProblemId, upload.UploadID, err)
		return nil, invalidDependency("test data storage")
	}
	return &pb.CreateTestDataUploadResp{
		UploadId: upload.UploadID, UploadUrl: uploadURL,
		ExpiresAt: timestamppb.New(upload.ExpiresAt),
	}, nil
}
