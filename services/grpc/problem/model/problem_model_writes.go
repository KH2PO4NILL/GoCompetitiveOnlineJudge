package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (m *defaultProblemModel) Create(ctx context.Context, input CreateProblemInput) (*ProblemAggregate, error) {
	if input.AuthorID <= 0 {
		return nil, fmt.Errorf("%w: author id must be positive", ErrInvalidProblemInput)
	}
	editable := input.Editable
	if err := validateEditable(&editable); err != nil {
		return nil, err
	}
	samplesJSON, languagesJSON, err := editableJSON(editable)
	if err != nil {
		return nil, err
	}
	now := m.now()
	problem := &Problems{
		AuthorId: input.AuthorID,
		Title:    editable.Title, Description: editable.Description,
		InputDescription: editable.InputDescription, OutputDescription: editable.OutputDescription,
		SamplesJson: samplesJSON, Hint: editable.Hint,
		Difficulty: editable.Difficulty, Source: editable.Source,
		TimeLimitMs: editable.TimeLimitMS, MemoryLimitMb: editable.MemoryLimitMB,
		AllowedLanguagesJson: languagesJSON, CheckerType: editable.CheckerType,
		CheckerFileKey:    pointerToNullString(editable.CheckerFileKey),
		InteractorFileKey: pointerToNullString(editable.InteractorFileKey),
		Status:            ProblemStatusDraft, Visibility: editable.Visibility, Revision: 1,
		CreatedAt: now, UpdatedAt: now,
	}
	err = m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(problem).Error; err != nil {
			return fmt.Errorf("insert problem: %w", err)
		}
		_, err := insertRevision(tx, problem, editable, input.AuthorID, now)
		return err
	})
	if err != nil {
		return nil, err
	}
	return aggregateFromProblem(problem, editable.Tags)
}

func (m *defaultProblemModel) Update(ctx context.Context, input UpdateProblemInput) (*ProblemAggregate, error) {
	if input.ProblemID <= 0 || input.ExpectedRevision <= 0 || input.ActorID <= 0 {
		return nil, fmt.Errorf("%w: problem, revision and actor ids must be positive", ErrInvalidProblemInput)
	}
	var result *ProblemAggregate
	err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		problem, err := lockProblem(tx, input.ProblemID)
		if err != nil {
			return err
		}
		if problem.DeletedAt.Valid {
			return ErrProblemAlreadyDeleted
		}
		if problem.Revision != input.ExpectedRevision {
			return ErrProblemRevisionConflict
		}
		tags, err := loadTagsForProblemRevision(tx, problem.Id, problem.Revision)
		if err != nil {
			return err
		}
		editable, err := decodeEditable(problem, tags)
		if err != nil {
			return err
		}
		applyPatch(&editable, input.Patch)
		if err := validateEditable(&editable); err != nil {
			return err
		}
		now := m.now()
		problem.Revision++
		problem.UpdatedAt = now
		if err := updateCurrentProblem(tx, problem, editable, input.ExpectedRevision, false); err != nil {
			return err
		}
		if _, err := insertRevision(tx, problem, editable, input.ActorID, now); err != nil {
			return err
		}
		result, err = aggregateFromProblem(problem, editable.Tags)
		return err
	})
	return result, err
}

func (m *defaultProblemModel) Publish(ctx context.Context, problemID, expectedRevision, actorID int64) (*ProblemAggregate, error) {
	return m.transition(ctx, problemID, expectedRevision, actorID, ProblemStatusPublished)
}

func (m *defaultProblemModel) Archive(ctx context.Context, problemID, expectedRevision, actorID int64) (*ProblemAggregate, error) {
	return m.transition(ctx, problemID, expectedRevision, actorID, ProblemStatusArchived)
}

func (m *defaultProblemModel) Restore(ctx context.Context, problemID, expectedRevision, actorID int64) (*ProblemAggregate, error) {
	return m.transition(ctx, problemID, expectedRevision, actorID, 0)
}

func (m *defaultProblemModel) SoftDelete(ctx context.Context, problemID, expectedRevision int64) (int64, error) {
	if problemID <= 0 || expectedRevision <= 0 {
		return 0, fmt.Errorf("%w: problem and revision ids must be positive", ErrInvalidProblemInput)
	}
	var newRevision int64
	err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		problem, err := lockProblem(tx, problemID)
		if err != nil {
			return err
		}
		if problem.DeletedAt.Valid {
			return ErrProblemAlreadyDeleted
		}
		if problem.Revision != expectedRevision {
			return ErrProblemRevisionConflict
		}
		tags, err := loadTagsForProblemRevision(tx, problem.Id, problem.Revision)
		if err != nil {
			return err
		}
		editable, err := decodeEditable(problem, tags)
		if err != nil {
			return err
		}
		now := m.now()
		previousStatus := problem.Status
		problem.Status = ProblemStatusDeleted
		problem.DeletedFromStatus = sql.NullInt64{Int64: previousStatus, Valid: true}
		problem.DeletedAt = sql.NullTime{Time: now, Valid: true}
		problem.Revision++
		problem.UpdatedAt = now
		updates := currentProblemUpdates(problem, editable)
		updates["deleted_from_status"] = previousStatus
		updates["deleted_at"] = now
		updates["status"] = ProblemStatusDeleted
		if err := conditionalProblemUpdate(tx, problem.Id, expectedRevision, false, updates); err != nil {
			return err
		}
		if _, err := insertRevision(tx, problem, editable, problem.AuthorId, now); err != nil {
			return err
		}
		newRevision = problem.Revision
		return nil
	})
	return newRevision, err
}

// targetStatus 0 means restore to deleted_from_status.
func (m *defaultProblemModel) transition(ctx context.Context, problemID, expectedRevision, actorID, targetStatus int64) (*ProblemAggregate, error) {
	if problemID <= 0 || expectedRevision <= 0 || actorID <= 0 {
		return nil, fmt.Errorf("%w: problem, revision and actor ids must be positive", ErrInvalidProblemInput)
	}
	var result *ProblemAggregate
	err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		problem, err := lockProblem(tx, problemID)
		if err != nil {
			return err
		}
		if problem.Revision != expectedRevision {
			return ErrProblemRevisionConflict
		}

		isRestore := targetStatus == 0
		if isRestore {
			if !problem.DeletedAt.Valid || problem.Status != ProblemStatusDeleted {
				return ErrProblemNotDeleted
			}
			targetStatus = ProblemStatusDraft
			if problem.DeletedFromStatus.Valid && problem.DeletedFromStatus.Int64 >= ProblemStatusDraft && problem.DeletedFromStatus.Int64 <= ProblemStatusArchived {
				targetStatus = problem.DeletedFromStatus.Int64
			}
		} else if problem.DeletedAt.Valid {
			return ErrProblemAlreadyDeleted
		}

		if !isRestore {
			switch targetStatus {
			case ProblemStatusPublished:
				if problem.Status != ProblemStatusDraft && problem.Status != ProblemStatusArchived && problem.Status != ProblemStatusPublished {
					return ErrProblemStateConflict
				}
				if problem.Status == ProblemStatusPublished && problem.LatestPublishedRevision.Valid && problem.LatestPublishedRevision.Int64 == problem.Revision {
					return ErrProblemStateConflict
				}
				if !problem.CurrentTestDataVersionId.Valid {
					return ErrProblemNotPublishable
				}
			case ProblemStatusArchived:
				if problem.Status != ProblemStatusPublished {
					return ErrProblemStateConflict
				}
			default:
				return ErrProblemStateConflict
			}
		}

		tags, err := loadTagsForProblemRevision(tx, problem.Id, problem.Revision)
		if err != nil {
			return err
		}
		editable, err := decodeEditable(problem, tags)
		if err != nil {
			return err
		}
		now := m.now()
		problem.Status = targetStatus
		problem.Revision++
		problem.UpdatedAt = now
		updates := currentProblemUpdates(problem, editable)
		updates["status"] = targetStatus
		if isRestore {
			problem.DeletedAt = sql.NullTime{}
			problem.DeletedFromStatus = sql.NullInt64{}
			updates["deleted_at"] = nil
			updates["deleted_from_status"] = nil
		}
		if targetStatus == ProblemStatusPublished {
			problem.LatestPublishedRevision = sql.NullInt64{Int64: problem.Revision, Valid: true}
			problem.PublishedAt = sql.NullTime{Time: now, Valid: true}
			updates["latest_published_revision"] = problem.Revision
			updates["published_at"] = now
		}
		// The published pointer has a composite foreign key to the immutable
		// snapshot, so create that snapshot before moving the root pointer.
		revisionInserted := false
		if targetStatus == ProblemStatusPublished {
			if _, err := insertRevision(tx, problem, editable, actorID, now); err != nil {
				return err
			}
			revisionInserted = true
		}
		if err := conditionalProblemUpdate(tx, problem.Id, expectedRevision, isRestore, updates); err != nil {
			return err
		}
		if !revisionInserted {
			if _, err := insertRevision(tx, problem, editable, actorID, now); err != nil {
				return err
			}
		}
		if targetStatus == ProblemStatusPublished {
			legacy := &ProblemJudgeRevisions{
				ProblemId: problem.Id, Revision: problem.Revision,
				TimeLimitMs: problem.TimeLimitMs, MemoryLimitMb: problem.MemoryLimitMb,
				AllowedLanguagesJson: problem.AllowedLanguagesJson, CheckerType: problem.CheckerType,
				CheckerFileKey: problem.CheckerFileKey, InteractorFileKey: problem.InteractorFileKey,
				TestDataVersionId: problem.CurrentTestDataVersionId.Int64, PublishedAt: now,
			}
			if err := tx.Create(legacy).Error; err != nil {
				return fmt.Errorf("insert published judge revision: %w", err)
			}
		}
		result, err = aggregateFromProblem(problem, editable.Tags)
		return err
	})
	return result, err
}

func lockProblem(tx *gorm.DB, problemID int64) (*Problems, error) {
	problem := new(Problems)
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", problemID).Take(problem).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrProblemNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock problem: %w", err)
	}
	return problem, nil
}

func loadTagsForProblemRevision(tx *gorm.DB, problemID, revision int64) ([]string, error) {
	var revisionID int64
	err := tx.Table("problem_revisions").Select("id").
		Where("problem_id = ? AND revision = ?", problemID, revision).
		Scan(&revisionID).Error
	if err != nil {
		return nil, fmt.Errorf("find current problem revision: %w", err)
	}
	if revisionID == 0 {
		return nil, fmt.Errorf("%w: current revision snapshot is missing", ErrProblemStateConflict)
	}
	tags, err := loadTagsByRevisionIDs(tx, []int64{revisionID})
	if err != nil {
		return nil, err
	}
	return tags[revisionID], nil
}

func updateCurrentProblem(tx *gorm.DB, problem *Problems, editable EditableProblem, expectedRevision int64, includeDeleted bool) error {
	updates := currentProblemUpdates(problem, editable)
	if err := conditionalProblemUpdate(tx, problem.Id, expectedRevision, includeDeleted, updates); err != nil {
		return err
	}
	samplesJSON, languagesJSON, err := editableJSON(editable)
	if err != nil {
		return err
	}
	problem.Title = editable.Title
	problem.Description = editable.Description
	problem.InputDescription = editable.InputDescription
	problem.OutputDescription = editable.OutputDescription
	problem.SamplesJson = samplesJSON
	problem.Hint = editable.Hint
	problem.Difficulty = editable.Difficulty
	problem.Source = editable.Source
	problem.TimeLimitMs = editable.TimeLimitMS
	problem.MemoryLimitMb = editable.MemoryLimitMB
	problem.AllowedLanguagesJson = languagesJSON
	problem.CheckerType = editable.CheckerType
	problem.CheckerFileKey = pointerToNullString(editable.CheckerFileKey)
	problem.InteractorFileKey = pointerToNullString(editable.InteractorFileKey)
	problem.Visibility = editable.Visibility
	return nil
}

func currentProblemUpdates(problem *Problems, editable EditableProblem) map[string]any {
	samplesJSON, _ := encodeJSON(editable.Samples)
	languagesJSON, _ := encodeJSON(editable.AllowedLanguages)
	return map[string]any{
		"title": editable.Title, "description": editable.Description,
		"input_description": editable.InputDescription, "output_description": editable.OutputDescription,
		"samples_json": samplesJSON, "hint": editable.Hint,
		"difficulty": editable.Difficulty, "source": editable.Source,
		"time_limit_ms": editable.TimeLimitMS, "memory_limit_mb": editable.MemoryLimitMB,
		"allowed_languages_json": languagesJSON, "checker_type": editable.CheckerType,
		"checker_file_key": editable.CheckerFileKey, "interactor_file_key": editable.InteractorFileKey,
		"visibility": editable.Visibility, "revision": problem.Revision, "updated_at": problem.UpdatedAt,
	}
}

func conditionalProblemUpdate(tx *gorm.DB, problemID, expectedRevision int64, includeDeleted bool, updates map[string]any) error {
	query := tx.Model(&Problems{}).Where("id = ? AND revision = ?", problemID, expectedRevision)
	if !includeDeleted {
		query = query.Where("deleted_at IS NULL")
	}
	result := query.Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update problem: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrProblemRevisionConflict
	}
	return nil
}
