package logic

import (
	"context"
	"errors"
	"testing"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/model"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type stubProblemModel struct {
	model.ProblemModel

	revision int64
	err      error

	gotProblemID        int64
	gotExpectedRevision int64
}

func (m *stubProblemModel) SoftDelete(
	_ context.Context,
	problemID int64,
	expectedRevision int64,
) (int64, error) {
	m.gotProblemID = problemID
	m.gotExpectedRevision = expectedRevision
	return m.revision, m.err
}

func TestDeleteProblemReturnsNewRevision(t *testing.T) {
	repository := &stubProblemModel{revision: 8}
	logic := NewDeleteProblemLogic(context.Background(), &svc.ServiceContext{
		ProblemModel: repository,
	})

	resp, err := logic.DeleteProblem(&pb.DeleteProblemReq{
		ProblemId:        42,
		ExpectedRevision: 7,
	})
	if err != nil {
		t.Fatalf("DeleteProblem() error = %v", err)
	}
	if resp.Revision != 8 {
		t.Fatalf("DeleteProblem() revision = %d, want 8", resp.Revision)
	}
	if repository.gotProblemID != 42 || repository.gotExpectedRevision != 7 {
		t.Fatalf(
			"SoftDelete() args = (%d, %d), want (42, 7)",
			repository.gotProblemID,
			repository.gotExpectedRevision,
		)
	}
}

func TestDeleteProblemMapsModelErrors(t *testing.T) {
	tests := []struct {
		name     string
		modelErr error
		wantCode codes.Code
	}{
		{name: "not found", modelErr: model.ErrProblemNotFound, wantCode: codes.NotFound},
		{name: "revision conflict", modelErr: model.ErrProblemRevisionConflict, wantCode: codes.Aborted},
		{name: "already deleted", modelErr: model.ErrProblemAlreadyDeleted, wantCode: codes.FailedPrecondition},
		{name: "database failure", modelErr: errors.New("database unavailable"), wantCode: codes.Internal},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			logic := NewDeleteProblemLogic(context.Background(), &svc.ServiceContext{
				ProblemModel: &stubProblemModel{err: test.modelErr},
			})

			_, err := logic.DeleteProblem(&pb.DeleteProblemReq{
				ProblemId:        42,
				ExpectedRevision: 7,
			})
			if status.Code(err) != test.wantCode {
				t.Fatalf("DeleteProblem() code = %s, want %s", status.Code(err), test.wantCode)
			}
		})
	}
}

func TestDeleteProblemValidatesRequest(t *testing.T) {
	tests := []*pb.DeleteProblemReq{
		nil,
		{ProblemId: 0, ExpectedRevision: 1},
		{ProblemId: 1, ExpectedRevision: 0},
	}

	for _, req := range tests {
		logic := NewDeleteProblemLogic(context.Background(), &svc.ServiceContext{})
		_, err := logic.DeleteProblem(req)
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("DeleteProblem(%v) code = %s, want %s", req, status.Code(err), codes.InvalidArgument)
		}
	}
}
