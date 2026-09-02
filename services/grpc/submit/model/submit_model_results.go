package model

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

func (m *defaultSubmitModel) ApplyJudgeResult(ctx context.Context, input ApplyJudgeResultInput) (*ApplyJudgeResultResult, error) {
	input.WorkerID = strings.TrimSpace(input.WorkerID)
	input.ReportID = strings.TrimSpace(input.ReportID)
	fingerprint, normalizedCases, err := encodeReportFingerprint(input)
	if err != nil {
		return nil, err
	}
	var result *ApplyJudgeResultResult
	err = m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		submission, findErr := findSubmissionRow(tx, input.SubmissionID, true)
		if findErr != nil {
			return findErr
		}
		attempt, findErr := findAttemptRow(tx, input.AttemptID, true)
		if findErr != nil {
			return findErr
		}
		if attempt.SubmissionID != submission.ID {
			return ErrStaleAttempt
		}

		if attempt.ReportID != nil {
			if *attempt.ReportID == input.ReportID && sameDigest(attempt.ReportFingerprint, fingerprint) {
				aggregate, loadErr := getSubmissionAggregateTx(tx, submission)
				if loadErr != nil {
					return loadErr
				}
				result = &ApplyJudgeResultResult{Disposition: ReportDispositionDuplicate, Submission: aggregate}
				return nil
			}
			if *attempt.ReportID == input.ReportID {
				return fmt.Errorf("%w: report ID was used for different content", ErrIdempotencyConflict)
			}
		}
		if submission.CurrentAttemptID == nil || *submission.CurrentAttemptID != attempt.ID || attempt.Status == AttemptStatusSuperseded {
			return ErrStaleAttempt
		}
		if attempt.ReportID != nil {
			return ErrReportConflict
		}
		if attempt.Status == AttemptStatusFinished || submission.Status == SubmissionStatusFinished {
			return ErrAttemptAlreadyFinished
		}
		if attempt.Status != AttemptStatusJudging || submission.Status != SubmissionStatusJudging {
			return ErrSubmissionStateConflict
		}
		if attempt.WorkerID != input.WorkerID || !sameDigest(attempt.LeaseTokenHash, leaseTokenDigest(input.LeaseToken)) {
			return ErrLeaseMismatch
		}
		judgedAt := m.now().UTC()
		if attempt.LeaseExpiresAt == nil || !judgedAt.Before(*attempt.LeaseExpiresAt) {
			return ErrLeaseExpired
		}

		if len(normalizedCases) > 0 {
			rows := make([]caseResultRow, len(normalizedCases))
			for index := range normalizedCases {
				item := normalizedCases[index]
				rows[index] = caseResultRow{AttemptID: attempt.ID, TestCaseID: item.TestCaseID, Ordinal: item.Ordinal,
					GroupName: item.GroupName, Verdict: item.Verdict, Score: item.Score, TimeUsedMS: item.TimeUsedMS,
					MemoryUsedKB: item.MemoryUsedKB, JudgeMessage: item.JudgeMessage, CreatedAt: judgedAt}
			}
			if err := tx.CreateInBatches(rows, 100).Error; err != nil {
				return normalizeDatabaseError(err)
			}
		}
		if err := tx.Model(&attemptRow{}).Where("id = ?", attempt.ID).Updates(map[string]any{
			"status": AttemptStatusFinished, "report_id": input.ReportID, "report_fingerprint": fingerprint,
			"verdict": input.Verdict, "score": input.Score, "max_score": input.MaxScore,
			"compile_output": input.CompileOutput, "completed_test_cases": len(normalizedCases),
			"finished_at": judgedAt, "updated_at": judgedAt,
		}).Error; err != nil {
			return normalizeDatabaseError(err)
		}
		if err := tx.Model(&submissionRow{}).Where("id = ?", submission.ID).Updates(map[string]any{
			"status": SubmissionStatusFinished, "verdict": input.Verdict, "score": input.Score,
			"max_score": input.MaxScore, "finished_at": judgedAt, "updated_at": judgedAt,
			"version": gorm.Expr("version + 1"),
		}).Error; err != nil {
			return normalizeDatabaseError(err)
		}
		attempt.Status = AttemptStatusFinished
		attempt.ReportID = stringPointer(input.ReportID)
		attempt.ReportFingerprint = append([]byte(nil), fingerprint...)
		attempt.Verdict, attempt.Score, attempt.MaxScore = int64Pointer(input.Verdict), int64Pointer(input.Score), int64Pointer(input.MaxScore)
		attempt.CompletedTestCases = int32(len(normalizedCases))
		attempt.CompileOutput = input.CompileOutput
		attempt.FinishedAt, attempt.UpdatedAt = utcTimePointer(&judgedAt), judgedAt
		submission.Status = SubmissionStatusFinished
		submission.Verdict, submission.Score, submission.MaxScore = int64Pointer(input.Verdict), int64Pointer(input.Score), int64Pointer(input.MaxScore)
		submission.FinishedAt, submission.UpdatedAt = utcTimePointer(&judgedAt), judgedAt
		submission.Version++
		if submission.ContestID != nil {
			if err := createSubmissionJudgedOutboxTx(tx, submission, attempt, judgedAt); err != nil {
				return err
			}
		}
		aggregate, loadErr := getSubmissionAggregateTx(tx, submission)
		if loadErr != nil {
			return loadErr
		}
		result = &ApplyJudgeResultResult{Disposition: ReportDispositionApplied, Submission: aggregate}
		return nil
	})
	if err != nil {
		return nil, normalizeDatabaseError(err)
	}
	return result, nil
}

func createSubmissionJudgedOutboxTx(tx *gorm.DB, submission *submissionRow, attempt *attemptRow, judgedAt time.Time) error {
	if submission.ContestID == nil {
		return nil
	}
	eventID, err := randomUUID()
	if err != nil {
		return err
	}
	payload, err := json.Marshal(submissionJudgedEventPayload{
		EventID: eventID, SubmissionID: submission.ID, AttemptID: attempt.ID, ContestID: *submission.ContestID,
		ProblemID: submission.ProblemID, UserID: submission.UserID, Verdict: *attempt.Verdict,
		Score: *attempt.Score, MaxScore: *attempt.MaxScore, SubmittedAt: submission.SubmittedAt.UTC(), JudgedAt: judgedAt,
	})
	if err != nil {
		return fmt.Errorf("marshal submission judged outbox event: %w", err)
	}
	row := outboxRow{
		EventID: eventID, EventType: OutboxEventSubmissionJudged, AggregateID: submission.ID, AttemptID: attempt.ID,
		EventKey: strconv.FormatInt(submission.ID, 10), Payload: payload, Status: OutboxStatusPending,
		AvailableAt: judgedAt, LastError: "", CreatedAt: judgedAt,
	}
	return normalizeDatabaseError(tx.Create(&row).Error)
}
