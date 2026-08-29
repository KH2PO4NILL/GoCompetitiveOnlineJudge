package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (m *defaultContestModel) Register(ctx context.Context, input RegisterContestInput) (*ContestRegistration, *ContestParticipant, error) {
	if input.ContestID <= 0 || input.UserID <= 0 {
		return nil, nil, fmt.Errorf("%w: contest and user ids must be positive", ErrInvalidContestInput)
	}
	note, err := normalizeNote(input.ApplicationNote, 2000, "application note")
	if err != nil {
		return nil, nil, err
	}
	var registration *ContestRegistration
	var participant *ContestParticipant
	err = m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		contest, err := lockContest(tx, input.ContestID, false)
		if err != nil {
			return err
		}
		now := m.at(input.At)
		if !registrationOpen(contest, now) {
			return ErrRegistrationClosed
		}
		if contest.RegistrationPolicy == RegistrationPolicyInviteOnly &&
			!inviteCodeMatches(contest.InviteCodeHash, input.InviteCode) {
			return ErrInvalidInviteCode
		}

		status := RegistrationStatusPending
		if contest.RegistrationPolicy == RegistrationPolicyOpen || contest.RegistrationPolicy == RegistrationPolicyInviteOnly {
			status = RegistrationStatusApproved
		}
		row := new(contestRegistrationRow)
		findErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("contest_id = ? AND user_id = ?", input.ContestID, input.UserID).Take(row).Error
		switch {
		case errors.Is(findErr, gorm.ErrRecordNotFound):
			row = &contestRegistrationRow{
				ContestID: input.ContestID, UserID: input.UserID, Status: status,
				ApplicationNote: note, Revision: 1, CreatedAt: now, UpdatedAt: now,
			}
			if status == RegistrationStatusApproved {
				row.ReviewedAt = utcTimePointer(&now)
			}
			if err := tx.Create(row).Error; err != nil {
				if isDuplicateKeyError(err) {
					return ErrRegistrationAlreadyExists
				}
				return fmt.Errorf("insert contest registration: %w", err)
			}
		case findErr != nil:
			return fmt.Errorf("lock existing contest registration: %w", findErr)
		default:
			if row.Status == RegistrationStatusPending || row.Status == RegistrationStatusApproved {
				return ErrRegistrationAlreadyExists
			}
			expected := row.Revision
			row.Status = status
			row.ApplicationNote = note
			row.ReviewerID = nil
			row.ReviewNote = ""
			row.ReviewedAt = nil
			row.CancelledAt = nil
			row.Revision++
			row.UpdatedAt = now
			if status == RegistrationStatusApproved {
				row.ReviewedAt = utcTimePointer(&now)
			}
			if err := conditionalRegistrationUpdate(tx, row, expected); err != nil {
				return err
			}
		}

		if row.Status == RegistrationStatusApproved {
			participantRow, err := ensureNormalParticipant(tx, contest.ID, row.ID, row.UserID, now)
			if err != nil {
				return err
			}
			participant = participantFromRow(participantRow)
		}
		registration = registrationFromRow(row)
		return nil
	})
	return registration, participant, err
}

func (m *defaultContestModel) CancelRegistration(ctx context.Context, contestID, userID, expectedRevision int64) (*ContestRegistration, error) {
	if contestID <= 0 || userID <= 0 || expectedRevision <= 0 {
		return nil, fmt.Errorf("%w: contest, user and revision values must be positive", ErrInvalidContestInput)
	}
	var result *ContestRegistration
	err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		contest, err := lockContest(tx, contestID, false)
		if err != nil {
			return err
		}
		now := m.now().UTC()
		if EffectiveContestStatus(contest.Status, contest.StartAt, contest.EndAt, now) != ContestStatusPublished {
			return ErrRegistrationClosed
		}
		row, err := lockRegistrationByUser(tx, contestID, userID)
		if err != nil {
			return err
		}
		if row.Revision != expectedRevision {
			return ErrRegistrationRevisionConflict
		}
		if row.Status != RegistrationStatusPending {
			return ErrRegistrationStateConflict
		}
		row.Status = RegistrationStatusCancelled
		row.CancelledAt = utcTimePointer(&now)
		row.Revision++
		row.UpdatedAt = now
		if err := conditionalRegistrationUpdate(tx, row, expectedRevision); err != nil {
			return err
		}
		result = registrationFromRow(row)
		return nil
	})
	return result, err
}

func (m *defaultContestModel) ReviewRegistration(ctx context.Context, input ReviewRegistrationInput) (*ContestRegistration, *ContestParticipant, error) {
	if input.ContestID <= 0 || input.RegistrationID <= 0 || input.ReviewerID <= 0 || input.ExpectedRevision <= 0 {
		return nil, nil, fmt.Errorf("%w: contest, registration, reviewer and revision values must be positive", ErrInvalidContestInput)
	}
	if input.Decision != RegistrationDecisionApprove && input.Decision != RegistrationDecisionReject {
		return nil, nil, fmt.Errorf("%w: invalid registration review decision", ErrInvalidContestInput)
	}
	note, err := normalizeNote(input.ReviewNote, 2000, "review note")
	if err != nil {
		return nil, nil, err
	}
	var registration *ContestRegistration
	var participant *ContestParticipant
	err = m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		contest, err := lockContest(tx, input.ContestID, false)
		if err != nil {
			return err
		}
		now := m.now().UTC()
		if contest.RegistrationPolicy != RegistrationPolicyApprovalRequired ||
			EffectiveContestStatus(contest.Status, contest.StartAt, contest.EndAt, now) != ContestStatusPublished {
			return ErrRegistrationStateConflict
		}
		row, err := lockRegistrationByID(tx, input.ContestID, input.RegistrationID)
		if err != nil {
			return err
		}
		if row.Revision != input.ExpectedRevision {
			return ErrRegistrationRevisionConflict
		}
		if row.Status != RegistrationStatusPending {
			return ErrRegistrationStateConflict
		}
		row.Status = RegistrationStatusRejected
		if input.Decision == RegistrationDecisionApprove {
			row.Status = RegistrationStatusApproved
		}
		row.ReviewerID = int64Pointer(input.ReviewerID)
		row.ReviewNote = note
		row.ReviewedAt = utcTimePointer(&now)
		row.CancelledAt = nil
		row.Revision++
		row.UpdatedAt = now
		if err := conditionalRegistrationUpdate(tx, row, input.ExpectedRevision); err != nil {
			return err
		}
		if row.Status == RegistrationStatusApproved {
			participantRow, err := ensureNormalParticipant(tx, contest.ID, row.ID, row.UserID, now)
			if err != nil {
				return err
			}
			participant = participantFromRow(participantRow)
		}
		registration = registrationFromRow(row)
		return nil
	})
	return registration, participant, err
}

func (m *defaultContestModel) SetParticipantStatus(ctx context.Context, input SetParticipantStatusInput) (*ContestParticipant, error) {
	if input.ContestID <= 0 || input.UserID <= 0 ||
		input.Status < ParticipantStatusNormal || input.Status > ParticipantStatusWithdrawn {
		return nil, fmt.Errorf("%w: contest, user and participant status are required", ErrInvalidContestInput)
	}
	var participant *ContestParticipant
	err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := lockContest(tx, input.ContestID, false); err != nil {
			return err
		}
		row := new(contestParticipantRow)
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("contest_id = ? AND user_id = ?", input.ContestID, input.UserID).Take(row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrParticipantNotFound
		}
		if err != nil {
			return fmt.Errorf("lock contest participant: %w", err)
		}
		if !allowedParticipantTransition(row.Status, input.Status) {
			return ErrParticipantStateConflict
		}
		delta := int64(0)
		if row.Status == ParticipantStatusNormal {
			delta = -1
		} else if input.Status == ParticipantStatusNormal {
			delta = 1
		}
		now := m.now().UTC()
		result := tx.Model(&contestParticipantRow{}).
			Where("contest_id = ? AND user_id = ? AND status = ?", row.ContestID, row.UserID, row.Status).
			Updates(map[string]any{"status": input.Status, "updated_at": now})
		if result.Error != nil {
			return fmt.Errorf("update contest participant status: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return ErrParticipantStateConflict
		}
		if delta != 0 {
			if err := adjustParticipantCount(tx, row.ContestID, delta); err != nil {
				return err
			}
		}
		row.Status = input.Status
		row.UpdatedAt = now
		participant = participantFromRow(row)
		return nil
	})
	return participant, err
}

func (m *defaultContestModel) GetMyStatus(ctx context.Context, contestID, userID int64) (*MyContestStatus, error) {
	if contestID <= 0 || userID <= 0 {
		return nil, fmt.Errorf("%w: contest and user ids must be positive", ErrInvalidContestInput)
	}
	var result *MyContestStatus
	err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		result, err = getMyContestStatus(tx, contestID, userID)
		return err
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return result, err
}

func getMyContestStatus(db *gorm.DB, contestID, userID int64) (*MyContestStatus, error) {
	if _, err := findContest(db, contestID, false); err != nil {
		return nil, err
	}
	result := &MyContestStatus{ContestID: contestID}
	var registration contestRegistrationRow
	err := db.Where("contest_id = ? AND user_id = ?", contestID, userID).Take(&registration).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("get contest registration status: %w", err)
	}
	if err == nil {
		result.Registration = registrationFromRow(&registration)
	}
	var participant contestParticipantRow
	err = db.Where("contest_id = ? AND user_id = ?", contestID, userID).Take(&participant).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("get contest participant status: %w", err)
	}
	if err == nil {
		result.Participant = participantFromRow(&participant)
	}
	return result, nil
}

func (m *defaultContestModel) ListRegistrations(ctx context.Context, filter RegistrationListFilter) ([]ContestRegistration, int64, error) {
	if filter.ContestID <= 0 {
		return nil, 0, fmt.Errorf("%w: contest id must be positive", ErrInvalidContestInput)
	}
	if filter.Status != nil && (*filter.Status < RegistrationStatusPending || *filter.Status > RegistrationStatusCancelled) {
		return nil, 0, fmt.Errorf("%w: invalid registration status filter", ErrInvalidContestInput)
	}
	var rows []ContestRegistration
	var total int64
	err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		rows, total, err = listRegistrationRows(tx, filter)
		return err
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return rows, total, err
}

func listRegistrationRows(db *gorm.DB, filter RegistrationListFilter) ([]ContestRegistration, int64, error) {
	if _, err := findContest(db, filter.ContestID, false); err != nil {
		return nil, 0, err
	}
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	query := db.Model(&contestRegistrationRow{}).Where("contest_id = ?", filter.ContestID)
	if filter.Status != nil {
		query = query.Where("status = ?", *filter.Status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count contest registrations: %w", err)
	}
	if total == 0 {
		return []ContestRegistration{}, 0, nil
	}
	var rows []contestRegistrationRow
	if err := query.Order("created_at ASC, id ASC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("list contest registrations: %w", err)
	}
	result := make([]ContestRegistration, 0, len(rows))
	for i := range rows {
		result = append(result, *registrationFromRow(&rows[i]))
	}
	return result, total, nil
}

func (m *defaultContestModel) ListParticipants(ctx context.Context, filter ParticipantListFilter) ([]ContestParticipant, int64, error) {
	if filter.ContestID <= 0 {
		return nil, 0, fmt.Errorf("%w: contest id must be positive", ErrInvalidContestInput)
	}
	if filter.Status != nil && (*filter.Status < ParticipantStatusNormal || *filter.Status > ParticipantStatusWithdrawn) {
		return nil, 0, fmt.Errorf("%w: invalid participant status filter", ErrInvalidContestInput)
	}
	var rows []ContestParticipant
	var total int64
	err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		rows, total, err = listParticipantRows(tx, filter)
		return err
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return rows, total, err
}

func listParticipantRows(db *gorm.DB, filter ParticipantListFilter) ([]ContestParticipant, int64, error) {
	if _, err := findContest(db, filter.ContestID, false); err != nil {
		return nil, 0, err
	}
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	query := db.Model(&contestParticipantRow{}).Where("contest_id = ?", filter.ContestID)
	if filter.Status != nil {
		query = query.Where("status = ?", *filter.Status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count contest participants: %w", err)
	}
	if total == 0 {
		return []ContestParticipant{}, 0, nil
	}
	var rows []contestParticipantRow
	if err := query.Order("qualified_at ASC, user_id ASC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("list contest participants: %w", err)
	}
	result := make([]ContestParticipant, 0, len(rows))
	for i := range rows {
		result = append(result, *participantFromRow(&rows[i]))
	}
	return result, total, nil
}

func (m *defaultContestModel) GetAccessSnapshot(ctx context.Context, contestID, problemID, userID int64, at time.Time) (*ContestAccessSnapshot, error) {
	if contestID <= 0 || problemID <= 0 || userID <= 0 {
		return nil, fmt.Errorf("%w: contest, problem and user ids must be positive", ErrInvalidContestInput)
	}
	type accessRow struct {
		ContestID          int64     `gorm:"column:contest_id"`
		Mode               int64     `gorm:"column:mode"`
		Status             int64     `gorm:"column:status"`
		Visibility         int64     `gorm:"column:visibility"`
		StartAt            time.Time `gorm:"column:start_at"`
		EndAt              time.Time `gorm:"column:end_at"`
		AssignedProblemID  *int64    `gorm:"column:assigned_problem_id"`
		RegistrationStatus *int64    `gorm:"column:registration_status"`
		ParticipantStatus  *int64    `gorm:"column:participant_status"`
	}
	var row accessRow
	err := m.db.WithContext(ctx).Table("contests AS c").
		Select(`c.id AS contest_id, c.mode, c.status, c.visibility, c.start_at, c.end_at,
			cp.problem_id AS assigned_problem_id, cr.status AS registration_status,
			pt.status AS participant_status`).
		Joins("LEFT JOIN contest_problems AS cp ON cp.contest_id = c.id AND cp.problem_id = ?", problemID).
		Joins("LEFT JOIN contest_registrations AS cr ON cr.contest_id = c.id AND cr.user_id = ?", userID).
		Joins("LEFT JOIN contest_participants AS pt ON pt.contest_id = c.id AND pt.user_id = ?", userID).
		Where("c.id = ? AND c.deleted_at IS NULL", contestID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrContestNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get contest access snapshot: %w", err)
	}
	at = m.at(at)
	return &ContestAccessSnapshot{
		ContestID: row.ContestID, Mode: row.Mode, LifecycleStatus: row.Status,
		EffectiveStatus: EffectiveContestStatus(row.Status, row.StartAt, row.EndAt, at),
		Visibility:      row.Visibility, StartAt: row.StartAt.UTC(), EndAt: row.EndAt.UTC(),
		ProblemAssigned:    row.AssignedProblemID != nil,
		RegistrationStatus: cloneInt64Pointer(row.RegistrationStatus),
		ParticipantStatus:  cloneInt64Pointer(row.ParticipantStatus),
	}, nil
}

func (m *defaultContestModel) VerifyInviteCode(ctx context.Context, contestID int64, candidate string) error {
	if contestID <= 0 {
		return fmt.Errorf("%w: contest id must be positive", ErrInvalidContestInput)
	}
	var row struct {
		InviteCodeHash *string `gorm:"column:invite_code_hash"`
	}
	err := m.db.WithContext(ctx).Model(&contestRow{}).Select("invite_code_hash").
		Where("id = ? AND deleted_at IS NULL", contestID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrContestNotFound
	}
	if err != nil {
		return fmt.Errorf("load contest invite code: %w", err)
	}
	if !inviteCodeMatches(row.InviteCodeHash, candidate) {
		return ErrInvalidInviteCode
	}
	return nil
}

func ensureNormalParticipant(tx *gorm.DB, contestID, registrationID, userID int64, now time.Time) (*contestParticipantRow, error) {
	row := new(contestParticipantRow)
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("contest_id = ? AND user_id = ?", contestID, userID).Take(row).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		row = &contestParticipantRow{ContestID: contestID, UserID: userID, RegistrationID: registrationID,
			Status: ParticipantStatusNormal, QualifiedAt: now, UpdatedAt: now}
		if err := tx.Create(row).Error; err != nil {
			if isDuplicateKeyError(err) {
				return nil, ErrParticipantStateConflict
			}
			return nil, fmt.Errorf("insert contest participant: %w", err)
		}
		if err := incrementParticipantCount(tx, contestID); err != nil {
			return nil, err
		}
	case err != nil:
		return nil, fmt.Errorf("lock contest participant: %w", err)
	default:
		if row.Status != ParticipantStatusNormal {
			return nil, ErrParticipantStateConflict
		}
		if row.RegistrationID != registrationID {
			return nil, ErrParticipantStateConflict
		}
	}
	return row, nil
}

func incrementParticipantCount(tx *gorm.DB, contestID int64) error {
	return adjustParticipantCount(tx, contestID, 1)
}

func adjustParticipantCount(tx *gorm.DB, contestID, delta int64) error {
	query := tx.Model(&contestRow{}).Where("id = ? AND deleted_at IS NULL", contestID)
	if delta < 0 {
		query = query.Where("participant_count >= ?", -delta)
	}
	result := query.UpdateColumn("participant_count", gorm.Expr("participant_count + ?", delta))
	if result.Error != nil {
		return fmt.Errorf("adjust contest participant count: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrParticipantStateConflict
	}
	return nil
}

func allowedParticipantTransition(from, to int64) bool {
	if from == to {
		return false
	}
	if from == ParticipantStatusNormal {
		return to == ParticipantStatusDisqualified || to == ParticipantStatusWithdrawn
	}
	return to == ParticipantStatusNormal
}

func lockRegistrationByUser(tx *gorm.DB, contestID, userID int64) (*contestRegistrationRow, error) {
	row := new(contestRegistrationRow)
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("contest_id = ? AND user_id = ?", contestID, userID).Take(row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRegistrationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock contest registration: %w", err)
	}
	return row, nil
}

func lockRegistrationByID(tx *gorm.DB, contestID, registrationID int64) (*contestRegistrationRow, error) {
	row := new(contestRegistrationRow)
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("contest_id = ? AND id = ?", contestID, registrationID).Take(row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRegistrationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock contest registration: %w", err)
	}
	return row, nil
}

func conditionalRegistrationUpdate(tx *gorm.DB, row *contestRegistrationRow, expectedRevision int64) error {
	updates := map[string]any{
		"status": row.Status, "application_note": row.ApplicationNote,
		"reviewer_id": row.ReviewerID, "review_note": row.ReviewNote,
		"revision": row.Revision, "updated_at": row.UpdatedAt,
		"reviewed_at": row.ReviewedAt, "cancelled_at": row.CancelledAt,
	}
	result := tx.Model(&contestRegistrationRow{}).
		Where("id = ? AND contest_id = ? AND revision = ?", row.ID, row.ContestID, expectedRevision).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update contest registration: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrRegistrationRevisionConflict
	}
	return nil
}
