package model

import (
	"errors"
	"testing"
	"time"
)

func TestEffectiveContestStatusUsesTimeWithoutPersistingDerivedStates(t *testing.T) {
	start := time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)
	tests := []struct {
		name      string
		persisted int64
		at        time.Time
		want      int64
	}{
		{name: "scheduled", persisted: ContestStatusPublished, at: start.Add(-time.Second), want: ContestStatusPublished},
		{name: "running at start", persisted: ContestStatusPublished, at: start, want: ContestStatusRunning},
		{name: "running", persisted: ContestStatusPublished, at: start.Add(time.Hour), want: ContestStatusRunning},
		{name: "finished at end", persisted: ContestStatusPublished, at: end, want: ContestStatusFinished},
		{name: "cancelled wins", persisted: ContestStatusCancelled, at: start.Add(time.Hour), want: ContestStatusCancelled},
		{name: "archived wins", persisted: ContestStatusArchived, at: end.Add(time.Hour), want: ContestStatusArchived},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := EffectiveContestStatus(test.persisted, start, end, test.at); got != test.want {
				t.Fatalf("EffectiveContestStatus() = %d, want %d", got, test.want)
			}
		})
	}
}

func TestValidateEditableContestEnforcesDomainBoundaries(t *testing.T) {
	valid := testEditableContest()
	tests := []struct {
		name   string
		mutate func(*EditableContest)
	}{
		{name: "empty title", mutate: func(value *EditableContest) { value.Title = " " }},
		{name: "invalid mode", mutate: func(value *EditableContest) { value.Mode = 0 }},
		{name: "reversed contest window", mutate: func(value *EditableContest) { value.EndAt = value.StartAt }},
		{name: "registration after start", mutate: func(value *EditableContest) {
			value.RegistrationEndAt = timePointer(value.StartAt.Add(time.Second))
		}},
		{name: "non icpc freeze", mutate: func(value *EditableContest) {
			value.Mode = ContestModeIOI
			value.ScoreboardFreezeAt = timePointer(value.StartAt.Add(time.Hour))
		}},
		{name: "invite policy without code", mutate: func(value *EditableContest) {
			value.RegistrationPolicy = RegistrationPolicyInviteOnly
			value.InviteCode = nil
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := valid
			test.mutate(&value)
			if err := validateEditableContest(&value, false); !errors.Is(err, ErrInvalidContestInput) {
				t.Fatalf("validateEditableContest() error = %v", err)
			}
		})
	}
}

func TestContestPatchSwitchesFreezeOneof(t *testing.T) {
	value := testEditableContest()
	freezeAt := value.StartAt.Add(time.Hour)
	value.ScoreboardFreezeAt = &freezeAt
	duration := 30 * time.Minute
	durationPointerValue := &duration
	applyContestPatch(&value, ContestPatch{ScoreboardFreezeBeforeEnd: &durationPointerValue})
	if value.ScoreboardFreezeAt != nil || value.ScoreboardFreezeBeforeEnd == nil || *value.ScoreboardFreezeBeforeEnd != duration {
		t.Fatalf("freeze policy after patch = at:%v before:%v", value.ScoreboardFreezeAt, value.ScoreboardFreezeBeforeEnd)
	}
}

func TestValidateContestProblemsRejectsDuplicatesAndModeMismatch(t *testing.T) {
	_, err := validateContestProblems([]ContestProblem{
		{ProblemID: 1, Label: "A", DisplayOrder: 0},
		{ProblemID: 2, Label: "A", DisplayOrder: 1},
	}, ContestModeICPC)
	if !errors.Is(err, ErrContestProblemConflict) {
		t.Fatalf("duplicate label error = %v", err)
	}
	_, err = validateContestProblems([]ContestProblem{{ProblemID: 1, Label: "A", DisplayOrder: 0, MaxScore: 100}}, ContestModeICPC)
	if !errors.Is(err, ErrInvalidContestInput) {
		t.Fatalf("ICPC max score error = %v", err)
	}
	_, err = validateContestProblems([]ContestProblem{{ProblemID: 1, Label: "A", DisplayOrder: 0}}, ContestModeIOI)
	if !errors.Is(err, ErrInvalidContestInput) {
		t.Fatalf("IOI max score error = %v", err)
	}
}

func TestValidateScoreboardRowsRejectsCrossModeAndDuplicateFirstSolve(t *testing.T) {
	valid := []ScoreboardRow{
		{Rank: 1, Ordinal: 1, UserID: 10, DisplayName: "one", ICPC: &IcpcScore{},
			ProblemResults: []ScoreboardProblemResult{{ProblemID: 1, Label: "A", ICPC: &IcpcProblemResult{FirstToSolve: true}}}},
		{Rank: 2, Ordinal: 2, UserID: 20, DisplayName: "two", ICPC: &IcpcScore{},
			ProblemResults: []ScoreboardProblemResult{{ProblemID: 1, Label: "A", ICPC: &IcpcProblemResult{FirstToSolve: true}}}},
	}
	if _, err := validateScoreboardRows(valid, ContestModeICPC); !errors.Is(err, ErrInvalidContestInput) {
		t.Fatalf("duplicate first solve error = %v", err)
	}
	points := []ScoreboardRow{{Rank: 1, Ordinal: 1, UserID: 10, DisplayName: "one", ICPC: &IcpcScore{}}}
	if _, err := validateScoreboardRows(points, ContestModeIOI); !errors.Is(err, ErrInvalidContestInput) {
		t.Fatalf("cross-mode score error = %v", err)
	}
}

func TestInviteCodeHashNeverStoresPlaintext(t *testing.T) {
	hash, err := hashInviteCode("secret-code")
	if err != nil {
		t.Fatalf("hashInviteCode() error = %v", err)
	}
	if hash == nil || *hash == "secret-code" {
		t.Fatalf("hashInviteCode() returned plaintext: %v", hash)
	}
	if !inviteCodeMatches(hash, "secret-code") || inviteCodeMatches(hash, "wrong-code") {
		t.Fatal("invite code verification did not preserve match semantics")
	}
}

func TestNormalizePageCapsPageSize(t *testing.T) {
	page, size := normalizePage(-1, 1000)
	if page != 1 || size != 100 {
		t.Fatalf("normalizePage() = (%d, %d), want (1, 100)", page, size)
	}
}

func testEditableContest() EditableContest {
	start := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	registrationStart := start.Add(-48 * time.Hour)
	registrationEnd := start.Add(-time.Hour)
	return EditableContest{
		Title: "Weekly Contest", Description: "description", Mode: ContestModeICPC,
		StartAt: start, EndAt: start.Add(2 * time.Hour),
		RegistrationStartAt: &registrationStart, RegistrationEndAt: &registrationEnd,
		RegistrationPolicy: RegistrationPolicyOpen, Visibility: ContestVisibilityPublic,
		ScoreboardVisibility: ScoreboardVisibilityPublic,
	}
}

func timePointer(value time.Time) *time.Time { return &value }
