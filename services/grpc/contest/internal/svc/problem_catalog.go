package svc

import (
	"context"
	"errors"
	"fmt"

	sharedauth "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"
	problempb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"
	problemservice "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/problemservice"
)

var ErrInvalidProblemCatalogResponse = errors.New("invalid problem catalog response")

// ProblemCatalog is the Contest RPC boundary for validating cross-service
// problem IDs. Implementations must not call ContestModel from inside this
// method, so the network lookup always completes before a Contest transaction.
type ProblemCatalog interface {
	ListPublishedForContest(ctx context.Context, problemIDs []int64) ([]int64, error)
}

type rpcProblemCatalog struct {
	client problemservice.ProblemService
	auth   sharedauth.ServiceClientAuthConf
}

func NewRPCProblemCatalog(
	client problemservice.ProblemService,
	auth sharedauth.ServiceClientAuthConf,
) ProblemCatalog {
	return &rpcProblemCatalog{client: client, auth: auth}
}

func (c *rpcProblemCatalog) ListPublishedForContest(
	ctx context.Context,
	problemIDs []int64,
) ([]int64, error) {
	if c == nil || c.client == nil {
		return nil, errors.New("problem catalog client is unavailable")
	}
	ctx = sharedauth.WithOutgoingServiceCredentials(ctx, c.auth.Name, c.auth.Token)
	response, err := c.client.BatchGetProblems(ctx, &problempb.BatchGetProblemsReq{ProblemIds: problemIDs})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, fmt.Errorf("%w: empty response", ErrInvalidProblemCatalogResponse)
	}
	result := make([]int64, 0, len(response.Problems))
	seen := make(map[int64]struct{}, len(response.Problems))
	for _, problem := range response.Problems {
		if problem == nil || problem.ProblemId <= 0 ||
			problem.Status != problempb.ProblemStatus_PROBLEM_STATUS_PUBLISHED ||
			(problem.Visibility != problempb.ProblemVisibility_PROBLEM_VISIBILITY_PUBLIC &&
				problem.Visibility != problempb.ProblemVisibility_PROBLEM_VISIBILITY_CONTEST_ONLY) {
			return nil, fmt.Errorf("%w: unexpected problem entry", ErrInvalidProblemCatalogResponse)
		}
		if _, exists := seen[problem.ProblemId]; exists {
			return nil, fmt.Errorf("%w: duplicate problem entry", ErrInvalidProblemCatalogResponse)
		}
		seen[problem.ProblemId] = struct{}{}
		result = append(result, problem.ProblemId)
	}
	return result, nil
}
