package model

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (m *defaultSubmitModel) CreateSubmission(ctx context.Context, input CreateSubmissionInput) (*CreateSubmissionResult, error) {
	if err := validateCreateInput(&input); err != nil {
		return nil, err
	}
	at := normalizedTime(input.SubmittedAt, m.now())
	fingerprint := createRequestFingerprint(input)
	var result *CreateSubmissionResult
	err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row := &submissionRow{
			UserID: input.UserID, ProblemID: input.ProblemID, ContestID: cloneInt64Pointer(input.ContestID),
			ProblemRevision: input.ProblemRevision, Language: input.Language, SourceCode: input.SourceCode,
			ClientRequestID: input.ClientRequestID, RequestFingerprint: fingerprint,
			Status: SubmissionStatusQueued, AttemptCount: 1, SubmittedAt: at, UpdatedAt: at, Version: 1,
		}
		if err := tx.Create(row).Error; err != nil {
			if !isDuplicateKey(err) {
				return normalizeDatabaseError(err)
			}
			var existing submissionRow
			findErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("user_id = ? AND client_request_id = ?", input.UserID, input.ClientRequestID).Take(&existing).Error
			if findErr != nil {
				return normalizeDatabaseError(findErr)
			}
			if !sameDigest(existing.RequestFingerprint, fingerprint) {
				return fmt.Errorf("%w: client request ID was used for different input", ErrIdempotencyConflict)
			}
			aggregate, loadErr := getSubmissionAggregateTx(tx, &existing)
			if loadErr != nil {
				return loadErr
			}
			result = &CreateSubmissionResult{Submission: aggregate, Deduplicated: true}
			return nil
		}

		attempt := createAttemptRow(row.ID, 1, AttemptKindInitial, nil, "", at)
		if err := tx.Create(attempt).Error; err != nil {
			return normalizeDatabaseError(err)
		}
		if err := tx.Model(&submissionRow{}).Where("id = ?", row.ID).
			Update("current_attempt_id", attempt.ID).Error; err != nil {
			return normalizeDatabaseError(err)
		}
		row.CurrentAttemptID = int64Pointer(attempt.ID)
		if err := createJudgeTaskOutboxTx(tx, row.ID, attempt.ID, at); err != nil {
			return err
		}
		aggregate, err := getSubmissionAggregateTx(tx, row)
		if err != nil {
			return err
		}
		result = &CreateSubmissionResult{Submission: aggregate}
		return nil
	})
	if err != nil {
		return nil, normalizeDatabaseError(err)
	}
	return result, nil
}

func (m *defaultSubmitModel) GetSubmission(ctx context.Context, submissionID int64) (*SubmissionAggregate, error) {
	if submissionID <= 0 {
		return nil, fmt.Errorf("%w: submission ID must be positive", ErrInvalidSubmissionInput)
	}
	var result *SubmissionAggregate
	err := m.withReadTransaction(ctx, func(tx *gorm.DB) error {
		row, err := findSubmissionRow(tx, submissionID, false)
		if err != nil {
			return err
		}
		result, err = getSubmissionAggregateTx(tx, row)
		return err
	})
	return result, err
}

func (m *defaultSubmitModel) GetSubmissionByClientRequestID(
	ctx context.Context,
	userID int64,
	clientRequestID string,
) (*SubmissionAggregate, error) {
	clientRequestID = strings.TrimSpace(clientRequestID)
	if userID <= 0 || clientRequestID == "" || len(clientRequestID) > 128 || !validASCII(clientRequestID) {
		return nil, fmt.Errorf("%w: invalid client request lookup", ErrInvalidSubmissionInput)
	}
	var result *SubmissionAggregate
	err := m.withReadTransaction(ctx, func(tx *gorm.DB) error {
		var row submissionRow
		if err := tx.Where("user_id = ? AND client_request_id = ?", userID, clientRequestID).Take(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrSubmissionNotFound
			}
			return normalizeDatabaseError(err)
		}
		var err error
		result, err = getSubmissionAggregateTx(tx, &row)
		return err
	})
	return result, err
}

func (m *defaultSubmitModel) ListSubmissions(ctx context.Context, filter SubmissionListFilter) ([]SubmissionSummary, int64, error) {
	if err := normalizeListFilter(&filter); err != nil {
		return nil, 0, err
	}
	results := make([]SubmissionSummary, 0)
	var total int64
	err := m.withReadTransaction(ctx, func(tx *gorm.DB) error {
		query := applySubmissionListFilter(tx.Model(&submissionRow{}), filter)
		if err := query.Count(&total).Error; err != nil {
			return normalizeDatabaseError(err)
		}
		var rows []submissionRow
		offset := (filter.Page - 1) * filter.PageSize
		if err := applySubmissionListFilter(tx.Model(&submissionRow{}), filter).
			Order("submitted_at DESC, id DESC").Offset(offset).Limit(filter.PageSize).Find(&rows).Error; err != nil {
			return normalizeDatabaseError(err)
		}
		results = make([]SubmissionSummary, len(rows))
		for index := range rows {
			if rows[index].CurrentAttemptID == nil {
				return fmt.Errorf("%w: submission %d has no current attempt", ErrSubmissionStateConflict, rows[index].ID)
			}
			results[index] = submissionSummaryFromRow(&rows[index])
		}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return results, total, nil
}

func applySubmissionListFilter(query *gorm.DB, filter SubmissionListFilter) *gorm.DB {
	if filter.UserID != nil {
		query = query.Where("user_id = ?", *filter.UserID)
	}
	if filter.ProblemID != nil {
		query = query.Where("problem_id = ?", *filter.ProblemID)
	}
	if filter.ContestID != nil {
		query = query.Where("contest_id = ?", *filter.ContestID)
	}
	if filter.Status != nil {
		query = query.Where("status = ?", *filter.Status)
	}
	if filter.Verdict != nil {
		query = query.Where("verdict = ?", *filter.Verdict)
	}
	if filter.Language != "" {
		query = query.Where("language = ?", filter.Language)
	}
	return query
}

func (m *defaultSubmitModel) submissionForUpdate(tx *gorm.DB, submissionID int64) (*submissionRow, *attemptRow, error) {
	row, err := findSubmissionRow(tx, submissionID, true)
	if err != nil {
		return nil, nil, err
	}
	if row.CurrentAttemptID == nil {
		return nil, nil, fmt.Errorf("%w: current attempt is missing", ErrSubmissionStateConflict)
	}
	attempt, err := findAttemptRow(tx, *row.CurrentAttemptID, true)
	if err != nil {
		if errors.Is(err, ErrAttemptNotFound) {
			return nil, nil, fmt.Errorf("%w: current attempt is missing", ErrSubmissionStateConflict)
		}
		return nil, nil, err
	}
	if attempt.SubmissionID != row.ID {
		return nil, nil, fmt.Errorf("%w: current attempt belongs to another submission", ErrSubmissionStateConflict)
	}
	return row, attempt, nil
}
