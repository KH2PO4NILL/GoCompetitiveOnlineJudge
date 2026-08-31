package svc

import (
	"context"
	"errors"

	sharedauth "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"
	contestpb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"google.golang.org/grpc"
)

// ContestAccessChecker is the Problem RPC boundary for contest membership and
// problem assignment checks. The Contest RPC implementation can satisfy this
// interface once its protocol is defined, while Logic remains independently
// testable in the meantime.
type ContestAccessChecker interface {
	CanAccessProblem(ctx context.Context, contestID, problemID, userID int64, requesterIsAdmin bool) (bool, error)
}

type contestAccessClient interface {
	CanAccessProblem(
		ctx context.Context,
		in *contestpb.CanAccessProblemReq,
		opts ...grpc.CallOption,
	) (*contestpb.CanAccessProblemResp, error)
}

type contestAccessRPCChecker struct {
	client     contestAccessClient
	credential sharedauth.ServiceClientAuthConf
}

func NewContestAccessChecker(
	connection grpc.ClientConnInterface,
	credential sharedauth.ServiceClientAuthConf,
) ContestAccessChecker {
	return &contestAccessRPCChecker{
		client:     contestpb.NewContestServiceClient(connection),
		credential: credential,
	}
}

func (c *contestAccessRPCChecker) CanAccessProblem(
	ctx context.Context,
	contestID, problemID, userID int64,
	requesterIsAdmin bool,
) (bool, error) {
	if c == nil || c.client == nil {
		return false, errors.New("contest RPC client is unavailable")
	}
	if c.credential.Name != "" || c.credential.Token != "" {
		ctx = sharedauth.WithOutgoingServiceCredentials(ctx, c.credential.Name, c.credential.Token)
	}
	response, err := c.client.CanAccessProblem(ctx, &contestpb.CanAccessProblemReq{
		ContestId:        contestID,
		ProblemId:        problemID,
		UserId:           userID,
		RequesterIsAdmin: requesterIsAdmin,
	})
	if err != nil {
		return false, err
	}
	if response == nil {
		return false, errors.New("contest RPC returned an empty response")
	}
	return response.Allowed, nil
}
