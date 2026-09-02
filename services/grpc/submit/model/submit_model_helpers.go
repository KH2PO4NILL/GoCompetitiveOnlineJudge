package model

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	drivermysql "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	maxListPageSize       = 100
	maxBatchRejudgeSize   = 100
	maxExpiredLeaseBatch  = 1000
	maxOutboxClaimBatch   = 500
	maxSourceCodeBytes    = 4 * 1024 * 1024
	maxCompileOutputBytes = 4 * 1024 * 1024
	maxJudgeMessageBytes  = 1024 * 1024
)

func normalizedTime(value time.Time, fallback time.Time) time.Time {
	if value.IsZero() {
		value = fallback
	}
	return value.UTC()
}

func utcTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	result := value.UTC()
	return &result
}

func int64Pointer(value int64) *int64    { return &value }
func stringPointer(value string) *string { return &value }

func validVerdict(value int64) bool {
	return value >= VerdictAccepted && value <= VerdictSystemError
}

func validASCII(value string) bool {
	for index := 0; index < len(value); index++ {
		if value[index] < 0x20 || value[index] > 0x7e {
			return false
		}
	}
	return true
}

func validateCreateInput(input *CreateSubmissionInput) error {
	if input.UserID <= 0 || input.ProblemID <= 0 || input.ProblemRevision <= 0 {
		return fmt.Errorf("%w: user, problem and revision must be positive", ErrInvalidSubmissionInput)
	}
	if input.ContestID != nil && *input.ContestID <= 0 {
		return fmt.Errorf("%w: contest ID must be positive", ErrInvalidSubmissionInput)
	}
	input.Language = strings.TrimSpace(input.Language)
	if input.Language == "" || len(input.Language) > 64 || !validASCII(input.Language) {
		return fmt.Errorf("%w: language must be 1-64 printable ASCII bytes", ErrInvalidSubmissionInput)
	}
	if input.SourceCode == "" || len(input.SourceCode) > maxSourceCodeBytes || !utf8.ValidString(input.SourceCode) {
		return fmt.Errorf("%w: source code must be valid UTF-8 and at most %d bytes", ErrInvalidSubmissionInput, maxSourceCodeBytes)
	}
	input.ClientRequestID = strings.TrimSpace(input.ClientRequestID)
	if input.ClientRequestID == "" || len(input.ClientRequestID) > 128 || !validASCII(input.ClientRequestID) {
		return fmt.Errorf("%w: client request ID must be 1-128 printable ASCII bytes", ErrInvalidSubmissionInput)
	}
	return nil
}

func createRequestFingerprint(input CreateSubmissionInput) []byte {
	hash := sha256.New()
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], uint64(input.ProblemID))
	_, _ = hash.Write(encoded[:])
	if input.ContestID == nil {
		_, _ = hash.Write([]byte{0})
	} else {
		_, _ = hash.Write([]byte{1})
		binary.BigEndian.PutUint64(encoded[:], uint64(*input.ContestID))
		_, _ = hash.Write(encoded[:])
	}
	writeFingerprintPart(hash, input.Language)
	writeFingerprintPart(hash, input.SourceCode)
	return hash.Sum(nil)
}

type hashWriter interface{ Write([]byte) (int, error) }

func writeFingerprintPart(hash hashWriter, value string) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], uint64(len(value)))
	_, _ = hash.Write(encoded[:])
	_, _ = hash.Write([]byte(value))
}

func randomUUID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate event ID: %w", err)
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	encoded := make([]byte, 36)
	hex.Encode(encoded[0:8], value[0:4])
	encoded[8] = '-'
	hex.Encode(encoded[9:13], value[4:6])
	encoded[13] = '-'
	hex.Encode(encoded[14:18], value[6:8])
	encoded[18] = '-'
	hex.Encode(encoded[19:23], value[8:10])
	encoded[23] = '-'
	hex.Encode(encoded[24:36], value[10:16])
	return string(encoded), nil
}

func randomLeaseToken() (string, []byte, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", nil, fmt.Errorf("generate lease token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(value)
	digest := sha256.Sum256([]byte(token))
	return token, digest[:], nil
}

func leaseTokenDigest(token string) []byte {
	digest := sha256.Sum256([]byte(token))
	return digest[:]
}

func sameDigest(left, right []byte) bool {
	return len(left) == sha256.Size && len(right) == sha256.Size && subtle.ConstantTimeCompare(left, right) == 1
}

func isDuplicateKey(err error) bool {
	var mysqlError *drivermysql.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}

func normalizeDatabaseError(err error) error {
	if err == nil {
		return nil
	}
	var mysqlError *drivermysql.MySQLError
	if errors.As(err, &mysqlError) {
		switch mysqlError.Number {
		case 1062:
			return fmt.Errorf("%w: duplicate key", ErrDatabaseConflict)
		case 1205:
			return fmt.Errorf("%w: lock wait timeout", ErrDatabaseConflict)
		case 1213:
			return fmt.Errorf("%w: deadlock", ErrDatabaseConflict)
		}
	}
	return err
}

func (m *defaultSubmitModel) withReadTransaction(ctx context.Context, fn func(*gorm.DB) error) error {
	tx := m.db.WithContext(ctx).Begin(&sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if tx.Error != nil {
		return normalizeDatabaseError(tx.Error)
	}
	defer func() { _ = tx.Rollback().Error }()
	if err := fn(tx); err != nil {
		return err
	}
	return normalizeDatabaseError(tx.Commit().Error)
}

func findSubmissionRow(tx *gorm.DB, submissionID int64, lock bool) (*submissionRow, error) {
	query := tx.Where("id = ?", submissionID)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var row submissionRow
	if err := query.Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSubmissionNotFound
		}
		return nil, normalizeDatabaseError(err)
	}
	return &row, nil
}

func findAttemptRow(tx *gorm.DB, attemptID int64, lock bool) (*attemptRow, error) {
	query := tx.Where("id = ?", attemptID)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var row attemptRow
	if err := query.Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAttemptNotFound
		}
		return nil, normalizeDatabaseError(err)
	}
	return &row, nil
}

func submissionSummaryFromRow(row *submissionRow) SubmissionSummary {
	currentAttemptID := int64(0)
	if row.CurrentAttemptID != nil {
		currentAttemptID = *row.CurrentAttemptID
	}
	return SubmissionSummary{
		SubmissionID: row.ID, UserID: row.UserID, ProblemID: row.ProblemID,
		ContestID: cloneInt64Pointer(row.ContestID), ProblemRevision: row.ProblemRevision,
		Language: row.Language, Status: row.Status, Verdict: cloneInt64Pointer(row.Verdict),
		Score: cloneInt64Pointer(row.Score), MaxScore: cloneInt64Pointer(row.MaxScore),
		AttemptCount: row.AttemptCount, CurrentAttemptID: currentAttemptID,
		SubmittedAt: row.SubmittedAt.UTC(), JudgingStartedAt: utcTimePointer(row.JudgingStartedAt),
		FinishedAt: utcTimePointer(row.FinishedAt), Version: row.Version,
	}
}

func attemptFromRow(row *attemptRow) *Attempt {
	return &Attempt{
		AttemptID: row.ID, SubmissionID: row.SubmissionID, AttemptNo: row.AttemptNo,
		Kind: row.Kind, Status: row.Status, RequestedBy: cloneInt64Pointer(row.RequestedBy),
		Reason: row.Reason, WorkerID: row.WorkerID, DeliveryID: row.DeliveryID,
		LeaseExpiresAt: utcTimePointer(row.LeaseExpiresAt), LastHeartbeatAt: utcTimePointer(row.LastHeartbeatAt),
		CompletedTestCases: row.CompletedTestCases, ReportID: cloneStringPointer(row.ReportID),
		Verdict: cloneInt64Pointer(row.Verdict), Score: cloneInt64Pointer(row.Score), MaxScore: cloneInt64Pointer(row.MaxScore),
		CompileOutput: row.CompileOutput, EnqueuedAt: row.EnqueuedAt.UTC(), StartedAt: utcTimePointer(row.StartedAt),
		FinishedAt: utcTimePointer(row.FinishedAt), SupersededAt: utcTimePointer(row.SupersededAt),
		CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC(),
	}
}

func cloneInt64Pointer(value *int64) *int64 {
	if value == nil {
		return nil
	}
	result := *value
	return &result
}

func cloneStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	result := *value
	return &result
}

func caseResultFromRow(row caseResultRow) CaseResult {
	return CaseResult{
		TestCaseID: row.TestCaseID, Ordinal: row.Ordinal, GroupName: row.GroupName,
		Verdict: row.Verdict, Score: row.Score, TimeUsedMS: row.TimeUsedMS,
		MemoryUsedKB: row.MemoryUsedKB, JudgeMessage: row.JudgeMessage,
	}
}

func outboxEventFromRow(row outboxRow) OutboxEvent {
	return OutboxEvent{
		ID: row.ID, EventID: row.EventID, EventType: row.EventType, AggregateID: row.AggregateID,
		AttemptID: row.AttemptID, EventKey: row.EventKey, Payload: append([]byte(nil), row.Payload...),
		Status: row.Status, AvailableAt: row.AvailableAt.UTC(), PublishAttempts: row.PublishAttempts,
		LockedBy: cloneStringPointer(row.LockedBy), LockedUntil: utcTimePointer(row.LockedUntil),
		LastError: row.LastError, CreatedAt: row.CreatedAt.UTC(), PublishedAt: utcTimePointer(row.PublishedAt),
	}
}

func getSubmissionAggregateTx(tx *gorm.DB, row *submissionRow) (*SubmissionAggregate, error) {
	if row.CurrentAttemptID == nil || *row.CurrentAttemptID <= 0 {
		return nil, fmt.Errorf("%w: submission has no current attempt", ErrSubmissionStateConflict)
	}
	var attempt attemptRow
	if err := tx.Where("id = ? AND submission_id = ?", *row.CurrentAttemptID, row.ID).Take(&attempt).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: current attempt is missing", ErrSubmissionStateConflict)
		}
		return nil, normalizeDatabaseError(err)
	}
	var caseRows []caseResultRow
	if err := tx.Where("attempt_id = ?", attempt.ID).Order("ordinal ASC, id ASC").Find(&caseRows).Error; err != nil {
		return nil, normalizeDatabaseError(err)
	}
	caseResults := make([]CaseResult, len(caseRows))
	for index := range caseRows {
		caseResults[index] = caseResultFromRow(caseRows[index])
	}
	return &SubmissionAggregate{
		SubmissionSummary: submissionSummaryFromRow(row),
		SourceCode:        row.SourceCode, CurrentAttempt: attemptFromRow(&attempt), CaseResults: caseResults,
	}, nil
}

func createAttemptRow(submissionID int64, attemptNo int32, kind int64, requestedBy *int64, reason string, at time.Time) *attemptRow {
	return &attemptRow{
		SubmissionID: submissionID, AttemptNo: attemptNo, Kind: kind, Status: AttemptStatusQueued,
		RequestedBy: requestedBy, Reason: reason, WorkerID: "", DeliveryID: "",
		CompletedTestCases: 0, CompileOutput: "", EnqueuedAt: at, CreatedAt: at, UpdatedAt: at,
	}
}

type judgeTaskEventPayload struct {
	EventID      string    `json:"event_id"`
	SubmissionID int64     `json:"submission_id"`
	AttemptID    int64     `json:"attempt_id"`
	EnqueuedAt   time.Time `json:"enqueued_at"`
}

type submissionJudgedEventPayload struct {
	EventID      string    `json:"event_id"`
	SubmissionID int64     `json:"submission_id"`
	AttemptID    int64     `json:"attempt_id"`
	ContestID    int64     `json:"contest_id"`
	ProblemID    int64     `json:"problem_id"`
	UserID       int64     `json:"user_id"`
	Verdict      int64     `json:"verdict"`
	Score        int64     `json:"score"`
	MaxScore     int64     `json:"max_score"`
	SubmittedAt  time.Time `json:"submitted_at"`
	JudgedAt     time.Time `json:"judged_at"`
}

func createJudgeTaskOutboxTx(tx *gorm.DB, submissionID, attemptID int64, at time.Time) error {
	eventID, err := randomUUID()
	if err != nil {
		return err
	}
	payload, err := json.Marshal(judgeTaskEventPayload{EventID: eventID, SubmissionID: submissionID, AttemptID: attemptID, EnqueuedAt: at})
	if err != nil {
		return fmt.Errorf("marshal judge task outbox event: %w", err)
	}
	row := outboxRow{
		EventID: eventID, EventType: OutboxEventJudgeTask, AggregateID: submissionID, AttemptID: attemptID,
		EventKey: strconv.FormatInt(submissionID, 10), Payload: payload, Status: OutboxStatusPending,
		AvailableAt: at, PublishAttempts: 0, LastError: "", CreatedAt: at,
	}
	return normalizeDatabaseError(tx.Create(&row).Error)
}

func normalizeRejudgeReason(reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	if len(reason) > 500 || !utf8.ValidString(reason) {
		return "", fmt.Errorf("%w: rejudge reason is too long or invalid UTF-8", ErrInvalidSubmissionInput)
	}
	return reason, nil
}

func uniqueSortedPositiveIDs(values []int64, maximum int) ([]int64, error) {
	if len(values) == 0 || len(values) > maximum {
		return nil, fmt.Errorf("%w: ID count must be 1-%d", ErrInvalidSubmissionInput, maximum)
	}
	seen := make(map[int64]struct{}, len(values))
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			return nil, fmt.Errorf("%w: IDs must be positive", ErrInvalidSubmissionInput)
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Slice(result, func(left, right int) bool { return result[left] < result[right] })
	return result, nil
}

func normalizeListFilter(filter *SubmissionListFilter) error {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > maxListPageSize {
		return fmt.Errorf("%w: page size exceeds %d", ErrInvalidSubmissionInput, maxListPageSize)
	}
	if filter.Page > math.MaxInt/filter.PageSize {
		return fmt.Errorf("%w: page offset overflows", ErrInvalidSubmissionInput)
	}
	for _, value := range []*int64{filter.UserID, filter.ProblemID, filter.ContestID} {
		if value != nil && *value <= 0 {
			return fmt.Errorf("%w: filter IDs must be positive", ErrInvalidSubmissionInput)
		}
	}
	if filter.Status != nil && (*filter.Status < SubmissionStatusQueued || *filter.Status > SubmissionStatusFinished) {
		return fmt.Errorf("%w: invalid submission status", ErrInvalidSubmissionInput)
	}
	if filter.Verdict != nil && !validVerdict(*filter.Verdict) {
		return fmt.Errorf("%w: invalid verdict", ErrInvalidSubmissionInput)
	}
	filter.Language = strings.TrimSpace(filter.Language)
	if len(filter.Language) > 64 || (filter.Language != "" && !validASCII(filter.Language)) {
		return fmt.Errorf("%w: invalid language filter", ErrInvalidSubmissionInput)
	}
	return nil
}

func validOutboxErrorCode(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for index := 0; index < len(value); index++ {
		character := value[index]
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') ||
			character == '.' || character == '_' || character == '-' || character == ':' {
			continue
		}
		return false
	}
	return true
}

func encodeReportFingerprint(input ApplyJudgeResultInput) ([]byte, []CaseResult, error) {
	if input.SubmissionID <= 0 || input.AttemptID <= 0 || strings.TrimSpace(input.WorkerID) == "" ||
		strings.TrimSpace(input.LeaseToken) == "" || strings.TrimSpace(input.ReportID) == "" {
		return nil, nil, fmt.Errorf("%w: report identity and lease fields are required", ErrInvalidSubmissionInput)
	}
	if len(input.WorkerID) > 128 || !validASCII(input.WorkerID) || len(input.LeaseToken) > 256 ||
		len(input.ReportID) > 128 || !validASCII(input.ReportID) {
		return nil, nil, fmt.Errorf("%w: invalid report identity field", ErrInvalidSubmissionInput)
	}
	if !validVerdict(input.Verdict) || input.Score < 0 || input.MaxScore < 0 || input.Score > input.MaxScore {
		return nil, nil, fmt.Errorf("%w: invalid aggregate judge result", ErrInvalidSubmissionInput)
	}
	if len(input.CompileOutput) > maxCompileOutputBytes || !utf8.ValidString(input.CompileOutput) {
		return nil, nil, fmt.Errorf("%w: compile output is too large or invalid UTF-8", ErrInvalidSubmissionInput)
	}
	if len(input.CaseResults) > math.MaxInt32 {
		return nil, nil, fmt.Errorf("%w: too many test case results", ErrInvalidSubmissionInput)
	}
	normalized := append([]CaseResult(nil), input.CaseResults...)
	sort.Slice(normalized, func(left, right int) bool {
		if normalized[left].Ordinal == normalized[right].Ordinal {
			return normalized[left].TestCaseID < normalized[right].TestCaseID
		}
		return normalized[left].Ordinal < normalized[right].Ordinal
	})
	caseIDs := make(map[int64]struct{}, len(normalized))
	ordinals := make(map[int32]struct{}, len(normalized))
	for index := range normalized {
		item := &normalized[index]
		if item.TestCaseID <= 0 || item.Ordinal < 0 || !validVerdict(item.Verdict) || item.Score < 0 ||
			item.TimeUsedMS < 0 || item.MemoryUsedKB < 0 || len(item.GroupName) > 128 || !utf8.ValidString(item.GroupName) ||
			len(item.JudgeMessage) > maxJudgeMessageBytes || !utf8.ValidString(item.JudgeMessage) {
			return nil, nil, fmt.Errorf("%w: invalid test case result", ErrInvalidSubmissionInput)
		}
		if _, exists := caseIDs[item.TestCaseID]; exists {
			return nil, nil, fmt.Errorf("%w: duplicate test case ID", ErrInvalidSubmissionInput)
		}
		if _, exists := ordinals[item.Ordinal]; exists {
			return nil, nil, fmt.Errorf("%w: duplicate test case ordinal", ErrInvalidSubmissionInput)
		}
		caseIDs[item.TestCaseID] = struct{}{}
		ordinals[item.Ordinal] = struct{}{}
	}
	type canonicalCaseResult struct {
		TestCaseID   int64  `json:"test_case_id"`
		Ordinal      int32  `json:"ordinal"`
		GroupName    string `json:"group_name"`
		Verdict      int64  `json:"verdict"`
		Score        int64  `json:"score"`
		TimeUsedMS   int64  `json:"time_used_ms"`
		MemoryUsedKB int64  `json:"memory_used_kb"`
		JudgeMessage string `json:"judge_message"`
	}
	canonicalCases := make([]canonicalCaseResult, len(normalized))
	for index := range normalized {
		item := normalized[index]
		canonicalCases[index] = canonicalCaseResult{
			TestCaseID: item.TestCaseID, Ordinal: item.Ordinal, GroupName: item.GroupName,
			Verdict: item.Verdict, Score: item.Score, TimeUsedMS: item.TimeUsedMS,
			MemoryUsedKB: item.MemoryUsedKB, JudgeMessage: item.JudgeMessage,
		}
	}
	canonical := struct {
		WorkerID      string                `json:"worker_id"`
		Verdict       int64                 `json:"verdict"`
		Score         int64                 `json:"score"`
		MaxScore      int64                 `json:"max_score"`
		CompileOutput string                `json:"compile_output"`
		Cases         []canonicalCaseResult `json:"cases"`
	}{input.WorkerID, input.Verdict, input.Score, input.MaxScore, input.CompileOutput, canonicalCases}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal report fingerprint: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return digest[:], normalized, nil
}
