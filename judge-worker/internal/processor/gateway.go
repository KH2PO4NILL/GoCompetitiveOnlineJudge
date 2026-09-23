package processor

import (
	"context"
	"errors"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/judgepolicy"
	problempb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"
	submitpb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type SubmitGateway interface {
	Acquire(context.Context, *submitpb.AcquireJudgeTaskReq) (*submitpb.AcquireJudgeTaskResp, error)
	Heartbeat(context.Context, *submitpb.HeartbeatJudgeTaskReq) (*submitpb.HeartbeatJudgeTaskResp, error)
	Report(context.Context, *submitpb.ReportJudgeResultReq) (*submitpb.ReportJudgeResultResp, error)
}
type ProblemGateway interface {
	Spec(context.Context, int64, int64) (*problempb.JudgeProblemSpec, error)
}

type RPCSubmit struct {
	Client  submitpb.SubmitServiceClient
	Token   string
	Timeout time.Duration
}
type RPCProblem struct {
	Client  problempb.ProblemServiceClient
	Token   string
	Timeout time.Duration
}

func NewGateways(submitTarget, problemTarget, submitToken, problemToken string, timeout time.Duration) (*RPCSubmit, *RPCProblem, func() error, error) {
	submit, err := grpc.NewClient(submitTarget, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, nil, err
	}
	problem, err := grpc.NewClient(problemTarget, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		_ = submit.Close()
		return nil, nil, nil, err
	}
	return &RPCSubmit{Client: submitpb.NewSubmitServiceClient(submit), Token: submitToken, Timeout: timeout}, &RPCProblem{Client: problempb.NewProblemServiceClient(problem), Token: problemToken, Timeout: timeout}, func() error { return errors.Join(submit.Close(), problem.Close()) }, nil
}

func rpcContext(ctx context.Context, token string, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(interceptor.WithOutgoingServiceCredentials(ctx, "judge.worker", token), timeout)
}
func (gateway *RPCSubmit) Acquire(ctx context.Context, request *submitpb.AcquireJudgeTaskReq) (*submitpb.AcquireJudgeTaskResp, error) {
	ctx, cancel := rpcContext(ctx, gateway.Token, gateway.Timeout)
	defer cancel()
	return gateway.Client.AcquireJudgeTask(ctx, request, grpc.MaxCallRecvMsgSize(judgepolicy.MaxLegacyTaskMessageBytes))
}
func (gateway *RPCSubmit) Heartbeat(ctx context.Context, request *submitpb.HeartbeatJudgeTaskReq) (*submitpb.HeartbeatJudgeTaskResp, error) {
	ctx, cancel := rpcContext(ctx, gateway.Token, gateway.Timeout)
	defer cancel()
	return gateway.Client.HeartbeatJudgeTask(ctx, request)
}
func (gateway *RPCSubmit) Report(ctx context.Context, request *submitpb.ReportJudgeResultReq) (*submitpb.ReportJudgeResultResp, error) {
	ctx, cancel := rpcContext(ctx, gateway.Token, gateway.Timeout)
	defer cancel()
	return gateway.Client.ReportJudgeResult(ctx, request)
}
func (gateway *RPCProblem) Spec(ctx context.Context, id, revision int64) (*problempb.JudgeProblemSpec, error) {
	ctx, cancel := rpcContext(ctx, gateway.Token, gateway.Timeout)
	defer cancel()
	response, err := gateway.Client.GetJudgeProblemSpec(ctx, &problempb.GetJudgeProblemSpecReq{ProblemId: id, Revision: revision})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, errors.New("empty problem response")
	}
	return response.Spec, nil
}
