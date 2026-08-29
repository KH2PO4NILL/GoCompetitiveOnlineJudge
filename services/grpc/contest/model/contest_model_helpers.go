package model

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

const maxProtoInt32 int64 = 1<<31 - 1

func EffectiveContestStatus(persisted int64, startAt, endAt, at time.Time) int64 {
	if persisted != ContestStatusPublished {
		return persisted
	}
	at = normalizedAt(at)
	if at.Before(startAt) {
		return ContestStatusPublished
	}
	if at.Before(endAt) {
		return ContestStatusRunning
	}
	return ContestStatusFinished
}

func normalizedAt(at time.Time) time.Time {
	if at.IsZero() {
		return time.Now().UTC()
	}
	return at.UTC()
}

func (m *defaultContestModel) at(value time.Time) time.Time {
	if value.IsZero() {
		return m.now().UTC()
	}
	return value.UTC()
}

func validateEditableContest(editable *EditableContest, inviteConfigured bool) error {
	editable.Title = strings.TrimSpace(editable.Title)
	if editable.Title == "" || utf8.RuneCountInString(editable.Title) > 200 {
		return fmt.Errorf("%w: title is required and must be at most 200 characters", ErrInvalidContestInput)
	}
	if utf8.RuneCountInString(editable.Description) > 200_000 {
		return fmt.Errorf("%w: description is too long", ErrInvalidContestInput)
	}
	if !validContestMode(editable.Mode) {
		return fmt.Errorf("%w: invalid contest mode", ErrInvalidContestInput)
	}
	if editable.StartAt.IsZero() || editable.EndAt.IsZero() {
		return fmt.Errorf("%w: start and end times are required", ErrInvalidContestInput)
	}
	editable.StartAt = editable.StartAt.UTC()
	editable.EndAt = editable.EndAt.UTC()
	if !editable.StartAt.Before(editable.EndAt) {
		return fmt.Errorf("%w: start time must be before end time", ErrInvalidContestInput)
	}
	editable.RegistrationStartAt = utcTimePointer(editable.RegistrationStartAt)
	editable.RegistrationEndAt = utcTimePointer(editable.RegistrationEndAt)
	if editable.RegistrationStartAt != nil && !editable.RegistrationStartAt.Before(editable.StartAt) {
		return fmt.Errorf("%w: registration start must be before contest start", ErrInvalidContestInput)
	}
	if editable.RegistrationEndAt != nil && editable.RegistrationEndAt.After(editable.StartAt) {
		return fmt.Errorf("%w: registration end cannot be after contest start", ErrInvalidContestInput)
	}
	if editable.RegistrationStartAt != nil && editable.RegistrationEndAt != nil &&
		!editable.RegistrationStartAt.Before(*editable.RegistrationEndAt) {
		return fmt.Errorf("%w: registration start must be before registration end", ErrInvalidContestInput)
	}
	if editable.RegistrationPolicy < RegistrationPolicyOpen || editable.RegistrationPolicy > RegistrationPolicyInviteOnly {
		return fmt.Errorf("%w: invalid registration policy", ErrInvalidContestInput)
	}
	if editable.Visibility < ContestVisibilityPublic || editable.Visibility > ContestVisibilityPrivate {
		return fmt.Errorf("%w: invalid contest visibility", ErrInvalidContestInput)
	}
	if editable.ScoreboardVisibility < ScoreboardVisibilityPublic || editable.ScoreboardVisibility > ScoreboardVisibilityHidden {
		return fmt.Errorf("%w: invalid scoreboard visibility", ErrInvalidContestInput)
	}

	editable.ScoreboardFreezeAt = utcTimePointer(editable.ScoreboardFreezeAt)
	if editable.ScoreboardFreezeAt != nil && editable.ScoreboardFreezeBeforeEnd != nil {
		return fmt.Errorf("%w: only one scoreboard freeze policy may be set", ErrInvalidContestInput)
	}
	if editable.Mode != ContestModeICPC &&
		(editable.ScoreboardFreezeAt != nil || editable.ScoreboardFreezeBeforeEnd != nil) {
		return fmt.Errorf("%w: only ICPC contests may configure scoreboard freeze", ErrInvalidContestInput)
	}
	if editable.ScoreboardFreezeAt != nil &&
		(!editable.ScoreboardFreezeAt.After(editable.StartAt) || !editable.ScoreboardFreezeAt.Before(editable.EndAt)) {
		return fmt.Errorf("%w: scoreboard freeze time must be inside the contest window", ErrInvalidContestInput)
	}
	if editable.ScoreboardFreezeBeforeEnd != nil {
		duration := *editable.ScoreboardFreezeBeforeEnd
		if duration <= 0 || duration >= editable.EndAt.Sub(editable.StartAt) || duration%time.Millisecond != 0 {
			return fmt.Errorf("%w: invalid scoreboard freeze duration", ErrInvalidContestInput)
		}
		editable.ScoreboardFreezeBeforeEnd = durationPointer(duration)
	}

	if editable.InviteCode != nil {
		value, err := normalizeInviteCode(*editable.InviteCode)
		if err != nil {
			return err
		}
		editable.InviteCode = &value
		inviteConfigured = true
	}
	if editable.RegistrationPolicy == RegistrationPolicyInviteOnly && !inviteConfigured {
		return fmt.Errorf("%w: invite-only contests require an invite code", ErrInvalidContestInput)
	}
	return nil
}

func validContestMode(mode int64) bool {
	return mode >= ContestModeICPC && mode <= ContestModeOI
}

func normalizeInviteCode(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 72 || utf8.RuneCountInString(value) > 64 {
		return "", fmt.Errorf("%w: invite code must be 1-64 characters and at most 72 bytes", ErrInvalidContestInput)
	}
	return value, nil
}

func hashInviteCode(value string) (*string, error) {
	value, err := normalizeInviteCode(value)
	if err != nil {
		return nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(value), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash contest invite code: %w", err)
	}
	encoded := string(hash)
	return &encoded, nil
}

func inviteCodeMatches(hash *string, candidate string) bool {
	if hash == nil {
		return false
	}
	candidate, err := normalizeInviteCode(candidate)
	if err != nil {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(*hash), []byte(candidate)) == nil
}

func validateContestPatch(patch ContestPatch) error {
	if patch.Title == nil && patch.Description == nil && patch.Mode == nil && patch.StartAt == nil &&
		patch.EndAt == nil && patch.RegistrationStartAt == nil && patch.RegistrationEndAt == nil &&
		patch.RegistrationPolicy == nil && patch.Visibility == nil && patch.ScoreboardVisibility == nil &&
		patch.ScoreboardFreezeAt == nil && patch.ScoreboardFreezeBeforeEnd == nil && patch.InviteCode == nil {
		return fmt.Errorf("%w: contest patch is empty", ErrInvalidContestInput)
	}
	if patch.ScoreboardFreezeAt != nil && *patch.ScoreboardFreezeAt != nil &&
		patch.ScoreboardFreezeBeforeEnd != nil && *patch.ScoreboardFreezeBeforeEnd != nil {
		return fmt.Errorf("%w: conflicting scoreboard freeze policies", ErrInvalidContestInput)
	}
	return nil
}

func applyContestPatch(editable *EditableContest, patch ContestPatch) {
	if patch.Title != nil {
		editable.Title = *patch.Title
	}
	if patch.Description != nil {
		editable.Description = *patch.Description
	}
	if patch.Mode != nil {
		editable.Mode = *patch.Mode
	}
	if patch.StartAt != nil {
		editable.StartAt = *patch.StartAt
	}
	if patch.EndAt != nil {
		editable.EndAt = *patch.EndAt
	}
	if patch.RegistrationStartAt != nil {
		editable.RegistrationStartAt = utcTimePointer(*patch.RegistrationStartAt)
	}
	if patch.RegistrationEndAt != nil {
		editable.RegistrationEndAt = utcTimePointer(*patch.RegistrationEndAt)
	}
	if patch.RegistrationPolicy != nil {
		editable.RegistrationPolicy = *patch.RegistrationPolicy
	}
	if patch.Visibility != nil {
		editable.Visibility = *patch.Visibility
	}
	if patch.ScoreboardVisibility != nil {
		editable.ScoreboardVisibility = *patch.ScoreboardVisibility
	}
	if patch.ScoreboardFreezeAt != nil {
		editable.ScoreboardFreezeAt = utcTimePointer(*patch.ScoreboardFreezeAt)
		if *patch.ScoreboardFreezeAt != nil {
			editable.ScoreboardFreezeBeforeEnd = nil
		}
	}
	if patch.ScoreboardFreezeBeforeEnd != nil {
		editable.ScoreboardFreezeBeforeEnd = cloneDurationPointer(*patch.ScoreboardFreezeBeforeEnd)
		if *patch.ScoreboardFreezeBeforeEnd != nil {
			editable.ScoreboardFreezeAt = nil
		}
	}
	if patch.InviteCode != nil {
		editable.InviteCode = cloneStringPointer(*patch.InviteCode)
	}
}

func editableContestFromRow(row *contestRow) EditableContest {
	var freezeBefore *time.Duration
	if row.ScoreboardFreezeBeforeEndMS != nil {
		value := time.Duration(*row.ScoreboardFreezeBeforeEndMS) * time.Millisecond
		freezeBefore = &value
	}
	return EditableContest{
		Title: row.Title, Description: row.Description, Mode: row.Mode,
		StartAt: row.StartAt.UTC(), EndAt: row.EndAt.UTC(),
		RegistrationStartAt: utcTimePointer(row.RegistrationStartAt),
		RegistrationEndAt:   utcTimePointer(row.RegistrationEndAt),
		RegistrationPolicy:  row.RegistrationPolicy, Visibility: row.Visibility,
		ScoreboardVisibility:      row.ScoreboardVisibility,
		ScoreboardFreezeAt:        utcTimePointer(row.ScoreboardFreezeAt),
		ScoreboardFreezeBeforeEnd: freezeBefore,
	}
}

func aggregateFromContestRow(row *contestRow, at time.Time) *ContestAggregate {
	return &ContestAggregate{
		ContestID: row.ID, CreatorID: row.CreatorID, Editable: editableContestFromRow(row),
		InviteCodeConfigured: row.InviteCodeHash != nil,
		Status:               row.Status, EffectiveStatus: EffectiveContestStatus(row.Status, row.StartAt, row.EndAt, at),
		Revision: row.Revision, ParticipantCount: row.ParticipantCount, CancelReason: row.CancelReason,
		PublishedAt: utcTimePointer(row.PublishedAt), CancelledAt: utcTimePointer(row.CancelledAt),
		ArchivedAt: utcTimePointer(row.ArchivedAt), DeletedAt: utcTimePointer(row.DeletedAt),
		ScoreboardFrozen: row.ScoreboardFrozen, ScoreboardFrozenAt: utcTimePointer(row.ScoreboardFrozenAt),
		CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC(),
	}
}

func detailFromContestRow(row *contestRow, at time.Time) *ContestDetailRecord {
	return &ContestDetailRecord{
		ContestID: row.ID, Title: row.Title, Description: row.Description, Mode: row.Mode,
		Status:  EffectiveContestStatus(row.Status, row.StartAt, row.EndAt, at),
		StartAt: row.StartAt.UTC(), EndAt: row.EndAt.UTC(),
		RegistrationStartAt: utcTimePointer(row.RegistrationStartAt),
		RegistrationEndAt:   utcTimePointer(row.RegistrationEndAt),
		RegistrationPolicy:  row.RegistrationPolicy, Visibility: row.Visibility,
		ScoreboardVisibility: row.ScoreboardVisibility,
		RegistrationOpen:     registrationOpen(row, at), ScoreboardFrozen: row.ScoreboardFrozen,
		ParticipantCount: row.ParticipantCount,
	}
}

func registrationOpen(row *contestRow, at time.Time) bool {
	at = normalizedAt(at)
	if row.Status != ContestStatusPublished || !at.Before(row.StartAt) {
		return false
	}
	if row.RegistrationStartAt != nil && at.Before(*row.RegistrationStartAt) {
		return false
	}
	deadline := row.StartAt
	if row.RegistrationEndAt != nil {
		deadline = *row.RegistrationEndAt
	}
	return at.Before(deadline)
}

func contestRowUpdates(row *contestRow) map[string]any {
	var freezeBefore any
	if row.ScoreboardFreezeBeforeEndMS != nil {
		freezeBefore = *row.ScoreboardFreezeBeforeEndMS
	}
	return map[string]any{
		"title": row.Title, "description": row.Description, "mode": row.Mode,
		"start_at": row.StartAt, "end_at": row.EndAt,
		"registration_start_at": row.RegistrationStartAt, "registration_end_at": row.RegistrationEndAt,
		"registration_policy": row.RegistrationPolicy, "visibility": row.Visibility,
		"scoreboard_visibility":           row.ScoreboardVisibility,
		"scoreboard_freeze_at":            row.ScoreboardFreezeAt,
		"scoreboard_freeze_before_end_ms": freezeBefore,
		"invite_code_hash":                row.InviteCodeHash, "status": row.Status, "revision": row.Revision,
		"participant_count": row.ParticipantCount, "cancel_reason": row.CancelReason,
		"published_at": row.PublishedAt, "cancelled_at": row.CancelledAt,
		"archived_at": row.ArchivedAt, "deleted_at": row.DeletedAt,
		"deleted_from_status": row.DeletedFromStatus,
		"scoreboard_frozen":   row.ScoreboardFrozen, "scoreboard_frozen_at": row.ScoreboardFrozenAt,
		"updated_at": row.UpdatedAt,
	}
}

func registrationFromRow(row *contestRegistrationRow) *ContestRegistration {
	if row == nil {
		return nil
	}
	return &ContestRegistration{
		RegistrationID: row.ID, ContestID: row.ContestID, UserID: row.UserID, Status: row.Status,
		ApplicationNote: row.ApplicationNote, ReviewerID: cloneInt64Pointer(row.ReviewerID),
		ReviewNote: row.ReviewNote, Revision: row.Revision,
		CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC(),
		ReviewedAt: utcTimePointer(row.ReviewedAt), CancelledAt: utcTimePointer(row.CancelledAt),
	}
}

func participantFromRow(row *contestParticipantRow) *ContestParticipant {
	if row == nil {
		return nil
	}
	return &ContestParticipant{
		ContestID: row.ContestID, UserID: row.UserID, RegistrationID: row.RegistrationID,
		Status: row.Status, QualifiedAt: row.QualifiedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC(),
	}
}

func validateContestProblems(problems []ContestProblem, mode int64) ([]ContestProblem, error) {
	result := make([]ContestProblem, len(problems))
	problemIDs := make(map[int64]struct{}, len(problems))
	labels := make(map[string]struct{}, len(problems))
	orders := make(map[int64]struct{}, len(problems))
	for i, problem := range problems {
		problem.Label = strings.TrimSpace(problem.Label)
		if problem.ProblemID <= 0 || problem.Label == "" || utf8.RuneCountInString(problem.Label) > 32 ||
			problem.DisplayOrder < 0 || problem.DisplayOrder > maxProtoInt32 {
			return nil, fmt.Errorf("%w: invalid contest problem at index %d", ErrInvalidContestInput, i)
		}
		if mode == ContestModeICPC && problem.MaxScore != 0 {
			return nil, fmt.Errorf("%w: ICPC problems must have max score 0", ErrInvalidContestInput)
		}
		if (mode == ContestModeIOI || mode == ContestModeOI) && problem.MaxScore <= 0 {
			return nil, fmt.Errorf("%w: IOI/OI problems require a positive max score", ErrInvalidContestInput)
		}
		if _, exists := problemIDs[problem.ProblemID]; exists {
			return nil, ErrContestProblemConflict
		}
		if _, exists := labels[problem.Label]; exists {
			return nil, ErrContestProblemConflict
		}
		if _, exists := orders[problem.DisplayOrder]; exists {
			return nil, ErrContestProblemConflict
		}
		problemIDs[problem.ProblemID] = struct{}{}
		labels[problem.Label] = struct{}{}
		orders[problem.DisplayOrder] = struct{}{}
		result[i] = problem
	}
	return result, nil
}

func normalizePage(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func normalizeNote(value string, max int, field string) (string, error) {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) > max {
		return "", fmt.Errorf("%w: %s is too long", ErrInvalidContestInput, field)
	}
	return value, nil
}

func isDuplicateKeyError(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}

func utcTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	result := value.UTC()
	return &result
}

func cloneStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	result := *value
	return &result
}

func cloneInt64Pointer(value *int64) *int64 {
	if value == nil {
		return nil
	}
	result := *value
	return &result
}

func durationPointer(value time.Duration) *time.Duration {
	result := value
	return &result
}

func cloneDurationPointer(value *time.Duration) *time.Duration {
	if value == nil {
		return nil
	}
	return durationPointer(*value)
}

func int64Pointer(value int64) *int64 { return &value }

func boolPointer(value bool) *bool { return &value }
