package svc

import (
	"context"
	"errors"

	sharedauth "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"
	problempb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"

	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
)

// ProblemSpecCatalog is the only Problem RPC boundary used by Submit logic.
// Implementations must complete the network call before SubmitModel starts a
// transaction.
type ProblemSpecCatalog interface {
	GetJudgeProblemSpec(ctx context.Context, problemID, revision int64) (*problempb.JudgeProblemSpec, error)
}

type problemSpecClient interface {
	GetJudgeProblemSpec(
		ctx context.Context,
		in *problempb.GetJudgeProblemSpecReq,
		opts ...grpc.CallOption,
	) (*problempb.GetJudgeProblemSpecResp, error)
}

type rpcProblemSpecCatalog struct {
	client     problemSpecClient
	credential sharedauth.ServiceClientAuthConf
}

func NewRPCProblemSpecCatalog(
	connection grpc.ClientConnInterface,
	credential sharedauth.ServiceClientAuthConf,
) ProblemSpecCatalog {
	// JudgeProblemSpec contains private object-store keys and must never be
	// included in client-side RPC content logs.
	zrpc.DontLogClientContentForMethod(problempb.ProblemService_GetJudgeProblemSpec_FullMethodName)
	return &rpcProblemSpecCatalog{
		client:     problempb.NewProblemServiceClient(connection),
		credential: credential,
	}
}

func (c *rpcProblemSpecCatalog) GetJudgeProblemSpec(
	ctx context.Context,
	problemID, revision int64,
) (*problempb.JudgeProblemSpec, error) {
	if c == nil || c.client == nil {
		return nil, errors.New("problem RPC client is unavailable")
	}
	ctx = sharedauth.WithOutgoingServiceCredentials(ctx, c.credential.Name, c.credential.Token)
	response, err := c.client.GetJudgeProblemSpec(ctx, &problempb.GetJudgeProblemSpecReq{
		ProblemId: problemID,
		Revision:  revision,
	})
	if err != nil {
		return nil, err
	}
	if response == nil || response.Spec == nil {
		return nil, errors.New("problem RPC returned an empty judge spec")
	}
	return response.Spec, nil
}
