package model

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (m *defaultSubmitModel) ClaimOutboxEvents(ctx context.Context, input ClaimOutboxInput) ([]OutboxEvent, error) {
	input.WorkerID = strings.TrimSpace(input.WorkerID)
	if input.WorkerID == "" || len(input.WorkerID) > 128 || !validASCII(input.WorkerID) ||
		input.Limit <= 0 || input.Limit > maxOutboxClaimBatch || input.LockDuration <= 0 || input.LockDuration > time.Hour {
		return nil, fmt.Errorf("%w: invalid outbox claim input", ErrInvalidSubmissionInput)
	}
	if input.EventType != 0 && input.EventType != OutboxEventJudgeTask && input.EventType != OutboxEventSubmissionJudged {
		return nil, ErrInvalidSubmissionInput
	}
	result := make([]OutboxEvent, 0)
	err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		at := m.now().UTC()
		lockedUntil := at.Add(input.LockDuration)
		var rows []outboxRow
		query := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
		if input.EventType != 0 {
			query = query.Where("event_type = ?", input.EventType)
		}
		if err := query.
			Where("status = ? AND available_at <= ? AND (locked_by IS NULL OR locked_until <= ?)", OutboxStatusPending, at, at).
			Order("available_at ASC, id ASC").Limit(input.Limit).Find(&rows).Error; err != nil {
			return normalizeDatabaseError(err)
		}
		if len(rows) == 0 {
			return nil
		}
		ids := make([]int64, len(rows))
		for index := range rows {
			ids[index] = rows[index].ID
		}
		update := tx.Model(&outboxRow{}).Where("id IN ? AND status = ?", ids, OutboxStatusPending).
			Updates(map[string]any{
				"locked_by": input.WorkerID, "locked_until": lockedUntil,
				"publish_attempts": gorm.Expr("publish_attempts + 1"),
			})
		if update.Error != nil {
			return normalizeDatabaseError(update.Error)
		}
		if update.RowsAffected != int64(len(rows)) {
			return ErrOutboxStateConflict
		}
		result = make([]OutboxEvent, len(rows))
		for index := range rows {
			rows[index].LockedBy = stringPointer(input.WorkerID)
			rows[index].LockedUntil = utcTimePointer(&lockedUntil)
			rows[index].PublishAttempts++
			result[index] = outboxEventFromRow(rows[index])
		}
		return nil
	})
	if err != nil {
		return nil, normalizeDatabaseError(err)
	}
	return result, nil
}

func (m *defaultSubmitModel) JudgeOutboxBacklog(ctx context.Context) (int64, error) {
	var count int64
	err := m.db.WithContext(ctx).Model(&outboxRow{}).Where("status = ? AND event_type = ?", OutboxStatusPending, OutboxEventJudgeTask).Count(&count).Error
	return count, normalizeDatabaseError(err)
}

func (m *defaultSubmitModel) MarkOutboxPublished(ctx context.Context, input MarkOutboxPublishedInput) error {
	input.EventID = strings.TrimSpace(input.EventID)
	input.WorkerID = strings.TrimSpace(input.WorkerID)
	if len(input.EventID) != 36 || !validASCII(input.EventID) || input.WorkerID == "" ||
		len(input.WorkerID) > 128 || !validASCII(input.WorkerID) {
		return fmt.Errorf("%w: invalid outbox publish input", ErrInvalidSubmissionInput)
	}
	return normalizeDatabaseError(m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row outboxRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("event_id = ?", input.EventID).Take(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrOutboxEventNotFound
			}
			return normalizeDatabaseError(err)
		}
		if row.Status == OutboxStatusPublished {
			return nil
		}
		if row.Status != OutboxStatusPending || row.LockedBy == nil || *row.LockedBy != input.WorkerID {
			return ErrOutboxStateConflict
		}
		publishedAt := m.now().UTC()
		update := tx.Model(&outboxRow{}).
			Where("id = ? AND status = ? AND locked_by = ?", row.ID, OutboxStatusPending, input.WorkerID).
			Updates(map[string]any{
				"status": OutboxStatusPublished, "published_at": publishedAt,
				"locked_by": nil, "locked_until": nil, "last_error": "",
			})
		if update.Error != nil {
			return normalizeDatabaseError(update.Error)
		}
		if update.RowsAffected != 1 {
			return ErrOutboxStateConflict
		}
		return nil
	}))
}

func (m *defaultSubmitModel) MarkOutboxFailed(ctx context.Context, input MarkOutboxFailedInput) error {
	input.EventID = strings.TrimSpace(input.EventID)
	input.WorkerID = strings.TrimSpace(input.WorkerID)
	input.ErrorCode = strings.ToLower(strings.TrimSpace(input.ErrorCode))
	if len(input.EventID) != 36 || !validASCII(input.EventID) || input.WorkerID == "" ||
		len(input.WorkerID) > 128 || !validASCII(input.WorkerID) || !validOutboxErrorCode(input.ErrorCode) ||
		input.MaxAttempts <= 0 {
		return fmt.Errorf("%w: invalid outbox failure input", ErrInvalidSubmissionInput)
	}
	retryAt := normalizedTime(input.RetryAt, m.now())
	return normalizeDatabaseError(m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row outboxRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("event_id = ?", input.EventID).Take(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrOutboxEventNotFound
			}
			return normalizeDatabaseError(err)
		}
		if row.Status != OutboxStatusPending {
			return ErrOutboxStateConflict
		}
		if row.LockedBy == nil || *row.LockedBy != input.WorkerID {
			return ErrOutboxStateConflict
		}
		status := OutboxStatusPending
		if row.PublishAttempts >= input.MaxAttempts {
			status = OutboxStatusDead
		}
		updates := map[string]any{
			"status": status, "locked_by": nil, "locked_until": nil, "last_error": input.ErrorCode,
		}
		if status == OutboxStatusPending {
			updates["available_at"] = retryAt
		}
		return normalizeDatabaseError(tx.Model(&outboxRow{}).Where("id = ?", row.ID).Updates(updates).Error)
	}))
}
