package contest

import (
	"context"
	"testing"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/contestservice"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"
	"google.golang.org/grpc"
)

type contestClientStub struct {
	contestservice.ContestService
	listCalled bool
	updateReq  *pb.UpdateContestReq
}

func (s *contestClientStub) ListContestsForManagement(
	_ context.Context, _ *pb.ListContestsReq, _ ...grpc.CallOption,
) (*pb.ListContestsResp, error) {
	s.listCalled = true
	return &pb.ListContestsResp{Total: 1, Contests: []*pb.ContestSummary{{ContestId: 8, Title: "draft"}}}, nil
}

func (s *contestClientStub) ReviewRegistration(
	_ context.Context, req *pb.ReviewRegistrationReq, _ ...grpc.CallOption,
) (*pb.ReviewRegistrationResp, error) {
	return &pb.ReviewRegistrationResp{Registration: &pb.ContestRegistration{
		ContestId: req.ContestId, RegistrationId: req.RegistrationId,
	}}, nil
}

func (s *contestClientStub) UpdateContest(
	_ context.Context, req *pb.UpdateContestReq, _ ...grpc.CallOption,
) (*pb.UpdateContestResp, error) {
	s.updateReq = req
	return &pb.UpdateContestResp{Contest: &pb.Contest{ContestId: req.ContestId, Revision: 2}}, nil
}

func TestAdminListContestsUsesManagementRPC(t *testing.T) {
	client := new(contestClientStub)
	response, err := listContests(context.Background(), &svc.ServiceContext{Contests: client}, &types.ListContestsReq{})
	if err != nil || response.Total != 1 || !client.listCalled {
		t.Fatalf("listContests() = (%+v, %v), called=%t", response, err, client.listCalled)
	}
}

func TestRejectedRegistrationKeepsParticipantNil(t *testing.T) {
	client := new(contestClientStub)
	response, err := reviewRegistration(context.Background(), &svc.ServiceContext{Contests: client}, &types.ReviewRegistrationReq{
		ContestID: 8, RegistrationID: 9, Decision: "reject", ExpectedRevision: 1,
	})
	if err != nil {
		t.Fatalf("reviewRegistration() error = %v", err)
	}
	if response.Participant != nil {
		t.Fatalf("participant = %+v, want nil", response.Participant)
	}
}

func TestUpdateContestTranslatesFreezeDurationMask(t *testing.T) {
	client := new(contestClientStub)
	beforeEnd := int64(1800)
	_, err := updateContest(context.Background(), &svc.ServiceContext{Contests: client}, &types.UpdateContestReq{
		ContestID: 8,
		Contest: types.ContestPatch{
			ScoreboardFreezeBeforeEndSec: &beforeEnd,
		},
		UpdateMask:       []string{"scoreboard_freeze_before_end_seconds"},
		ExpectedRevision: 1,
	})
	if err != nil {
		t.Fatalf("updateContest() error = %v", err)
	}
	if client.updateReq == nil || client.updateReq.UpdateMask == nil ||
		len(client.updateReq.UpdateMask.Paths) != 1 ||
		client.updateReq.UpdateMask.Paths[0] != "scoreboard_freeze_before_end" {
		t.Fatalf("UpdateContest request = %+v", client.updateReq)
	}
	if _, ok := client.updateReq.Contest.ScoreboardFreezePolicy.(*pb.ContestEditableFields_ScoreboardFreezeBeforeEnd); !ok {
		t.Fatalf("freeze policy type = %T", client.updateReq.Contest.ScoreboardFreezePolicy)
	}
}
