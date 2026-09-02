package model

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

func (m *defaultSubmitModel) RejudgeSubmission(ctx context.Context, input RejudgeSubmissionInput) (*RejudgeResult, error) {
	results, err := m.BatchRejudgeSubmissions(ctx, BatchRejudgeInput{
		SubmissionIDs: []int64{input.SubmissionID}, RequestedBy: input.RequestedBy,
		Reason: input.Reason,
	})
	if err != nil {
		return nil, err
	}
	return &results[0], nil
}

func (m *defaultSubmitModel) BatchRejudgeSubmissions(ctx context.Context, input BatchRejudgeInput) ([]RejudgeResult, error) {
	ids, err := uniqueSortedPositiveIDs(input.SubmissionIDs, maxBatchRejudgeSize)
	if err != nil {
		return nil, err
	}
	if input.RequestedBy <= 0 {
		return nil, fmt.Errorf("%w: rejudge requester must be positive", ErrInvalidSubmissionInput)
	}
	reason, err := normalizeRejudgeReason(input.Reason)
	if err != nil {
		return nil, err
	}
	results := make([]RejudgeResult, 0, len(ids))
	err = m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		at := m.now().UTC()
		for _, submissionID := range ids {
			submission, current, lockErr := m.submissionForUpdate(tx, submissionID)
			if lockErr != nil {
				return lockErr
			}
			attempt, createErr := supersedeAndCreateAttemptTx(
				tx, submission, current, AttemptKindRejudge, int64Pointer(input.RequestedBy), reason, at,
			)
			if createErr != nil {
				return createErr
			}
			aggregate, loadErr := getSubmissionAggregateTx(tx, submission)
			if loadErr != nil {
				return loadErr
			}
			results = append(results, RejudgeResult{
				SubmissionID: submission.ID, AttemptID: attempt.ID, AttemptNo: attempt.AttemptNo,
				Submission: aggregate,
			})
		}
		return nil
	})
	if err != nil {
		return nil, normalizeDatabaseError(err)
	}
	return results, nil
}

func supersedeAndCreateAttemptTx(
	tx *gorm.DB,
	submission *submissionRow,
	current *attemptRow,
	kind int64,
	requestedBy *int64,
	reason string,
	at time.Time,
) (*attemptRow, error) {
	if current.Status == AttemptStatusSuperseded {
		return nil, fmt.Errorf("%w: current attempt is already superseded", ErrSubmissionStateConflict)
	}
	if submission.AttemptCount == int32(^uint32(0)>>1) {
		return nil, fmt.Errorf("%w: attempt counter exhausted", ErrSubmissionStateConflict)
	}
	if err := tx.Model(&attemptRow{}).Where("id = ? AND submission_id = ?", current.ID, submission.ID).
		Updates(map[string]any{
			"status": AttemptStatusSuperseded, "superseded_at": at, "updated_at": at,
		}).Error; err != nil {
		return nil, normalizeDatabaseError(err)
	}
	current.Status = AttemptStatusSuperseded
	current.SupersededAt = utcTimePointer(&at)
	current.UpdatedAt = at

	attemptNo := submission.AttemptCount + 1
	attempt := createAttemptRow(submission.ID, attemptNo, kind, cloneInt64Pointer(requestedBy), reason, at)
	if err := tx.Create(attempt).Error; err != nil {
		return nil, normalizeDatabaseError(err)
	}
	if err := tx.Model(&submissionRow{}).Where("id = ?", submission.ID).
		Updates(map[string]any{
			"status": SubmissionStatusQueued, "current_attempt_id": attempt.ID, "attempt_count": attemptNo,
			"verdict": nil, "score": nil, "max_score": nil, "judging_started_at": nil,
			"finished_at": nil, "updated_at": at, "version": gorm.Expr("version + 1"),
		}).Error; err != nil {
		return nil, normalizeDatabaseError(err)
	}
	submission.Status = SubmissionStatusQueued
	submission.CurrentAttemptID = int64Pointer(attempt.ID)
	submission.AttemptCount = attemptNo
	submission.Verdict, submission.Score, submission.MaxScore = nil, nil, nil
	submission.JudgingStartedAt, submission.FinishedAt = nil, nil
	submission.UpdatedAt = at
	submission.Version++
	if err := createJudgeTaskOutboxTx(tx, submission.ID, attempt.ID, at); err != nil {
		return nil, err
	}
	return attempt, nil
}

func (m *defaultSubmitModel) AcquireAttempt(ctx context.Context, input AcquireAttemptInput) (*AcquireAttemptResult, error) {
	input.WorkerID = strings.TrimSpace(input.WorkerID)
	input.DeliveryID = strings.TrimSpace(input.DeliveryID)
	if input.SubmissionID <= 0 || input.AttemptID <= 0 || input.WorkerID == "" || len(input.WorkerID) > 128 ||
		!validASCII(input.WorkerID) || len(input.DeliveryID) > 255 || (input.DeliveryID != "" && !validASCII(input.DeliveryID)) ||
		input.LeaseDuration <= 0 || input.LeaseDuration > 24*time.Hour {
		return nil, fmt.Errorf("%w: invalid acquire input", ErrInvalidSubmissionInput)
	}
	token, tokenHash, err := randomLeaseToken()
	if err != nil {
		return nil, err
	}
	result := &AcquireAttemptResult{SubmissionID: input.SubmissionID, AttemptID: input.AttemptID}
	err = m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		submission, findErr := findSubmissionRow(tx, input.SubmissionID, true)
		if findErr != nil {
			return findErr
		}
		if submission.CurrentAttemptID == nil || *submission.CurrentAttemptID != input.AttemptID {
			result.DenyReason = AcquireDenyStaleAttempt
			return nil
		}
		attempt, findErr := findAttemptRow(tx, input.AttemptID, true)
		if findErr != nil {
			return findErr
		}
		if attempt.SubmissionID != submission.ID {
			result.DenyReason = AcquireDenyStaleAttempt
			return nil
		}
		now := m.now().UTC()
		if submission.Status == SubmissionStatusFinished || attempt.Status == AttemptStatusFinished {
			result.DenyReason = AcquireDenyAlreadyFinished
			return nil
		}
		if attempt.Status == AttemptStatusSuperseded {
			result.DenyReason = AcquireDenyStaleAttempt
			return nil
		}
		if attempt.Status == AttemptStatusJudging {
			if attempt.LeaseExpiresAt != nil && now.Before(*attempt.LeaseExpiresAt) {
				result.DenyReason = AcquireDenyLeasedByOtherWorker
				return nil
			}
			return ErrLeaseExpired
		}
		if attempt.Status != AttemptStatusQueued || submission.Status != SubmissionStatusQueued {
			return ErrSubmissionStateConflict
		}
		expiresAt := now.Add(input.LeaseDuration)
		if err := tx.Model(&attemptRow{}).Where("id = ?", attempt.ID).Updates(map[string]any{
			"status": AttemptStatusJudging, "worker_id": input.WorkerID, "delivery_id": input.DeliveryID,
			"lease_token_hash": tokenHash, "lease_expires_at": expiresAt, "last_heartbeat_at": now,
			"started_at": now, "updated_at": now,
		}).Error; err != nil {
			return normalizeDatabaseError(err)
		}
		if err := tx.Model(&submissionRow{}).Where("id = ?", submission.ID).Updates(map[string]any{
			"status": SubmissionStatusJudging, "judging_started_at": now, "updated_at": now,
			"version": gorm.Expr("version + 1"),
		}).Error; err != nil {
			return normalizeDatabaseError(err)
		}
		result.Acquired = true
		result.DenyReason = AcquireDenyNone
		result.ProblemID = submission.ProblemID
		result.ProblemRevision = submission.ProblemRevision
		result.Language = submission.Language
		result.SourceCode = submission.SourceCode
		result.LeaseToken = token
		result.LeaseExpiresAt = utcTimePointer(&expiresAt)
		return nil
	})
	if err != nil {
		return nil, normalizeDatabaseError(err)
	}
	return result, nil
}

func (m *defaultSubmitModel) HeartbeatAttempt(ctx context.Context, input HeartbeatAttemptInput) (*HeartbeatResult, error) {
	input.WorkerID = strings.TrimSpace(input.WorkerID)
	if input.SubmissionID <= 0 || input.AttemptID <= 0 || input.WorkerID == "" || len(input.WorkerID) > 128 ||
		!validASCII(input.WorkerID) || input.LeaseToken == "" || len(input.LeaseToken) > 256 ||
		input.CompletedTestCases < 0 || input.TotalTestCases < 0 || input.CompletedTestCases > input.TotalTestCases ||
		input.LeaseDuration <= 0 || input.LeaseDuration > 24*time.Hour {
		return nil, fmt.Errorf("%w: invalid heartbeat input", ErrInvalidSubmissionInput)
	}
	var result *HeartbeatResult
	err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		submission, findErr := findSubmissionRow(tx, input.SubmissionID, true)
		if findErr != nil {
			return findErr
		}
		if submission.CurrentAttemptID == nil || *submission.CurrentAttemptID != input.AttemptID {
			return ErrStaleAttempt
		}
		attempt, findErr := findAttemptRow(tx, input.AttemptID, true)
		if findErr != nil {
			return findErr
		}
		if attempt.SubmissionID != submission.ID {
			return ErrStaleAttempt
		}
		if attempt.Status == AttemptStatusFinished {
			return ErrAttemptAlreadyFinished
		}
		if attempt.Status != AttemptStatusJudging {
			return ErrSubmissionStateConflict
		}
		if attempt.WorkerID != input.WorkerID || !sameDigest(attempt.LeaseTokenHash, leaseTokenDigest(input.LeaseToken)) {
			return ErrLeaseMismatch
		}
		now := m.now().UTC()
		if attempt.LeaseExpiresAt == nil || !now.Before(*attempt.LeaseExpiresAt) {
			return ErrLeaseExpired
		}
		if input.CompletedTestCases < attempt.CompletedTestCases {
			return fmt.Errorf("%w: completed test cases cannot decrease", ErrInvalidSubmissionInput)
		}
		expiresAt := now.Add(input.LeaseDuration)
		if expiresAt.Before(*attempt.LeaseExpiresAt) {
			expiresAt = attempt.LeaseExpiresAt.UTC()
		}
		if err := tx.Model(&attemptRow{}).Where("id = ?", attempt.ID).Updates(map[string]any{
			"completed_test_cases": input.CompletedTestCases, "last_heartbeat_at": now,
			"lease_expires_at": expiresAt, "updated_at": now,
		}).Error; err != nil {
			return normalizeDatabaseError(err)
		}
		result = &HeartbeatResult{LeaseExpiresAt: expiresAt, CompletedTestCases: input.CompletedTestCases}
		return nil
	})
	if err != nil {
		return nil, normalizeDatabaseError(err)
	}
	return result, nil
}

func (m *defaultSubmitModel) ListExpiredLeases(ctx context.Context, before time.Time, limit int) ([]ExpiredLease, error) {
	if limit <= 0 || limit > maxExpiredLeaseBatch {
		return nil, fmt.Errorf("%w: expired lease limit must be 1-%d", ErrInvalidSubmissionInput, maxExpiredLeaseBatch)
	}
	before = normalizedTime(before, m.now())
	type expiredLeaseRow struct {
		SubmissionID   int64     `gorm:"column:submission_id"`
		AttemptID      int64     `gorm:"column:attempt_id"`
		AttemptNo      int32     `gorm:"column:attempt_no"`
		LeaseExpiresAt time.Time `gorm:"column:lease_expires_at"`
	}
	var rows []expiredLeaseRow
	err := m.db.WithContext(ctx).Table("submission_attempts AS a").
		Select("a.submission_id, a.id AS attempt_id, a.attempt_no, a.lease_expires_at").
		Joins("JOIN submissions AS s ON s.id = a.submission_id AND s.current_attempt_id = a.id").
		Where("a.status = ? AND a.lease_expires_at <= ?", AttemptStatusJudging, before).
		Order("a.lease_expires_at ASC, a.id ASC").Limit(limit).Scan(&rows).Error
	if err != nil {
		return nil, normalizeDatabaseError(err)
	}
	result := make([]ExpiredLease, len(rows))
	for index := range rows {
		result[index] = ExpiredLease{SubmissionID: rows[index].SubmissionID, AttemptID: rows[index].AttemptID,
			AttemptNo: rows[index].AttemptNo, LeaseExpiresAt: rows[index].LeaseExpiresAt.UTC()}
	}
	return result, nil
}

func (m *defaultSubmitModel) SupersedeExpiredAttempt(ctx context.Context, input SupersedeAttemptInput) (*Attempt, error) {
	if input.SubmissionID <= 0 || input.AttemptID <= 0 {
		return nil, fmt.Errorf("%w: submission and attempt IDs must be positive", ErrInvalidSubmissionInput)
	}
	reason, err := normalizeRejudgeReason(input.Reason)
	if err != nil {
		return nil, err
	}
	if reason == "" {
		reason = "judge lease expired"
	}
	var result *Attempt
	err = m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		submission, findErr := findSubmissionRow(tx, input.SubmissionID, true)
		if findErr != nil {
			return findErr
		}
		if submission.CurrentAttemptID == nil || *submission.CurrentAttemptID != input.AttemptID {
			return ErrStaleAttempt
		}
		attempt, findErr := findAttemptRow(tx, input.AttemptID, true)
		if findErr != nil {
			return findErr
		}
		if attempt.SubmissionID != submission.ID {
			return ErrStaleAttempt
		}
		if attempt.Status == AttemptStatusFinished {
			return ErrAttemptAlreadyFinished
		}
		if attempt.Status != AttemptStatusJudging {
			return ErrSubmissionStateConflict
		}
		at := m.now().UTC()
		if attempt.LeaseExpiresAt == nil || at.Before(*attempt.LeaseExpiresAt) {
			return ErrLeaseHeld
		}
		created, createErr := supersedeAndCreateAttemptTx(tx, submission, attempt, AttemptKindLeaseRetry, nil, reason, at)
		if createErr != nil {
			return createErr
		}
		result = attemptFromRow(created)
		return nil
	})
	if err != nil {
		return nil, normalizeDatabaseError(err)
	}
	return result, nil
}
