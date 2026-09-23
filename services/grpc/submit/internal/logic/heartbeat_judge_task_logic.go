package logic

import (
	"context"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type HeartbeatJudgeTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewHeartbeatJudgeTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HeartbeatJudgeTaskLogic {
	return &HeartbeatJudgeTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *HeartbeatJudgeTaskLogic) HeartbeatJudgeTask(in *pb.HeartbeatJudgeTaskReq) (*pb.HeartbeatJudgeTaskResp, error) {
	if err := requireJudgeWorker(l.ctx); err != nil {
		return nil, err
	}
	if in == nil || in.SubmissionId <= 0 || in.AttemptId <= 0 || in.CompletedTestCases < 0 {
		return nil, status.Error(codes.InvalidArgument, "heartbeat request is invalid")
	}
	switch in.CurrentStage {
	case "", "spec", "download", "compile", "run", "done", "report":
	default:
		return nil, status.Error(codes.InvalidArgument, "heartbeat stage is invalid")
	}
	if int64(in.CompletedTestCases) > int64(runtimeLimits(l.svcCtx).MaxCaseResults) {
		return nil, status.Error(codes.InvalidArgument, "completed_test_cases exceeds the service limit")
	}
	workerID, err := normalizeWorkerIdentity(in.WorkerId, 128, "worker_id", true)
	if err != nil {
		return nil, err
	}
	leaseToken, err := normalizeWorkerIdentity(in.LeaseToken, 256, "lease_token", true)
	if err != nil {
		return nil, err
	}
	repository, err := requireSubmitModel(l.svcCtx)
	if err != nil {
		return nil, err
	}
	result, err := repository.HeartbeatAttempt(l.ctx, model.HeartbeatAttemptInput{
		WorkerID: workerID, SubmissionID: in.SubmissionId, AttemptID: in.AttemptId,
		LeaseToken: leaseToken, CompletedTestCases: in.CompletedTestCases,
		LeaseDuration: leaseDuration(l.svcCtx),
	})
	if err != nil {
		return nil, mapModelError(err)
	}
	if result == nil {
		return nil, status.Error(codes.Internal, "submit model returned an empty heartbeat response")
	}
	expiresAt, err := timestamp(result.LeaseExpiresAt, "lease_expires_at")
	if err != nil {
		return nil, err
	}
	return &pb.HeartbeatJudgeTaskResp{LeaseExpiresAt: expiresAt}, nil
}
