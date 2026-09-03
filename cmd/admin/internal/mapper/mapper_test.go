package mapper

import (
	"testing"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/types"
	contestpb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"
	problempb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestProblemPatchKeepsUnspecifiedFieldsAbsent(t *testing.T) {
	difficulty := "easy"
	patch, err := ProblemPatch(types.ProblemPatch{Difficulty: &difficulty}, []string{"difficulty"})
	if err != nil {
		t.Fatalf("ProblemPatch() error = %v", err)
	}
	if patch.Statement != nil || patch.JudgeSettings != nil {
		t.Fatalf("unspecified nested fields were populated: %+v", patch)
	}
	if patch.Difficulty != problempb.ProblemDifficulty_PROBLEM_DIFFICULTY_EASY {
		t.Fatalf("difficulty = %s", patch.Difficulty)
	}
}

func TestContestPatchRejectsTwoFreezePolicies(t *testing.T) {
	freezeAt := "2026-09-03T10:00:00Z"
	beforeEnd := int64(1800)
	_, err := ContestPatch(types.ContestPatch{
		ScoreboardFreezeAt: &freezeAt, ScoreboardFreezeBeforeEndSec: &beforeEnd,
	}, []string{"scoreboard_freeze_at", "scoreboard_freeze_before_end"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("ContestPatch() code = %s, want %s", status.Code(err), codes.InvalidArgument)
	}
}

func TestContestPatchBuildsDurationFreezePolicy(t *testing.T) {
	beforeEnd := int64(1800)
	patch, err := ContestPatch(
		types.ContestPatch{ScoreboardFreezeBeforeEndSec: &beforeEnd},
		[]string{"scoreboard_freeze_before_end"},
	)
	if err != nil {
		t.Fatalf("ContestPatch() error = %v", err)
	}
	if _, ok := patch.ScoreboardFreezePolicy.(*contestpb.ContestEditableFields_ScoreboardFreezeBeforeEnd); !ok {
		t.Fatalf("freeze policy type = %T", patch.ScoreboardFreezePolicy)
	}
}

func TestProblemPatchIgnoresFieldsOutsideMask(t *testing.T) {
	invalidDifficulty := "not-a-difficulty"
	source := "updated source"
	patch, err := ProblemPatch(types.ProblemPatch{
		Difficulty: &invalidDifficulty,
		Source:     &source,
	}, []string{"source"})
	if err != nil {
		t.Fatalf("ProblemPatch() error = %v", err)
	}
	if patch.Source != source || patch.Difficulty != problempb.ProblemDifficulty_PROBLEM_DIFFICULTY_UNSPECIFIED {
		t.Fatalf("ProblemPatch() = %+v", patch)
	}
}

func TestContestPatchIgnoresFreezePolicyOutsideMask(t *testing.T) {
	freezeAt := "2026-09-03T10:00:00Z"
	beforeEnd := int64(1800)
	patch, err := ContestPatch(types.ContestPatch{
		ScoreboardFreezeAt: &freezeAt, ScoreboardFreezeBeforeEndSec: &beforeEnd,
	}, []string{"scoreboard_freeze_at"})
	if err != nil {
		t.Fatalf("ContestPatch() error = %v", err)
	}
	if _, ok := patch.ScoreboardFreezePolicy.(*contestpb.ContestEditableFields_ScoreboardFreezeAt); !ok {
		t.Fatalf("freeze policy type = %T", patch.ScoreboardFreezePolicy)
	}
}

func TestContestUpdateMaskTranslatesDurationField(t *testing.T) {
	mask := ContestUpdateMask([]string{"title", "scoreboard_freeze_before_end_seconds"})
	if len(mask) != 2 || mask[0] != "title" || mask[1] != "scoreboard_freeze_before_end" {
		t.Fatalf("ContestUpdateMask() = %v", mask)
	}
}
