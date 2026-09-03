package problem

import (
	"context"
	"testing"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/problemservice"
	"google.golang.org/grpc"
)

type problemClientStub struct {
	problemservice.ProblemService
	listCalled bool
	updateReq  *pb.UpdateProblemReq
}

func (s *problemClientStub) ListProblemsForManagement(
	_ context.Context, _ *pb.ListProblemsReq, _ ...grpc.CallOption,
) (*pb.ListProblemsResp, error) {
	s.listCalled = true
	return &pb.ListProblemsResp{Total: 1, Problems: []*pb.ProblemSummary{{ProblemId: 7, Title: "draft"}}}, nil
}

func (s *problemClientStub) UpdateProblem(
	_ context.Context, req *pb.UpdateProblemReq, _ ...grpc.CallOption,
) (*pb.UpdateProblemResp, error) {
	s.updateReq = req
	return &pb.UpdateProblemResp{Problem: &pb.Problem{ProblemId: req.ProblemId, Revision: 2}}, nil
}

func TestAdminListProblemsUsesManagementRPC(t *testing.T) {
	client := new(problemClientStub)
	response, err := listProblems(context.Background(), &svc.ServiceContext{Problems: client}, &types.ListProblemsReq{})
	if err != nil || response.Total != 1 || !client.listCalled {
		t.Fatalf("listProblems() = (%+v, %v), called=%t", response, err, client.listCalled)
	}
}

func TestUpdateProblemForwardsPartialPatchAndMask(t *testing.T) {
	client := new(problemClientStub)
	difficulty := "easy"
	_, err := updateProblem(context.Background(), &svc.ServiceContext{Problems: client}, &types.UpdateProblemReq{
		ProblemID: 7, Problem: types.ProblemPatch{Difficulty: &difficulty},
		UpdateMask: []string{"difficulty"}, ExpectedRevision: 1,
	})
	if err != nil {
		t.Fatalf("updateProblem() error = %v", err)
	}
	if client.updateReq == nil || client.updateReq.Problem.Statement != nil ||
		len(client.updateReq.UpdateMask.Paths) != 1 || client.updateReq.UpdateMask.Paths[0] != "difficulty" {
		t.Fatalf("UpdateProblem request = %+v", client.updateReq)
	}
}
