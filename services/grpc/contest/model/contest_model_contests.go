package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (m *defaultContestModel) Create(ctx context.Context, input CreateContestInput) (*ContestAggregate, error) {
	if input.CreatorID <= 0 {
		return nil, fmt.Errorf("%w: creator id must be positive", ErrInvalidContestInput)
	}
	editable := input.Editable
	inviteConfigured := editable.InviteCode != nil
	if err := validateEditableContest(&editable, inviteConfigured); err != nil {
		return nil, err
	}
	var inviteHash *string
	var err error
	if editable.InviteCode != nil {
		inviteHash, err = hashInviteCode(*editable.InviteCode)
		if err != nil {
			return nil, err
		}
	}
	now := m.now().UTC()
	row := &contestRow{
		CreatorID: input.CreatorID, Status: ContestStatusDraft, Revision: 1,
		InviteCodeHash: inviteHash, CreatedAt: now, UpdatedAt: now,
	}
	applyEditableToContestRow(row, editable)
	err = m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(row).Error; err != nil {
			return fmt.Errorf("insert contest: %w", err)
		}
		state := &scoreboardStateRow{
			ContestID: row.ID, Mode: row.Mode, SourceVersion: 0, UpdatedAt: now,
		}
		if err := tx.Create(state).Error; err != nil {
			return fmt.Errorf("initialize contest scoreboard state: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return aggregateFromContestRow(row, now), nil
}

func (m *defaultContestModel) Update(ctx context.Context, input UpdateContestInput) (*ContestAggregate, error) {
	if input.ContestID <= 0 || input.ExpectedRevision <= 0 {
		return nil, fmt.Errorf("%w: contest id and expected revision must be positive", ErrInvalidContestInput)
	}
	if err := validateContestPatch(input.Patch); err != nil {
		return nil, err
	}
	inviteChanged := input.Patch.InviteCode != nil
	var replacementInviteHash *string
	if inviteChanged && *input.Patch.InviteCode != nil {
		var err error
		replacementInviteHash, err = hashInviteCode(**input.Patch.InviteCode)
		if err != nil {
			return nil, err
		}
	}

	var result *ContestAggregate
	err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, err := lockContest(tx, input.ContestID, true)
		if err != nil {
			return err
		}
		if row.DeletedAt != nil || row.Status == ContestStatusDeleted {
			return ErrContestAlreadyDeleted
		}
		if row.Revision != input.ExpectedRevision {
			return ErrContestRevisionConflict
		}
		now := m.now().UTC()
		effective := EffectiveContestStatus(row.Status, row.StartAt, row.EndAt, now)
		if (row.Status != ContestStatusDraft && row.Status != ContestStatusPublished) ||
			effective == ContestStatusRunning || effective == ContestStatusFinished {
			return ErrContestStateConflict
		}

		editable := editableContestFromRow(row)
		applyContestPatch(&editable, input.Patch)
		inviteConfigured := row.InviteCodeHash != nil
		if inviteChanged {
			inviteConfigured = replacementInviteHash != nil
		}
		if err := validateEditableContest(&editable, inviteConfigured); err != nil {
			return err
		}
		if row.Status == ContestStatusPublished &&
			EffectiveContestStatus(row.Status, editable.StartAt, editable.EndAt, now) != ContestStatusPublished {
			return ErrContestStateConflict
		}

		if editable.Mode != row.Mode {
			state, err := lockScoreboardState(tx, row.ID)
			if err != nil {
				return err
			}
			if state.SourceVersion != 0 {
				return ErrContestStateConflict
			}
			var existingProblems []contestProblemRow
			if err := tx.Where("contest_id = ?", row.ID).Find(&existingProblems).Error; err != nil {
				return fmt.Errorf("load contest problems for mode update: %w", err)
			}
			problems := make([]ContestProblem, 0, len(existingProblems))
			for _, problem := range existingProblems {
				problems = append(problems, ContestProblem{ProblemID: problem.ProblemID, Label: problem.Label,
					DisplayOrder: problem.DisplayOrder, MaxScore: problem.MaxScore})
			}
			if _, err := validateContestProblems(problems, editable.Mode); err != nil {
				return ErrContestStateConflict
			}
			if err := tx.Model(&scoreboardStateRow{}).Where("contest_id = ?", row.ID).
				Updates(map[string]any{"mode": editable.Mode, "updated_at": now}).Error; err != nil {
				return fmt.Errorf("update scoreboard mode: %w", err)
			}
		}

		applyEditableToContestRow(row, editable)
		if inviteChanged {
			row.InviteCodeHash = replacementInviteHash
		}
		row.Revision++
		row.UpdatedAt = now
		if err := conditionalContestUpdate(tx, row, input.ExpectedRevision, false); err != nil {
			return err
		}
		result = aggregateFromContestRow(row, now)
		return nil
	})
	return result, err
}

func (m *defaultContestModel) Publish(ctx context.Context, contestID, expectedRevision int64) (*ContestAggregate, error) {
	return m.transition(ctx, contestID, expectedRevision, ContestStatusPublished, "")
}

func (m *defaultContestModel) Cancel(ctx context.Context, contestID, expectedRevision int64, reason string) (*ContestAggregate, error) {
	reason, err := normalizeNote(reason, 500, "cancel reason")
	if err != nil {
		return nil, err
	}
	return m.transition(ctx, contestID, expectedRevision, ContestStatusCancelled, reason)
}

func (m *defaultContestModel) Archive(ctx context.Context, contestID, expectedRevision int64) (*ContestAggregate, error) {
	return m.transition(ctx, contestID, expectedRevision, ContestStatusArchived, "")
}

func (m *defaultContestModel) Restore(ctx context.Context, contestID, expectedRevision int64) (*ContestAggregate, error) {
	return m.transition(ctx, contestID, expectedRevision, 0, "")
}

func (m *defaultContestModel) SoftDelete(ctx context.Context, contestID, expectedRevision int64) (int64, error) {
	if contestID <= 0 || expectedRevision <= 0 {
		return 0, fmt.Errorf("%w: contest id and expected revision must be positive", ErrInvalidContestInput)
	}
	var revision int64
	err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, err := lockContest(tx, contestID, true)
		if err != nil {
			return err
		}
		if row.Status == ContestStatusDeleted || row.DeletedAt != nil {
			return ErrContestAlreadyDeleted
		}
		if row.Revision != expectedRevision {
			return ErrContestRevisionConflict
		}
		now := m.now().UTC()
		if EffectiveContestStatus(row.Status, row.StartAt, row.EndAt, now) == ContestStatusRunning {
			return ErrContestStateConflict
		}
		previous := row.Status
		row.Status = ContestStatusDeleted
		row.DeletedFromStatus = int64Pointer(previous)
		row.DeletedAt = utcTimePointer(&now)
		row.Revision++
		row.UpdatedAt = now
		if err := conditionalContestUpdate(tx, row, expectedRevision, false); err != nil {
			return err
		}
		revision = row.Revision
		return nil
	})
	return revision, err
}

// targetStatus 0 restores deleted_from_status.
func (m *defaultContestModel) transition(ctx context.Context, contestID, expectedRevision, targetStatus int64, reason string) (*ContestAggregate, error) {
	if contestID <= 0 || expectedRevision <= 0 {
		return nil, fmt.Errorf("%w: contest id and expected revision must be positive", ErrInvalidContestInput)
	}
	var result *ContestAggregate
	err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, err := lockContest(tx, contestID, true)
		if err != nil {
			return err
		}
		if row.Revision != expectedRevision {
			return ErrContestRevisionConflict
		}
		now := m.now().UTC()
		isRestore := targetStatus == 0
		if isRestore {
			if row.Status != ContestStatusDeleted || row.DeletedAt == nil {
				return ErrContestNotDeleted
			}
			if row.DeletedFromStatus == nil || !isPersistedActiveStatus(*row.DeletedFromStatus) {
				return ErrContestStateConflict
			}
			targetStatus = *row.DeletedFromStatus
		} else if row.Status == ContestStatusDeleted || row.DeletedAt != nil {
			return ErrContestAlreadyDeleted
		}

		if !isRestore {
			switch targetStatus {
			case ContestStatusPublished:
				if row.Status != ContestStatusDraft {
					return ErrContestStateConflict
				}
				var problemCount int64
				if err := tx.Model(&contestProblemRow{}).Where("contest_id = ?", row.ID).Count(&problemCount).Error; err != nil {
					return fmt.Errorf("count contest problems before publish: %w", err)
				}
				if problemCount == 0 {
					return ErrContestNotPublishable
				}
			case ContestStatusCancelled:
				if row.Status != ContestStatusPublished ||
					EffectiveContestStatus(row.Status, row.StartAt, row.EndAt, now) == ContestStatusFinished {
					return ErrContestStateConflict
				}
			case ContestStatusArchived:
				effective := EffectiveContestStatus(row.Status, row.StartAt, row.EndAt, now)
				if row.Status != ContestStatusCancelled && effective != ContestStatusFinished {
					return ErrContestStateConflict
				}
			default:
				return ErrContestStateConflict
			}
		}

		row.Status = targetStatus
		row.Revision++
		row.UpdatedAt = now
		if isRestore {
			row.DeletedAt = nil
			row.DeletedFromStatus = nil
		} else {
			switch targetStatus {
			case ContestStatusPublished:
				row.PublishedAt = utcTimePointer(&now)
				row.CancelReason = ""
			case ContestStatusCancelled:
				row.CancelledAt = utcTimePointer(&now)
				row.CancelReason = reason
			case ContestStatusArchived:
				row.ArchivedAt = utcTimePointer(&now)
			}
		}
		if err := conditionalContestUpdate(tx, row, expectedRevision, isRestore); err != nil {
			return err
		}
		result = aggregateFromContestRow(row, now)
		return nil
	})
	return result, err
}

func (m *defaultContestModel) GetForEdit(ctx context.Context, contestID int64) (*ContestAggregate, error) {
	if contestID <= 0 {
		return nil, fmt.Errorf("%w: contest id must be positive", ErrInvalidContestInput)
	}
	row, err := findContest(m.db.WithContext(ctx), contestID, true)
	if err != nil {
		return nil, err
	}
	return aggregateFromContestRow(row, m.now().UTC()), nil
}

func (m *defaultContestModel) GetDetail(ctx context.Context, contestID int64, at time.Time) (*ContestDetailRecord, error) {
	if contestID <= 0 {
		return nil, fmt.Errorf("%w: contest id must be positive", ErrInvalidContestInput)
	}
	row, err := findContest(m.db.WithContext(ctx), contestID, false)
	if err != nil {
		return nil, err
	}
	at = m.at(at)
	return detailFromContestRow(row, at), nil
}

func (m *defaultContestModel) ListPublic(ctx context.Context, filter ContestListFilter) ([]ContestSummary, int64, error) {
	return m.listContests(ctx, filter, true)
}

func (m *defaultContestModel) ListForManagement(ctx context.Context, filter ContestListFilter) ([]ContestSummary, int64, error) {
	return m.listContests(ctx, filter, false)
}

func (m *defaultContestModel) listContests(ctx context.Context, filter ContestListFilter, publicOnly bool) ([]ContestSummary, int64, error) {
	var rows []ContestSummary
	var total int64
	err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		rows, total, err = m.listContestRows(tx, filter, publicOnly)
		return err
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return rows, total, err
}

func (m *defaultContestModel) listContestRows(db *gorm.DB, filter ContestListFilter, publicOnly bool) ([]ContestSummary, int64, error) {
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	at := m.at(filter.At)
	if filter.Mode != nil && !validContestMode(*filter.Mode) {
		return nil, 0, fmt.Errorf("%w: invalid contest mode filter", ErrInvalidContestInput)
	}
	if filter.Status != nil && !validFilterStatus(*filter.Status) {
		return nil, 0, fmt.Errorf("%w: invalid contest status filter", ErrInvalidContestInput)
	}
	if filter.CreatorID != nil && *filter.CreatorID <= 0 {
		return nil, 0, fmt.Errorf("%w: creator id filter must be positive", ErrInvalidContestInput)
	}

	base := db.Model(&contestRow{})
	if publicOnly {
		base = base.Where("visibility = ? AND deleted_at IS NULL AND status <> ?", ContestVisibilityPublic, ContestStatusDraft)
	} else if filter.Status != nil && *filter.Status == ContestStatusDeleted {
		base = base.Where("status = ? AND deleted_at IS NOT NULL", ContestStatusDeleted)
	} else {
		base = base.Where("deleted_at IS NULL")
	}
	if filter.Status != nil {
		base = applyContestStatusFilter(base, *filter.Status, at)
	}
	if filter.Mode != nil {
		base = base.Where("mode = ?", *filter.Mode)
	}
	if filter.CreatorID != nil {
		base = base.Where("creator_id = ?", *filter.CreatorID)
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		base = base.Where("title LIKE ?", "%"+keyword+"%")
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count contests: %w", err)
	}
	if total == 0 {
		return []ContestSummary{}, 0, nil
	}
	var rows []contestRow
	if err := base.Order("start_at DESC, id DESC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("list contests: %w", err)
	}
	result := make([]ContestSummary, 0, len(rows))
	for i := range rows {
		row := &rows[i]
		result = append(result, ContestSummary{
			ContestID: row.ID, Title: row.Title, Mode: row.Mode,
			Status:  EffectiveContestStatus(row.Status, row.StartAt, row.EndAt, at),
			StartAt: row.StartAt.UTC(), EndAt: row.EndAt.UTC(),
			RegistrationPolicy: row.RegistrationPolicy, Visibility: row.Visibility,
			RegistrationOpen: registrationOpen(row, at), ParticipantCount: row.ParticipantCount,
		})
	}
	return result, total, nil
}

func (m *defaultContestModel) SetProblems(ctx context.Context, input SetContestProblemsInput) (*ContestAggregate, []ContestProblem, error) {
	if input.ContestID <= 0 || input.ExpectedRevision <= 0 {
		return nil, nil, fmt.Errorf("%w: contest id and expected revision must be positive", ErrInvalidContestInput)
	}
	var aggregate *ContestAggregate
	var normalized []ContestProblem
	err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, err := lockContest(tx, input.ContestID, false)
		if err != nil {
			return err
		}
		if row.Revision != input.ExpectedRevision {
			return ErrContestRevisionConflict
		}
		now := m.now().UTC()
		effective := EffectiveContestStatus(row.Status, row.StartAt, row.EndAt, now)
		if (row.Status != ContestStatusDraft && row.Status != ContestStatusPublished) ||
			effective == ContestStatusRunning || effective == ContestStatusFinished {
			return ErrContestStateConflict
		}
		state, err := lockScoreboardState(tx, row.ID)
		if err != nil {
			return err
		}
		if state.SourceVersion != 0 {
			return ErrContestStateConflict
		}
		normalized, err = validateContestProblems(input.Problems, row.Mode)
		if err != nil {
			return err
		}
		if err := tx.Where("contest_id = ?", row.ID).Delete(&contestProblemRow{}).Error; err != nil {
			return fmt.Errorf("replace contest problems: %w", err)
		}
		rows := make([]contestProblemRow, 0, len(normalized))
		for i := range normalized {
			problem := normalized[i]
			problem.ContestID = row.ID
			normalized[i] = problem
			rows = append(rows, contestProblemRow{ContestID: row.ID, ProblemID: problem.ProblemID,
				Label: problem.Label, DisplayOrder: problem.DisplayOrder, MaxScore: problem.MaxScore,
				CreatedAt: now, UpdatedAt: now})
		}
		if len(rows) > 0 {
			if err := tx.Create(&rows).Error; err != nil {
				if isDuplicateKeyError(err) {
					return ErrContestProblemConflict
				}
				return fmt.Errorf("insert contest problems: %w", err)
			}
		}
		row.Revision++
		row.UpdatedAt = now
		if err := conditionalContestUpdate(tx, row, input.ExpectedRevision, false); err != nil {
			return err
		}
		aggregate = aggregateFromContestRow(row, now)
		return nil
	})
	return aggregate, normalized, err
}

func (m *defaultContestModel) ListProblems(ctx context.Context, contestID int64) ([]ContestProblem, error) {
	if contestID <= 0 {
		return nil, fmt.Errorf("%w: contest id must be positive", ErrInvalidContestInput)
	}
	if _, err := findContest(m.db.WithContext(ctx), contestID, false); err != nil {
		return nil, err
	}
	var rows []contestProblemRow
	if err := m.db.WithContext(ctx).Where("contest_id = ?", contestID).
		Order("display_order ASC, problem_id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list contest problems: %w", err)
	}
	result := make([]ContestProblem, 0, len(rows))
	for _, row := range rows {
		result = append(result, ContestProblem{ContestID: row.ContestID, ProblemID: row.ProblemID,
			Label: row.Label, DisplayOrder: row.DisplayOrder, MaxScore: row.MaxScore})
	}
	return result, nil
}

func applyEditableToContestRow(row *contestRow, editable EditableContest) {
	row.Title = editable.Title
	row.Description = editable.Description
	row.Mode = editable.Mode
	row.StartAt = editable.StartAt.UTC()
	row.EndAt = editable.EndAt.UTC()
	row.RegistrationStartAt = utcTimePointer(editable.RegistrationStartAt)
	row.RegistrationEndAt = utcTimePointer(editable.RegistrationEndAt)
	row.RegistrationPolicy = editable.RegistrationPolicy
	row.Visibility = editable.Visibility
	row.ScoreboardVisibility = editable.ScoreboardVisibility
	row.ScoreboardFreezeAt = utcTimePointer(editable.ScoreboardFreezeAt)
	row.ScoreboardFreezeBeforeEndMS = nil
	if editable.ScoreboardFreezeBeforeEnd != nil {
		milliseconds := editable.ScoreboardFreezeBeforeEnd.Milliseconds()
		row.ScoreboardFreezeBeforeEndMS = &milliseconds
	}
}

func lockContest(tx *gorm.DB, contestID int64, includeDeleted bool) (*contestRow, error) {
	query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", contestID)
	if !includeDeleted {
		query = query.Where("deleted_at IS NULL")
	}
	row := new(contestRow)
	err := query.Take(row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrContestNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock contest: %w", err)
	}
	return row, nil
}

func findContest(db *gorm.DB, contestID int64, includeDeleted bool) (*contestRow, error) {
	query := db.Where("id = ?", contestID)
	if !includeDeleted {
		query = query.Where("deleted_at IS NULL")
	}
	row := new(contestRow)
	err := query.Take(row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrContestNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find contest: %w", err)
	}
	return row, nil
}

func conditionalContestUpdate(tx *gorm.DB, row *contestRow, expectedRevision int64, includeDeleted bool) error {
	query := tx.Model(&contestRow{}).Where("id = ? AND revision = ?", row.ID, expectedRevision)
	if !includeDeleted {
		query = query.Where("deleted_at IS NULL")
	}
	result := query.Updates(contestRowUpdates(row))
	if result.Error != nil {
		return fmt.Errorf("update contest: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrContestRevisionConflict
	}
	return nil
}

func isPersistedActiveStatus(status int64) bool {
	return status == ContestStatusDraft || status == ContestStatusPublished ||
		status == ContestStatusCancelled || status == ContestStatusArchived
}

func validFilterStatus(status int64) bool {
	return status >= ContestStatusDraft && status <= ContestStatusDeleted
}

func applyContestStatusFilter(db *gorm.DB, status int64, at time.Time) *gorm.DB {
	switch status {
	case ContestStatusPublished:
		return db.Where("status = ? AND start_at > ?", ContestStatusPublished, at)
	case ContestStatusRunning:
		return db.Where("status = ? AND start_at <= ? AND end_at > ?", ContestStatusPublished, at, at)
	case ContestStatusFinished:
		return db.Where("status = ? AND end_at <= ?", ContestStatusPublished, at)
	default:
		return db.Where("status = ?", status)
	}
}
