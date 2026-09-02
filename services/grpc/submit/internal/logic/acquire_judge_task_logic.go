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

type AcquireJudgeTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAcquireJudgeTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AcquireJudgeTaskLogic {
	return &AcquireJudgeTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AcquireJudgeTaskLogic) AcquireJudgeTask(in *pb.AcquireJudgeTaskReq) (*pb.AcquireJudgeTaskResp, error) {
	if err := requireJudgeWorker(l.ctx); err != nil {
		return nil, err
	}
	if in == nil || in.SubmissionId <= 0 || in.AttemptId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "submission_id and attempt_id must be positive")
	}
	workerID, err := normalizeWorkerIdentity(in.WorkerId, 128, "worker_id", true)
	if err != nil {
		return nil, err
	}
	deliveryID, err := normalizeWorkerIdentity(in.DeliveryId, 255, "delivery_id", false)
	if err != nil {
		return nil, err
	}
	repository, err := requireSubmitModel(l.svcCtx)
	if err != nil {
		return nil, err
	}
	result, err := repository.AcquireAttempt(l.ctx, model.AcquireAttemptInput{
		WorkerID: workerID, SubmissionID: in.SubmissionId, AttemptID: in.AttemptId,
		DeliveryID: deliveryID, LeaseDuration: leaseDuration(l.svcCtx),
	})
	if err != nil {
		return nil, mapModelError(err)
	}
	if result == nil {
		return nil, status.Error(codes.Internal, "submit model returned an empty acquire response")
	}
	if !result.Acquired {
		denyReason, err := acquireDenyReason(result.DenyReason)
		if err != nil {
			return nil, err
		}
		return &pb.AcquireJudgeTaskResp{Acquired: false, DenyReason: denyReason}, nil
	}
	if result.DenyReason != model.AcquireDenyNone || result.ProblemID <= 0 || result.ProblemRevision <= 0 ||
		result.Language == "" || result.SourceCode == "" || result.LeaseToken == "" || result.LeaseExpiresAt == nil {
		return nil, status.Error(codes.Internal, "submit model returned an invalid acquire response")
	}
	expiresAt, err := timestamp(*result.LeaseExpiresAt, "lease_expires_at")
	if err != nil {
		return nil, err
	}
	return &pb.AcquireJudgeTaskResp{
		Acquired: true, DenyReason: pb.AcquireDenyReason_ACQUIRE_DENY_REASON_UNSPECIFIED,
		Task: &pb.JudgeTask{
			SubmissionId: result.SubmissionID, AttemptId: result.AttemptID,
			ProblemId: result.ProblemID, ProblemRevision: result.ProblemRevision,
			Language: result.Language, SourceCode: result.SourceCode,
		},
		LeaseToken: result.LeaseToken, LeaseExpiresAt: expiresAt,
	}, nil
}

func acquireDenyReason(value int64) (pb.AcquireDenyReason, error) {
	switch value {
	case model.AcquireDenyAlreadyFinished:
		return pb.AcquireDenyReason_ACQUIRE_DENY_REASON_ALREADY_FINISHED, nil
	case model.AcquireDenyStaleAttempt:
		return pb.AcquireDenyReason_ACQUIRE_DENY_REASON_STALE_ATTEMPT, nil
	case model.AcquireDenyLeasedByOtherWorker:
		return pb.AcquireDenyReason_ACQUIRE_DENY_REASON_LEASED_BY_OTHER_WORKER, nil
	default:
		return 0, status.Error(codes.Internal, "submit model returned an invalid acquire denial")
	}
}
