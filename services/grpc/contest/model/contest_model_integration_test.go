package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestContestModelLifecycleProblemsAndRegistration(t *testing.T) {
	repository, _ := newContestModelTestDatabase(t)
	ctx := context.Background()
	current := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	repository.now = func() time.Time { return current }
	start := current.Add(4 * time.Hour)
	registrationStart := current.Add(-time.Hour)
	registrationEnd := current.Add(3 * time.Hour)

	created, err := repository.Create(ctx, CreateContestInput{CreatorID: 10, Editable: EditableContest{
		Title: "ICPC Round", Description: "contest", Mode: ContestModeICPC,
		StartAt: start, EndAt: start.Add(2 * time.Hour), RegistrationStartAt: &registrationStart,
		RegistrationEndAt: &registrationEnd, RegistrationPolicy: RegistrationPolicyOpen,
		Visibility: ContestVisibilityPublic, ScoreboardVisibility: ScoreboardVisibilityPublic,
	}})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Revision != 1 || created.Status != ContestStatusDraft {
		t.Fatalf("created contest = %+v", created)
	}
	updated, problems, err := repository.SetProblems(ctx, SetContestProblemsInput{
		ContestID: created.ContestID, ExpectedRevision: 1,
		Problems: []ContestProblem{{ProblemID: 101, Label: "A", DisplayOrder: 0}, {ProblemID: 102, Label: "B", DisplayOrder: 1}},
	})
	if err != nil {
		t.Fatalf("SetProblems() error = %v", err)
	}
	if updated.Revision != 2 || len(problems) != 2 {
		t.Fatalf("SetProblems() = revision %d, problems %+v", updated.Revision, problems)
	}
	_, _, err = repository.SetProblems(ctx, SetContestProblemsInput{
		ContestID: created.ContestID, ExpectedRevision: 2,
		Problems: []ContestProblem{{ProblemID: 201, Label: "A", DisplayOrder: 0}, {ProblemID: 202, Label: "A", DisplayOrder: 1}},
	})
	if !errors.Is(err, ErrContestProblemConflict) {
		t.Fatalf("duplicate SetProblems() error = %v", err)
	}
	remaining, err := repository.ListProblems(ctx, created.ContestID)
	if err != nil || len(remaining) != 2 || remaining[0].ProblemID != 101 {
		t.Fatalf("problems changed after rejected replacement: rows=%+v err=%v", remaining, err)
	}

	published, err := repository.Publish(ctx, created.ContestID, 2)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if published.Revision != 3 || published.EffectiveStatus != ContestStatusPublished {
		t.Fatalf("published contest = %+v", published)
	}
	public, total, err := repository.ListPublic(ctx, ContestListFilter{At: current})
	if err != nil || total != 1 || len(public) != 1 || public[0].Status != ContestStatusPublished {
		t.Fatalf("ListPublic() total=%d rows=%+v err=%v", total, public, err)
	}

	registration, participant, err := repository.Register(ctx, RegisterContestInput{
		ContestID: created.ContestID, UserID: 20, ApplicationNote: "ready", At: current,
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if registration.Status != RegistrationStatusApproved || participant == nil || participant.Status != ParticipantStatusNormal {
		t.Fatalf("open registration = %+v participant=%+v", registration, participant)
	}
	if _, _, err := repository.Register(ctx, RegisterContestInput{ContestID: created.ContestID, UserID: 20, At: current}); !errors.Is(err, ErrRegistrationAlreadyExists) {
		t.Fatalf("duplicate Register() error = %v", err)
	}
	registerErrs := make([]error, 2)
	registerResults := make([]*ContestRegistration, 2)
	registerStart := make(chan struct{})
	registerDone := make(chan int, 2)
	for i := range registerErrs {
		go func(index int) {
			<-registerStart
			registerResults[index], _, registerErrs[index] = repository.Register(ctx, RegisterContestInput{
				ContestID: created.ContestID, UserID: 21, At: current,
			})
			registerDone <- index
		}(i)
	}
	close(registerStart)
	<-registerDone
	<-registerDone
	registerSuccesses, registerConflicts := 0, 0
	for i := range registerErrs {
		switch {
		case registerErrs[i] == nil:
			registerSuccesses++
			if registerResults[i].Status != RegistrationStatusApproved {
				t.Fatalf("concurrent registration result = %+v", registerResults[i])
			}
		case errors.Is(registerErrs[i], ErrRegistrationAlreadyExists):
			registerConflicts++
		default:
			t.Fatalf("concurrent Register() error = %v", registerErrs[i])
		}
	}
	if registerSuccesses != 1 || registerConflicts != 1 {
		t.Fatalf("concurrent registrations successes=%d conflicts=%d", registerSuccesses, registerConflicts)
	}
	access, err := repository.GetAccessSnapshot(ctx, created.ContestID, 101, 20, current)
	if err != nil || !access.ProblemAssigned || access.ParticipantStatus == nil || *access.ParticipantStatus != ParticipantStatusNormal {
		t.Fatalf("GetAccessSnapshot() = %+v err=%v", access, err)
	}
	managerAccess, err := repository.GetAccessSnapshot(ctx, created.ContestID, 101, created.CreatorID, current)
	if err != nil || !managerAccess.Manager || !managerAccess.ProblemAssigned {
		t.Fatalf("GetAccessSnapshot(manager) = %+v err=%v", managerAccess, err)
	}
	detailBeforeStart, err := repository.GetDetail(ctx, created.ContestID, current)
	if err != nil || detailBeforeStart.ParticipantCount != 2 {
		t.Fatalf("participant counter = %+v err=%v", detailBeforeStart, err)
	}

	titles := []string{"Concurrent A", "Concurrent B"}
	errs := make([]error, 2)
	results := make([]*ContestAggregate, 2)
	startUpdates := make(chan struct{})
	done := make(chan int, 2)
	for i := range titles {
		go func(index int) {
			<-startUpdates
			results[index], errs[index] = repository.Update(ctx, UpdateContestInput{
				ContestID: created.ContestID, ExpectedRevision: 3, Patch: ContestPatch{Title: &titles[index]},
			})
			done <- index
		}(i)
	}
	close(startUpdates)
	<-done
	<-done
	successes, conflicts := 0, 0
	for i := range errs {
		switch {
		case errs[i] == nil:
			successes++
			if results[i].Revision != 4 {
				t.Fatalf("concurrent success revision = %d", results[i].Revision)
			}
		case errors.Is(errs[i], ErrContestRevisionConflict):
			conflicts++
		default:
			t.Fatalf("concurrent Update() error = %v", errs[i])
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent updates successes=%d conflicts=%d", successes, conflicts)
	}

	current = start.Add(30 * time.Minute)
	detail, err := repository.GetDetail(ctx, created.ContestID, current)
	if err != nil || detail.Status != ContestStatusRunning {
		t.Fatalf("running GetDetail() = %+v err=%v", detail, err)
	}
	if _, err := repository.SoftDelete(ctx, created.ContestID, 4); !errors.Is(err, ErrContestStateConflict) {
		t.Fatalf("SoftDelete(running) error = %v", err)
	}
	current = start.Add(3 * time.Hour)
	archived, err := repository.Archive(ctx, created.ContestID, 4)
	if err != nil || archived.Revision != 5 || archived.Status != ContestStatusArchived {
		t.Fatalf("Archive() = %+v err=%v", archived, err)
	}
	deletedRevision, err := repository.SoftDelete(ctx, created.ContestID, 5)
	if err != nil || deletedRevision != 6 {
		t.Fatalf("SoftDelete() revision=%d err=%v", deletedRevision, err)
	}
	restored, err := repository.Restore(ctx, created.ContestID, 6)
	if err != nil || restored.Revision != 7 || restored.Status != ContestStatusArchived {
		t.Fatalf("Restore() = %+v err=%v", restored, err)
	}
}

func TestContestModelApprovalAndScoreboardFreeze(t *testing.T) {
	repository, _ := newContestModelTestDatabase(t)
	ctx := context.Background()
	current := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	repository.now = func() time.Time { return current }

	approvalStart := current.Add(4 * time.Hour)
	approval, err := repository.Create(ctx, CreateContestInput{CreatorID: 1, Editable: EditableContest{
		Title: "Approval Contest", Mode: ContestModeIOI, StartAt: approvalStart, EndAt: approvalStart.Add(2 * time.Hour),
		RegistrationPolicy: RegistrationPolicyApprovalRequired, Visibility: ContestVisibilityPublic,
		ScoreboardVisibility: ScoreboardVisibilityParticipantsOnly,
	}})
	if err != nil {
		t.Fatalf("Create(approval) error = %v", err)
	}
	approval, _, err = repository.SetProblems(ctx, SetContestProblemsInput{ContestID: approval.ContestID, ExpectedRevision: 1,
		Problems: []ContestProblem{{ProblemID: 301, Label: "A", DisplayOrder: 0, MaxScore: 100}}})
	if err != nil {
		t.Fatalf("SetProblems(approval) error = %v", err)
	}
	approval, err = repository.Publish(ctx, approval.ContestID, approval.Revision)
	if err != nil {
		t.Fatalf("Publish(approval) error = %v", err)
	}
	pending, participant, err := repository.Register(ctx, RegisterContestInput{ContestID: approval.ContestID, UserID: 44, At: current})
	if err != nil || pending.Status != RegistrationStatusPending || participant != nil {
		t.Fatalf("approval Register() = %+v participant=%+v err=%v", pending, participant, err)
	}
	reapply, _, err := repository.Register(ctx, RegisterContestInput{ContestID: approval.ContestID, UserID: 45, At: current})
	if err != nil || reapply.Status != RegistrationStatusPending {
		t.Fatalf("initial reapply registration = %+v err=%v", reapply, err)
	}
	cancelled, err := repository.CancelRegistration(ctx, approval.ContestID, 45, reapply.Revision)
	if err != nil || cancelled.Status != RegistrationStatusCancelled || cancelled.Revision != 2 {
		t.Fatalf("CancelRegistration() = %+v err=%v", cancelled, err)
	}
	reapplied, _, err := repository.Register(ctx, RegisterContestInput{ContestID: approval.ContestID, UserID: 45, At: current})
	if err != nil || reapplied.RegistrationID != reapply.RegistrationID || reapplied.Revision != 3 ||
		reapplied.Status != RegistrationStatusPending {
		t.Fatalf("reapplied registration = %+v original=%+v err=%v", reapplied, reapply, err)
	}
	reviewed, participant, err := repository.ReviewRegistration(ctx, ReviewRegistrationInput{
		ContestID: approval.ContestID, RegistrationID: pending.RegistrationID, ReviewerID: 2,
		Decision: RegistrationDecisionApprove, ExpectedRevision: 1,
	})
	if err != nil || reviewed.Status != RegistrationStatusApproved || participant == nil {
		t.Fatalf("ReviewRegistration() = %+v participant=%+v err=%v", reviewed, participant, err)
	}
	if _, _, err := repository.ReviewRegistration(ctx, ReviewRegistrationInput{
		ContestID: approval.ContestID, RegistrationID: pending.RegistrationID, ReviewerID: 2,
		Decision: RegistrationDecisionReject, ExpectedRevision: 1,
	}); !errors.Is(err, ErrRegistrationRevisionConflict) {
		t.Fatalf("stale ReviewRegistration() error = %v", err)
	}
	participants, total, err := repository.ListParticipants(ctx, ParticipantListFilter{ContestID: approval.ContestID})
	if err != nil || total != 1 || len(participants) != 1 {
		t.Fatalf("ListParticipants() total=%d rows=%+v err=%v", total, participants, err)
	}
	disqualified, err := repository.SetParticipantStatus(ctx, SetParticipantStatusInput{
		ContestID: approval.ContestID, UserID: 44, Status: ParticipantStatusDisqualified,
	})
	if err != nil || disqualified.Status != ParticipantStatusDisqualified {
		t.Fatalf("SetParticipantStatus(disqualified) = %+v err=%v", disqualified, err)
	}
	restoredParticipant, err := repository.SetParticipantStatus(ctx, SetParticipantStatusInput{
		ContestID: approval.ContestID, UserID: 44, Status: ParticipantStatusNormal,
	})
	if err != nil || restoredParticipant.Status != ParticipantStatusNormal {
		t.Fatalf("SetParticipantStatus(normal) = %+v err=%v", restoredParticipant, err)
	}
	if _, err := repository.SetParticipantStatus(ctx, SetParticipantStatusInput{
		ContestID: approval.ContestID, UserID: 44, Status: ParticipantStatusNormal,
	}); !errors.Is(err, ErrParticipantStateConflict) {
		t.Fatalf("same-state SetParticipantStatus() error = %v", err)
	}

	runningStart := current.Add(-time.Hour)
	runningEnd := current.Add(2 * time.Hour)
	freezeBeforeEnd := 2 * time.Hour
	contest, err := repository.Create(ctx, CreateContestInput{CreatorID: 1, Editable: EditableContest{
		Title: "Live ICPC", Mode: ContestModeICPC, StartAt: runningStart, EndAt: runningEnd,
		RegistrationPolicy: RegistrationPolicyOpen, Visibility: ContestVisibilityPublic,
		ScoreboardVisibility: ScoreboardVisibilityPublic, ScoreboardFreezeBeforeEnd: &freezeBeforeEnd,
	}})
	if err != nil {
		t.Fatalf("Create(scoreboard) error = %v", err)
	}
	contest, _, err = repository.SetProblems(ctx, SetContestProblemsInput{ContestID: contest.ContestID, ExpectedRevision: 1,
		Problems: []ContestProblem{{ProblemID: 401, Label: "A", DisplayOrder: 0}, {ProblemID: 402, Label: "B", DisplayOrder: 1}}})
	if err != nil {
		t.Fatalf("SetProblems(scoreboard) error = %v", err)
	}
	contest, err = repository.Publish(ctx, contest.ContestID, contest.Revision)
	if err != nil || contest.EffectiveStatus != ContestStatusRunning {
		t.Fatalf("Publish(scoreboard) = %+v err=%v", contest, err)
	}
	due, err := repository.ListDueScoreboardFreezes(ctx, current, 10)
	if err != nil || len(due) != 1 || due[0].ContestID != contest.ContestID || !due[0].FreezeAt.Equal(current) {
		t.Fatalf("ListDueScoreboardFreezes() = %+v err=%v", due, err)
	}
	firstAccepted := current.Add(-30 * time.Minute)
	versionOne := []ScoreboardRow{{Rank: 1, Ordinal: 1, UserID: 51, DisplayName: "alpha",
		ICPC: &IcpcScore{SolvedCount: 1, TotalPenalty: 20 * time.Minute, LastSolvedAt: &firstAccepted},
		ProblemResults: []ScoreboardProblemResult{{ProblemID: 401, Label: "A", ICPC: &IcpcProblemResult{
			Solved: true, WrongAttempts: 0, Penalty: 20 * time.Minute, AcceptedAt: &firstAccepted, FirstToSolve: true,
		}}}}, {Rank: 2, Ordinal: 2, UserID: 52, DisplayName: "beta", ICPC: &IcpcScore{}}}
	if err := repository.ReplaceScoreboard(ctx, ReplaceScoreboardInput{ContestID: contest.ContestID, Mode: ContestModeICPC,
		SourceVersion: 1, GeneratedAt: current, Rows: versionOne}); err != nil {
		t.Fatalf("ReplaceScoreboard(v1) error = %v", err)
	}
	live, err := repository.GetScoreboard(ctx, ScoreboardFilter{ContestID: contest.ContestID, View: ScoreboardViewLive})
	if err != nil || live.SourceVersion != 1 || live.Total != 2 || live.Rows[0].ICPC.SolvedCount != 1 {
		t.Fatalf("GetScoreboard(live) = %+v err=%v", live, err)
	}
	frozenContest, err := repository.FreezeScoreboard(ctx, contest.ContestID, contest.Revision)
	if err != nil || !frozenContest.ScoreboardFrozen || frozenContest.Revision != 4 {
		t.Fatalf("FreezeScoreboard() = %+v err=%v", frozenContest, err)
	}
	versionTwo := []ScoreboardRow{{Rank: 1, Ordinal: 1, UserID: 52, DisplayName: "beta",
		ICPC: &IcpcScore{SolvedCount: 2, TotalPenalty: 50 * time.Minute},
		ProblemResults: []ScoreboardProblemResult{
			{ProblemID: 401, Label: "A", ICPC: &IcpcProblemResult{Solved: true, Penalty: 20 * time.Minute}},
			{ProblemID: 402, Label: "B", ICPC: &IcpcProblemResult{Solved: true, Penalty: 30 * time.Minute}},
		}}}
	if err := repository.ReplaceScoreboard(ctx, ReplaceScoreboardInput{ContestID: contest.ContestID, Mode: ContestModeICPC,
		SourceVersion: 2, GeneratedAt: current.Add(time.Minute), Rows: versionTwo}); err != nil {
		t.Fatalf("ReplaceScoreboard(v2) error = %v", err)
	}
	if err := repository.ReplaceScoreboard(ctx, ReplaceScoreboardInput{ContestID: contest.ContestID, Mode: ContestModeICPC,
		SourceVersion: 2, GeneratedAt: current.Add(time.Minute), Rows: versionTwo}); err != nil {
		t.Fatalf("idempotent ReplaceScoreboard(v2) error = %v", err)
	}
	if _, err := repository.GetScoreboard(ctx, ScoreboardFilter{ContestID: contest.ContestID, View: ScoreboardViewLive}); !errors.Is(err, ErrScoreboardAlreadyFrozen) {
		t.Fatalf("live scoreboard leaked during freeze: %v", err)
	}
	frozen, err := repository.GetScoreboard(ctx, ScoreboardFilter{ContestID: contest.ContestID, View: ScoreboardViewFrozenPublic})
	if err != nil || frozen.SourceVersion != 1 || frozen.Total != 2 || frozen.Rows[0].UserID != 51 {
		t.Fatalf("GetScoreboard(frozen) = %+v err=%v", frozen, err)
	}
	unfrozen, err := repository.UnfreezeScoreboard(ctx, contest.ContestID, frozenContest.Revision)
	if err != nil || unfrozen.ScoreboardFrozen || unfrozen.Revision != 5 {
		t.Fatalf("UnfreezeScoreboard() = %+v err=%v", unfrozen, err)
	}
	live, err = repository.GetScoreboard(ctx, ScoreboardFilter{ContestID: contest.ContestID, View: ScoreboardViewLive})
	if err != nil || live.SourceVersion != 2 || live.Total != 1 || live.Rows[0].UserID != 52 {
		t.Fatalf("GetScoreboard(after unfreeze) = %+v err=%v", live, err)
	}
	if err := repository.ReplaceScoreboard(ctx, ReplaceScoreboardInput{ContestID: contest.ContestID, Mode: ContestModeICPC,
		SourceVersion: 1, GeneratedAt: current, Rows: versionOne}); !errors.Is(err, ErrScoreboardVersionConflict) {
		t.Fatalf("stale ReplaceScoreboard() error = %v", err)
	}
	current = runningEnd.Add(time.Minute)
	final, err := repository.GetScoreboard(ctx, ScoreboardFilter{ContestID: contest.ContestID, View: ScoreboardViewFinal})
	if err != nil || final.SourceVersion != 2 || final.View != ScoreboardViewFinal {
		t.Fatalf("GetScoreboard(final) = %+v err=%v", final, err)
	}
}

func TestContestModelInviteAndPointsScoreboards(t *testing.T) {
	repository, _ := newContestModelTestDatabase(t)
	ctx := context.Background()
	current := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)
	repository.now = func() time.Time { return current }
	inviteCode := "invite-only-secret"
	inviteStart := current.Add(3 * time.Hour)
	inviteContest, err := repository.Create(ctx, CreateContestInput{CreatorID: 1, Editable: EditableContest{
		Title: "Invite Contest", Mode: ContestModeIOI, StartAt: inviteStart, EndAt: inviteStart.Add(time.Hour),
		RegistrationPolicy: RegistrationPolicyInviteOnly, Visibility: ContestVisibilityPrivate,
		ScoreboardVisibility: ScoreboardVisibilityParticipantsOnly, InviteCode: &inviteCode,
	}})
	if err != nil {
		t.Fatalf("Create(invite) error = %v", err)
	}
	if !inviteContest.InviteCodeConfigured || inviteContest.Editable.InviteCode != nil {
		t.Fatalf("invite secret leaked or was not configured: %+v", inviteContest)
	}
	inviteContest, _, err = repository.SetProblems(ctx, SetContestProblemsInput{ContestID: inviteContest.ContestID,
		ExpectedRevision: 1, Problems: []ContestProblem{{ProblemID: 501, Label: "A", DisplayOrder: 0, MaxScore: 100}}})
	if err != nil {
		t.Fatalf("SetProblems(invite) error = %v", err)
	}
	inviteContest, err = repository.Publish(ctx, inviteContest.ContestID, inviteContest.Revision)
	if err != nil {
		t.Fatalf("Publish(invite) error = %v", err)
	}
	if _, _, err := repository.Register(ctx, RegisterContestInput{ContestID: inviteContest.ContestID,
		UserID: 71, InviteCode: "wrong", At: current}); !errors.Is(err, ErrInvalidInviteCode) {
		t.Fatalf("wrong invite registration error = %v", err)
	}
	registration, participant, err := repository.Register(ctx, RegisterContestInput{ContestID: inviteContest.ContestID,
		UserID: 71, InviteCode: inviteCode, At: current})
	if err != nil || registration.Status != RegistrationStatusApproved || participant == nil {
		t.Fatalf("invite registration = %+v participant=%+v err=%v", registration, participant, err)
	}
	if err := repository.VerifyInviteCode(ctx, inviteContest.ContestID, inviteCode); err != nil {
		t.Fatalf("VerifyInviteCode() error = %v", err)
	}

	for index, mode := range []int64{ContestModeIOI, ContestModeOI} {
		t.Run(fmt.Sprintf("mode-%d", mode), func(t *testing.T) {
			start := current.Add(-time.Hour)
			contest, err := repository.Create(ctx, CreateContestInput{CreatorID: 1, Editable: EditableContest{
				Title: fmt.Sprintf("Points Contest %d", mode), Mode: mode, StartAt: start, EndAt: current.Add(time.Hour),
				RegistrationPolicy: RegistrationPolicyOpen, Visibility: ContestVisibilityPublic,
				ScoreboardVisibility: ScoreboardVisibilityPublic,
			}})
			if err != nil {
				t.Fatalf("Create(points) error = %v", err)
			}
			problemID := int64(600 + index)
			contest, _, err = repository.SetProblems(ctx, SetContestProblemsInput{ContestID: contest.ContestID,
				ExpectedRevision: 1, Problems: []ContestProblem{{ProblemID: problemID, Label: "A", DisplayOrder: 0, MaxScore: 100}}})
			if err != nil {
				t.Fatalf("SetProblems(points) error = %v", err)
			}
			contest, err = repository.Publish(ctx, contest.ContestID, contest.Revision)
			if err != nil {
				t.Fatalf("Publish(points) error = %v", err)
			}
			lastSubmission := current.Add(-time.Minute)
			err = repository.ReplaceScoreboard(ctx, ReplaceScoreboardInput{ContestID: contest.ContestID, Mode: mode,
				SourceVersion: 1, GeneratedAt: current, Rows: []ScoreboardRow{{
					Rank: 1, Ordinal: 1, UserID: int64(80 + index), DisplayName: "points-user",
					Points: &PointsScore{TotalScore: 80, LastEffectiveSubmissionAt: &lastSubmission},
					ProblemResults: []ScoreboardProblemResult{{ProblemID: problemID, Label: "A",
						Points: &PointsProblemResult{Score: 80, MaxScore: 100, SubmissionCount: 2,
							LastEffectiveSubmissionAt: &lastSubmission}}},
				}}})
			if err != nil {
				t.Fatalf("ReplaceScoreboard(points) error = %v", err)
			}
			page, err := repository.GetScoreboard(ctx, ScoreboardFilter{ContestID: contest.ContestID, View: ScoreboardViewLive})
			if err != nil || page.Mode != mode || page.Total != 1 || page.Rows[0].Points == nil ||
				page.Rows[0].Points.TotalScore != 80 || page.Rows[0].ProblemResults[0].Points.Score != 80 {
				t.Fatalf("GetScoreboard(points) = %+v err=%v", page, err)
			}
		})
	}
}

func newContestModelTestDatabase(t *testing.T) (*defaultContestModel, *gorm.DB) {
	t.Helper()
	dsn := os.Getenv("CONTEST_MODEL_TEST_DSN")
	if dsn == "" {
		t.Skip("set CONTEST_MODEL_TEST_DSN to run isolated MySQL integration tests")
	}
	config, err := mysqlDriver.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("parse CONTEST_MODEL_TEST_DSN: %v", err)
	}
	config.DBName = ""
	config.ParseTime = true
	config.Loc = time.UTC
	admin, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatalf("open MySQL admin connection: %v", err)
	}
	databaseName := fmt.Sprintf("contest_model_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec("CREATE DATABASE `" + databaseName + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"); err != nil {
		_ = admin.Close()
		t.Fatalf("create isolated database: %v", err)
	}
	var testSQLDB *sql.DB
	t.Cleanup(func() {
		if testSQLDB != nil {
			_ = testSQLDB.Close()
		}
		_, _ = admin.Exec("DROP DATABASE IF EXISTS `" + databaseName + "`")
		_ = admin.Close()
	})

	config.DBName = databaseName
	testSQLDB, err = sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatalf("open isolated database: %v", err)
	}
	applyContestMigration(t, testSQLDB, filepath.Join("..", "migrations", "001_create_contest.sql"))
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: testSQLDB}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open GORM test database: %v", err)
	}
	return &defaultContestModel{db: db, now: func() time.Time { return time.Now().UTC() }}, db
}

func applyContestMigration(t *testing.T, db *sql.DB, fileName string) {
	t.Helper()
	contents, err := os.ReadFile(fileName)
	if err != nil {
		t.Fatalf("read migration %s: %v", fileName, err)
	}
	lines := strings.Split(string(contents), "\n")
	filtered := lines[:0]
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		filtered = append(filtered, line)
	}
	for _, statement := range strings.Split(strings.Join(filtered, "\n"), ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("apply migration %s: %v\nSQL: %s", fileName, err, statement)
		}
	}
}
