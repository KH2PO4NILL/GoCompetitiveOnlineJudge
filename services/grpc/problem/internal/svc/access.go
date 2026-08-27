package svc

import (
	"context"
	"errors"

	contestpb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"google.golang.org/grpc"
)

// ContestAccessChecker is the Problem RPC boundary for contest membership and
// problem assignment checks. The Contest RPC implementation can satisfy this
// interface once its protocol is defined, while Logic remains independently
// testable in the meantime.
type ContestAccessChecker interface {
	CanAccessProblem(ctx context.Context, contestID, problemID, userID int64) (bool, error)
}

type contestAccessClient interface {
	CanAccessProblem(
		ctx context.Context,
		in *contestpb.CanAccessProblemReq,
		opts ...grpc.CallOption,
	) (*contestpb.CanAccessProblemResp, error)
}

type contestAccessRPCChecker struct {
	client contestAccessClient
}

func NewContestAccessChecker(connection grpc.ClientConnInterface) ContestAccessChecker {
	return &contestAccessRPCChecker{client: contestpb.NewContestServiceClient(connection)}
}

func (c *contestAccessRPCChecker) CanAccessProblem(
	ctx context.Context,
	contestID, problemID, userID int64,
) (bool, error) {
	if c == nil || c.client == nil {
		return false, errors.New("contest RPC client is unavailable")
	}
	response, err := c.client.CanAccessProblem(ctx, &contestpb.CanAccessProblemReq{
		ContestId: contestID,
		ProblemId: problemID,
		UserId:    userID,
	})
	if err != nil {
		return false, err
	}
	if response == nil {
		return false, errors.New("contest RPC returned an empty response")
	}
	return response.Allowed, nil
}
