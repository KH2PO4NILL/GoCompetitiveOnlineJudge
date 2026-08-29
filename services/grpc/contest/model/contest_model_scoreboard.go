package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (m *defaultContestModel) ReplaceScoreboard(ctx context.Context, input ReplaceScoreboardInput) error {
	if input.ContestID <= 0 || input.SourceVersion <= 0 || !validContestMode(input.Mode) {
		return fmt.Errorf("%w: contest id, mode and source version are required", ErrInvalidContestInput)
	}
	generatedAt := input.GeneratedAt
	if generatedAt.IsZero() {
		generatedAt = m.now().UTC()
	} else {
		generatedAt = generatedAt.UTC()
	}
	rows, err := validateScoreboardRows(input.Rows, input.Mode)
	if err != nil {
		return err
	}

	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		contest, err := lockContest(tx, input.ContestID, false)
		if err != nil {
			return err
		}
		if contest.Mode != input.Mode || contest.Status != ContestStatusPublished {
			return ErrContestStateConflict
		}
		state, err := lockScoreboardState(tx, contest.ID)
		if err != nil {
			return err
		}
		if input.SourceVersion == state.SourceVersion {
			return nil
		}
		if input.SourceVersion < state.SourceVersion {
			return ErrScoreboardVersionConflict
		}
		assigned, err := loadAssignedProblems(tx, contest.ID)
		if err != nil {
			return err
		}
		if err := validateScoreboardAssignments(rows, input.Mode, assigned); err != nil {
			return err
		}

		if err := tx.Where("contest_id = ?", contest.ID).Delete(&scoreboardProblemResultRow{}).Error; err != nil {
			return fmt.Errorf("delete previous scoreboard problem results: %w", err)
		}
		if err := tx.Where("contest_id = ?", contest.ID).Delete(&scoreboardRowRecord{}).Error; err != nil {
			return fmt.Errorf("delete previous scoreboard rows: %w", err)
		}
		rowRecords, resultRecords := scoreboardRecords(contest.ID, input.Mode, rows)
		if len(rowRecords) > 0 {
			if err := tx.CreateInBatches(&rowRecords, 500).Error; err != nil {
				return fmt.Errorf("insert scoreboard rows: %w", err)
			}
		}
		if len(resultRecords) > 0 {
			if err := tx.CreateInBatches(&resultRecords, 1000).Error; err != nil {
				return fmt.Errorf("insert scoreboard problem results: %w", err)
			}
		}
		now := m.now().UTC()
		result := tx.Model(&scoreboardStateRow{}).Where("contest_id = ? AND source_version = ?", contest.ID, state.SourceVersion).
			Updates(map[string]any{"mode": input.Mode, "source_version": input.SourceVersion,
				"generated_at": generatedAt, "updated_at": now})
		if result.Error != nil {
			return fmt.Errorf("update scoreboard state: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return ErrScoreboardVersionConflict
		}
		return nil
	})
}

func (m *defaultContestModel) GetScoreboard(ctx context.Context, filter ScoreboardFilter) (*ScoreboardPage, error) {
	if filter.ContestID <= 0 {
		return nil, fmt.Errorf("%w: contest id must be positive", ErrInvalidContestInput)
	}
	if filter.View < ScoreboardViewLive || filter.View > ScoreboardViewFinal {
		return nil, fmt.Errorf("%w: invalid scoreboard view", ErrInvalidContestInput)
	}
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	var result *ScoreboardPage
	err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		result, err = m.getScoreboard(tx, filter, page, pageSize)
		return err
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return result, err
}

func (m *defaultContestModel) ListDueScoreboardFreezes(ctx context.Context, at time.Time, limit int) ([]ScoreboardFreezeCandidate, error) {
	at = m.at(at)
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	const dueQuery = `
		SELECT due.contest_id, due.revision, due.freeze_at
		FROM (
			SELECT id AS contest_id, revision, scoreboard_freeze_at AS freeze_at
			FROM contests
			WHERE mode = ? AND status = ? AND deleted_at IS NULL AND scoreboard_frozen = FALSE
				AND start_at <= ? AND end_at > ?
				AND scoreboard_freeze_at IS NOT NULL AND scoreboard_freeze_at <= ?
			UNION ALL
			SELECT id AS contest_id, revision,
				TIMESTAMPADD(MICROSECOND, -CAST(scoreboard_freeze_before_end_ms * 1000 AS SIGNED), end_at) AS freeze_at
			FROM contests
			WHERE mode = ? AND status = ? AND deleted_at IS NULL AND scoreboard_frozen = FALSE
				AND start_at <= ? AND end_at > ? AND scoreboard_freeze_before_end_ms IS NOT NULL
				AND TIMESTAMPADD(MICROSECOND, -CAST(scoreboard_freeze_before_end_ms * 1000 AS SIGNED), end_at) <= ?
		) AS due
		ORDER BY due.freeze_at ASC, due.contest_id ASC
		LIMIT ?`
	var rows []struct {
		ContestID int64     `gorm:"column:contest_id"`
		Revision  int64     `gorm:"column:revision"`
		FreezeAt  time.Time `gorm:"column:freeze_at"`
	}
	err := m.db.WithContext(ctx).Raw(dueQuery,
		ContestModeICPC, ContestStatusPublished, at, at, at,
		ContestModeICPC, ContestStatusPublished, at, at, at, limit).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list due scoreboard freezes: %w", err)
	}
	result := make([]ScoreboardFreezeCandidate, 0, len(rows))
	for _, row := range rows {
		result = append(result, ScoreboardFreezeCandidate{
			ContestID: row.ContestID, Revision: row.Revision, FreezeAt: row.FreezeAt.UTC(),
		})
	}
	return result, nil
}

func (m *defaultContestModel) getScoreboard(db *gorm.DB, filter ScoreboardFilter, page, pageSize int) (*ScoreboardPage, error) {
	contest, err := findContest(db, filter.ContestID, false)
	if err != nil {
		return nil, err
	}
	var state scoreboardStateRow
	if err := db.Where("contest_id = ?", contest.ID).Take(&state).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrScoreboardNotReady
		}
		return nil, fmt.Errorf("get scoreboard state: %w", err)
	}
	if state.SourceVersion == 0 || state.GeneratedAt == nil {
		return nil, ErrScoreboardNotReady
	}
	if contest.ScoreboardFrozen && filter.View != ScoreboardViewFrozenPublic {
		return nil, ErrScoreboardAlreadyFrozen
	}
	if !contest.ScoreboardFrozen && filter.View == ScoreboardViewFrozenPublic {
		return nil, ErrScoreboardNotFrozen
	}
	if filter.View == ScoreboardViewFinal {
		effective := EffectiveContestStatus(contest.Status, contest.StartAt, contest.EndAt, m.now().UTC())
		if effective != ContestStatusFinished && contest.Status != ContestStatusArchived {
			return nil, ErrContestStateConflict
		}
	}

	result := &ScoreboardPage{
		ContestID: contest.ID, Mode: state.Mode, GeneratedAt: state.GeneratedAt.UTC(),
		Frozen: contest.ScoreboardFrozen, View: filter.View, SourceVersion: state.SourceVersion,
	}
	if filter.View == ScoreboardViewFrozenPublic {
		if state.ActiveFrozenSnapshotID == nil {
			return nil, ErrScoreboardSnapshotConflict
		}
		return loadSnapshotScoreboard(db, result, *state.ActiveFrozenSnapshotID, page, pageSize)
	}
	return loadLiveScoreboard(db, result, page, pageSize)
}

func (m *defaultContestModel) FreezeScoreboard(ctx context.Context, contestID, expectedRevision int64) (*ContestAggregate, error) {
	if contestID <= 0 || expectedRevision <= 0 {
		return nil, fmt.Errorf("%w: contest id and expected revision must be positive", ErrInvalidContestInput)
	}
	var aggregate *ContestAggregate
	err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		contest, err := lockContest(tx, contestID, false)
		if err != nil {
			return err
		}
		if contest.Revision != expectedRevision {
			return ErrContestRevisionConflict
		}
		if contest.ScoreboardFrozen {
			return ErrScoreboardAlreadyFrozen
		}
		now := m.now().UTC()
		if contest.Mode != ContestModeICPC ||
			EffectiveContestStatus(contest.Status, contest.StartAt, contest.EndAt, now) != ContestStatusRunning {
			return ErrContestStateConflict
		}
		state, err := lockScoreboardState(tx, contest.ID)
		if err != nil {
			return err
		}
		if state.SourceVersion == 0 || state.GeneratedAt == nil {
			return ErrScoreboardNotReady
		}
		snapshot := &scoreboardSnapshotRow{
			ContestID: contest.ID, SourceVersion: state.SourceVersion, Mode: contest.Mode,
			GeneratedAt: state.GeneratedAt.UTC(), FrozenAt: now, CreatedAt: now,
		}
		if err := tx.Create(snapshot).Error; err != nil {
			if isDuplicateKeyError(err) {
				return ErrScoreboardSnapshotConflict
			}
			return fmt.Errorf("create scoreboard snapshot: %w", err)
		}
		if err := copyScoreboardSnapshot(tx, contest.ID, snapshot.ID); err != nil {
			return err
		}
		if err := tx.Model(&scoreboardStateRow{}).Where("contest_id = ? AND source_version = ?", contest.ID, state.SourceVersion).
			Updates(map[string]any{"active_frozen_snapshot_id": snapshot.ID, "updated_at": now}).Error; err != nil {
			return fmt.Errorf("activate scoreboard snapshot: %w", err)
		}
		contest.ScoreboardFrozen = true
		contest.ScoreboardFrozenAt = utcTimePointer(&now)
		contest.Revision++
		contest.UpdatedAt = now
		if err := conditionalContestUpdate(tx, contest, expectedRevision, false); err != nil {
			return err
		}
		aggregate = aggregateFromContestRow(contest, now)
		return nil
	})
	return aggregate, err
}

func (m *defaultContestModel) UnfreezeScoreboard(ctx context.Context, contestID, expectedRevision int64) (*ContestAggregate, error) {
	if contestID <= 0 || expectedRevision <= 0 {
		return nil, fmt.Errorf("%w: contest id and expected revision must be positive", ErrInvalidContestInput)
	}
	var aggregate *ContestAggregate
	err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		contest, err := lockContest(tx, contestID, false)
		if err != nil {
			return err
		}
		if contest.Revision != expectedRevision {
			return ErrContestRevisionConflict
		}
		if !contest.ScoreboardFrozen {
			return ErrScoreboardNotFrozen
		}
		if _, err := lockScoreboardState(tx, contest.ID); err != nil {
			return err
		}
		now := m.now().UTC()
		if err := tx.Model(&scoreboardStateRow{}).Where("contest_id = ?", contest.ID).
			Updates(map[string]any{"active_frozen_snapshot_id": nil, "updated_at": now}).Error; err != nil {
			return fmt.Errorf("deactivate scoreboard snapshot: %w", err)
		}
		contest.ScoreboardFrozen = false
		contest.ScoreboardFrozenAt = nil
		contest.Revision++
		contest.UpdatedAt = now
		if err := conditionalContestUpdate(tx, contest, expectedRevision, false); err != nil {
			return err
		}
		aggregate = aggregateFromContestRow(contest, now)
		return nil
	})
	return aggregate, err
}

func lockScoreboardState(tx *gorm.DB, contestID int64) (*scoreboardStateRow, error) {
	row := new(scoreboardStateRow)
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("contest_id = ?", contestID).Take(row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrScoreboardNotReady
	}
	if err != nil {
		return nil, fmt.Errorf("lock scoreboard state: %w", err)
	}
	return row, nil
}

func validateScoreboardRows(rows []ScoreboardRow, mode int64) ([]ScoreboardRow, error) {
	result := make([]ScoreboardRow, len(rows))
	users := make(map[int64]struct{}, len(rows))
	ordinals := make(map[int64]struct{}, len(rows))
	firstToSolve := make(map[int64]int64)
	for i, row := range rows {
		row.DisplayName = strings.TrimSpace(row.DisplayName)
		if row.UserID <= 0 || row.Rank <= 0 || row.Ordinal <= 0 || row.DisplayName == "" ||
			utf8.RuneCountInString(row.DisplayName) > 200 {
			return nil, fmt.Errorf("%w: invalid scoreboard row at index %d", ErrInvalidContestInput, i)
		}
		if _, exists := users[row.UserID]; exists {
			return nil, fmt.Errorf("%w: duplicate scoreboard user", ErrInvalidContestInput)
		}
		if _, exists := ordinals[row.Ordinal]; exists {
			return nil, fmt.Errorf("%w: duplicate scoreboard ordinal", ErrInvalidContestInput)
		}
		users[row.UserID] = struct{}{}
		ordinals[row.Ordinal] = struct{}{}
		if mode == ContestModeICPC {
			if row.ICPC == nil || row.Points != nil || row.ICPC.SolvedCount < 0 ||
				row.ICPC.SolvedCount > maxProtoInt32 || row.ICPC.TotalPenalty < 0 ||
				row.ICPC.TotalPenalty%time.Millisecond != 0 {
				return nil, fmt.Errorf("%w: invalid ICPC score", ErrInvalidContestInput)
			}
		} else if row.Points == nil || row.ICPC != nil || row.Points.TotalScore < 0 {
			return nil, fmt.Errorf("%w: invalid points score", ErrInvalidContestInput)
		}
		problems := make(map[int64]struct{}, len(row.ProblemResults))
		for j, problem := range row.ProblemResults {
			problem.Label = strings.TrimSpace(problem.Label)
			if problem.ProblemID <= 0 || problem.Label == "" {
				return nil, fmt.Errorf("%w: invalid scoreboard problem result at row %d index %d", ErrInvalidContestInput, i, j)
			}
			if _, exists := problems[problem.ProblemID]; exists {
				return nil, fmt.Errorf("%w: duplicate scoreboard problem result", ErrInvalidContestInput)
			}
			problems[problem.ProblemID] = struct{}{}
			if mode == ContestModeICPC {
				if problem.ICPC == nil || problem.Points != nil || problem.ICPC.WrongAttempts < 0 ||
					problem.ICPC.WrongAttempts > maxProtoInt32 ||
					problem.ICPC.Penalty < 0 || problem.ICPC.Penalty%time.Millisecond != 0 {
					return nil, fmt.Errorf("%w: invalid ICPC problem result", ErrInvalidContestInput)
				}
				if problem.ICPC.FirstToSolve {
					if _, exists := firstToSolve[problem.ProblemID]; exists {
						return nil, fmt.Errorf("%w: multiple first-to-solve rows", ErrInvalidContestInput)
					}
					firstToSolve[problem.ProblemID] = row.UserID
				}
			} else if problem.Points == nil || problem.ICPC != nil || problem.Points.Score < 0 ||
				problem.Points.MaxScore <= 0 || problem.Points.Score > problem.Points.MaxScore ||
				problem.Points.SubmissionCount < 0 || problem.Points.SubmissionCount > maxProtoInt32 {
				return nil, fmt.Errorf("%w: invalid points problem result", ErrInvalidContestInput)
			}
			row.ProblemResults[j] = problem
		}
		result[i] = row
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Ordinal < result[j].Ordinal })
	for i := 1; i < len(result); i++ {
		if result[i].Rank < result[i-1].Rank {
			return nil, fmt.Errorf("%w: scoreboard rank must be monotonic by ordinal", ErrInvalidContestInput)
		}
	}
	return result, nil
}

func loadAssignedProblems(tx *gorm.DB, contestID int64) (map[int64]ContestProblem, error) {
	var rows []contestProblemRow
	if err := tx.Where("contest_id = ?", contestID).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("load assigned contest problems: %w", err)
	}
	result := make(map[int64]ContestProblem, len(rows))
	for _, row := range rows {
		result[row.ProblemID] = ContestProblem{ContestID: row.ContestID, ProblemID: row.ProblemID,
			Label: row.Label, DisplayOrder: row.DisplayOrder, MaxScore: row.MaxScore}
	}
	return result, nil
}

func validateScoreboardAssignments(rows []ScoreboardRow, mode int64, assigned map[int64]ContestProblem) error {
	for _, row := range rows {
		for _, result := range row.ProblemResults {
			problem, exists := assigned[result.ProblemID]
			if !exists || problem.Label != result.Label {
				return fmt.Errorf("%w: scoreboard result references an unassigned problem", ErrInvalidContestInput)
			}
			if mode != ContestModeICPC && result.Points.MaxScore != problem.MaxScore {
				return fmt.Errorf("%w: scoreboard max score differs from contest assignment", ErrInvalidContestInput)
			}
		}
	}
	return nil
}

func scoreboardRecords(contestID, mode int64, rows []ScoreboardRow) ([]scoreboardRowRecord, []scoreboardProblemResultRow) {
	rowRecords := make([]scoreboardRowRecord, 0, len(rows))
	resultRecords := make([]scoreboardProblemResultRow, 0)
	for _, row := range rows {
		record := scoreboardRowRecord{ContestID: contestID, UserID: row.UserID, DisplayName: row.DisplayName,
			Mode: mode, Rank: row.Rank, Ordinal: row.Ordinal}
		if mode == ContestModeICPC {
			record.SolvedCount = int64Pointer(row.ICPC.SolvedCount)
			record.TotalPenaltyMS = int64Pointer(row.ICPC.TotalPenalty.Milliseconds())
			record.LastSolvedAt = utcTimePointer(row.ICPC.LastSolvedAt)
		} else {
			record.TotalScore = int64Pointer(row.Points.TotalScore)
			record.LastEffectiveSubmissionAt = utcTimePointer(row.Points.LastEffectiveSubmissionAt)
		}
		rowRecords = append(rowRecords, record)
		for _, problem := range row.ProblemResults {
			problemRecord := scoreboardProblemResultRow{ContestID: contestID, UserID: row.UserID,
				ProblemID: problem.ProblemID, Label: problem.Label, Mode: mode}
			if mode == ContestModeICPC {
				problemRecord.Solved = boolPointer(problem.ICPC.Solved)
				problemRecord.WrongAttempts = int64Pointer(problem.ICPC.WrongAttempts)
				problemRecord.PenaltyMS = int64Pointer(problem.ICPC.Penalty.Milliseconds())
				problemRecord.AcceptedAt = utcTimePointer(problem.ICPC.AcceptedAt)
				problemRecord.FirstToSolve = boolPointer(problem.ICPC.FirstToSolve)
			} else {
				problemRecord.Score = int64Pointer(problem.Points.Score)
				problemRecord.MaxScore = int64Pointer(problem.Points.MaxScore)
				problemRecord.SubmissionCount = int64Pointer(problem.Points.SubmissionCount)
				problemRecord.LastEffectiveSubmissionAt = utcTimePointer(problem.Points.LastEffectiveSubmissionAt)
			}
			resultRecords = append(resultRecords, problemRecord)
		}
	}
	return rowRecords, resultRecords
}

func copyScoreboardSnapshot(tx *gorm.DB, contestID, snapshotID int64) error {
	rowSQL := `INSERT INTO contest_scoreboard_snapshot_rows
		(snapshot_id, user_id, display_name, mode, rank_value, ordinal, solved_count,
		total_penalty_ms, last_solved_at, total_score, last_effective_submission_at)
		SELECT ?, user_id, display_name, mode, rank_value, ordinal, solved_count,
		total_penalty_ms, last_solved_at, total_score, last_effective_submission_at
		FROM contest_scoreboard_rows WHERE contest_id = ?`
	if err := tx.Exec(rowSQL, snapshotID, contestID).Error; err != nil {
		return fmt.Errorf("copy scoreboard snapshot rows: %w", err)
	}
	resultSQL := `INSERT INTO contest_scoreboard_snapshot_problem_results
		(snapshot_id, user_id, problem_id, label, mode, solved, wrong_attempts, penalty_ms,
		accepted_at, first_to_solve, score, max_score, submission_count, last_effective_submission_at)
		SELECT ?, user_id, problem_id, label, mode, solved, wrong_attempts, penalty_ms,
		accepted_at, first_to_solve, score, max_score, submission_count, last_effective_submission_at
		FROM contest_scoreboard_problem_results WHERE contest_id = ?`
	if err := tx.Exec(resultSQL, snapshotID, contestID).Error; err != nil {
		return fmt.Errorf("copy scoreboard snapshot problem results: %w", err)
	}
	return nil
}

func loadLiveScoreboard(db *gorm.DB, result *ScoreboardPage, page, pageSize int) (*ScoreboardPage, error) {
	query := db.Model(&scoreboardRowRecord{}).Where("contest_id = ?", result.ContestID)
	if err := query.Count(&result.Total).Error; err != nil {
		return nil, fmt.Errorf("count live scoreboard rows: %w", err)
	}
	if result.Total == 0 {
		result.Rows = []ScoreboardRow{}
		return result, nil
	}
	var records []scoreboardRowRecord
	if err := query.Order("ordinal ASC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&records).Error; err != nil {
		return nil, fmt.Errorf("load live scoreboard rows: %w", err)
	}
	userIDs := scoreboardUserIDs(records)
	var problemRecords []scoreboardProblemResultRow
	if len(userIDs) > 0 {
		err := db.Table("contest_scoreboard_problem_results AS r").Select("r.*").
			Joins("LEFT JOIN contest_problems AS cp ON cp.contest_id = r.contest_id AND cp.problem_id = r.problem_id").
			Where("r.contest_id = ? AND r.user_id IN ?", result.ContestID, userIDs).
			Order("r.user_id ASC, cp.display_order ASC, r.problem_id ASC").Scan(&problemRecords).Error
		if err != nil {
			return nil, fmt.Errorf("load live scoreboard problem results: %w", err)
		}
	}
	result.Rows = scoreboardRowsFromLiveRecords(records, problemRecords, result.Mode)
	return result, nil
}

func loadSnapshotScoreboard(db *gorm.DB, result *ScoreboardPage, snapshotID int64, page, pageSize int) (*ScoreboardPage, error) {
	var snapshot scoreboardSnapshotRow
	if err := db.Where("id = ? AND contest_id = ?", snapshotID, result.ContestID).Take(&snapshot).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrScoreboardSnapshotConflict
		}
		return nil, fmt.Errorf("load scoreboard snapshot: %w", err)
	}
	result.Mode = snapshot.Mode
	result.GeneratedAt = snapshot.GeneratedAt.UTC()
	result.SourceVersion = snapshot.SourceVersion
	query := db.Model(&scoreboardSnapshotRowRecord{}).Where("snapshot_id = ?", snapshotID)
	if err := query.Count(&result.Total).Error; err != nil {
		return nil, fmt.Errorf("count scoreboard snapshot rows: %w", err)
	}
	if result.Total == 0 {
		result.Rows = []ScoreboardRow{}
		return result, nil
	}
	var records []scoreboardSnapshotRowRecord
	if err := query.Order("ordinal ASC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&records).Error; err != nil {
		return nil, fmt.Errorf("load scoreboard snapshot rows: %w", err)
	}
	userIDs := snapshotUserIDs(records)
	var problemRecords []scoreboardSnapshotProblemResultRow
	if len(userIDs) > 0 {
		err := db.Table("contest_scoreboard_snapshot_problem_results AS r").Select("r.*").
			Joins("JOIN contest_scoreboard_snapshots AS s ON s.id = r.snapshot_id").
			Joins("LEFT JOIN contest_problems AS cp ON cp.contest_id = s.contest_id AND cp.problem_id = r.problem_id").
			Where("r.snapshot_id = ? AND r.user_id IN ?", snapshotID, userIDs).
			Order("r.user_id ASC, cp.display_order ASC, r.problem_id ASC").Scan(&problemRecords).Error
		if err != nil {
			return nil, fmt.Errorf("load scoreboard snapshot problem results: %w", err)
		}
	}
	result.Rows = scoreboardRowsFromSnapshotRecords(records, problemRecords, result.Mode)
	return result, nil
}

func scoreboardRowsFromLiveRecords(rows []scoreboardRowRecord, results []scoreboardProblemResultRow, mode int64) []ScoreboardRow {
	byUser := make(map[int64][]ScoreboardProblemResult)
	for _, result := range results {
		byUser[result.UserID] = append(byUser[result.UserID], problemResultFromRecord(mode, result.ProblemID,
			result.Label, result.Solved, result.WrongAttempts, result.PenaltyMS, result.AcceptedAt,
			result.FirstToSolve, result.Score, result.MaxScore, result.SubmissionCount, result.LastEffectiveSubmissionAt))
	}
	output := make([]ScoreboardRow, 0, len(rows))
	for _, row := range rows {
		output = append(output, scoreboardRowFromValues(mode, row.Rank, row.Ordinal, row.UserID, row.DisplayName,
			row.SolvedCount, row.TotalPenaltyMS, row.LastSolvedAt, row.TotalScore,
			row.LastEffectiveSubmissionAt, byUser[row.UserID]))
	}
	return output
}

func scoreboardRowsFromSnapshotRecords(rows []scoreboardSnapshotRowRecord, results []scoreboardSnapshotProblemResultRow, mode int64) []ScoreboardRow {
	byUser := make(map[int64][]ScoreboardProblemResult)
	for _, result := range results {
		byUser[result.UserID] = append(byUser[result.UserID], problemResultFromRecord(mode, result.ProblemID,
			result.Label, result.Solved, result.WrongAttempts, result.PenaltyMS, result.AcceptedAt,
			result.FirstToSolve, result.Score, result.MaxScore, result.SubmissionCount, result.LastEffectiveSubmissionAt))
	}
	output := make([]ScoreboardRow, 0, len(rows))
	for _, row := range rows {
		output = append(output, scoreboardRowFromValues(mode, row.Rank, row.Ordinal, row.UserID, row.DisplayName,
			row.SolvedCount, row.TotalPenaltyMS, row.LastSolvedAt, row.TotalScore,
			row.LastEffectiveSubmissionAt, byUser[row.UserID]))
	}
	return output
}

func scoreboardRowFromValues(mode, rank, ordinal, userID int64, displayName string, solvedCount, penaltyMS *int64,
	lastSolvedAt *time.Time, totalScore *int64, lastEffectiveAt *time.Time, results []ScoreboardProblemResult) ScoreboardRow {
	row := ScoreboardRow{Rank: rank, Ordinal: ordinal, UserID: userID, DisplayName: displayName,
		ProblemResults: results}
	if mode == ContestModeICPC {
		row.ICPC = &IcpcScore{LastSolvedAt: utcTimePointer(lastSolvedAt)}
		if solvedCount != nil {
			row.ICPC.SolvedCount = *solvedCount
		}
		if penaltyMS != nil {
			row.ICPC.TotalPenalty = time.Duration(*penaltyMS) * time.Millisecond
		}
	} else {
		row.Points = &PointsScore{LastEffectiveSubmissionAt: utcTimePointer(lastEffectiveAt)}
		if totalScore != nil {
			row.Points.TotalScore = *totalScore
		}
	}
	return row
}

func problemResultFromRecord(mode, problemID int64, label string, solved *bool, wrong, penaltyMS *int64,
	acceptedAt *time.Time, first *bool, score, maxScore, submissions *int64, lastEffectiveAt *time.Time) ScoreboardProblemResult {
	result := ScoreboardProblemResult{ProblemID: problemID, Label: label}
	if mode == ContestModeICPC {
		result.ICPC = &IcpcProblemResult{AcceptedAt: utcTimePointer(acceptedAt)}
		if solved != nil {
			result.ICPC.Solved = *solved
		}
		if wrong != nil {
			result.ICPC.WrongAttempts = *wrong
		}
		if penaltyMS != nil {
			result.ICPC.Penalty = time.Duration(*penaltyMS) * time.Millisecond
		}
		if first != nil {
			result.ICPC.FirstToSolve = *first
		}
	} else {
		result.Points = &PointsProblemResult{LastEffectiveSubmissionAt: utcTimePointer(lastEffectiveAt)}
		if score != nil {
			result.Points.Score = *score
		}
		if maxScore != nil {
			result.Points.MaxScore = *maxScore
		}
		if submissions != nil {
			result.Points.SubmissionCount = *submissions
		}
	}
	return result
}

func scoreboardUserIDs(rows []scoreboardRowRecord) []int64 {
	result := make([]int64, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.UserID)
	}
	return result
}

func snapshotUserIDs(rows []scoreboardSnapshotRowRecord) []int64 {
	result := make([]int64, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.UserID)
	}
	return result
}
