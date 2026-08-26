package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (m *defaultProblemModel) CreateTestDataUpload(ctx context.Context, input CreateTestDataUploadInput) (*TestDataUpload, error) {
	if input.ProblemID <= 0 || input.ContentLength < 0 || input.TTL <= 0 {
		return nil, fmt.Errorf("%w: invalid upload parameters", ErrInvalidProblemInput)
	}
	checksum := strings.ToLower(strings.TrimSpace(input.SHA256))
	if !validSHA256(checksum) {
		return nil, fmt.Errorf("%w: sha256 must be 64 hexadecimal characters", ErrInvalidProblemInput)
	}
	fileName, err := safeFileName(input.FileName)
	if err != nil {
		return nil, err
	}
	uploadID, err := newUUID()
	if err != nil {
		return nil, err
	}
	now := m.now()
	row := &testDataUploadRow{
		UploadID: uploadID, ProblemID: input.ProblemID,
		ObjectKey: fmt.Sprintf("problem-test-data/%d/%s/%s", input.ProblemID, uploadID, fileName),
		FileName:  fileName, ContentLength: input.ContentLength, Checksum: checksum,
		Status: TestDataUploadStatusPending, ExpiresAt: now.Add(input.TTL), CreatedAt: now,
	}
	err = m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		problem, err := lockProblem(tx, input.ProblemID)
		if err != nil {
			return err
		}
		if problem.DeletedAt.Valid {
			return ErrProblemNotFound
		}
		if err := tx.Create(row).Error; err != nil {
			return fmt.Errorf("create test data upload: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return uploadFromRow(row), nil
}

func (m *defaultProblemModel) FinalizeTestDataUpload(ctx context.Context, input FinalizeTestDataUploadInput) (*FinalizeTestDataUploadResult, error) {
	if input.ProblemID <= 0 || input.ContentLength < 0 || input.ExpectedRevision <= 0 || input.ActorID <= 0 || strings.TrimSpace(input.UploadID) == "" {
		return nil, fmt.Errorf("%w: invalid finalize parameters", ErrInvalidProblemInput)
	}
	checksum := strings.ToLower(strings.TrimSpace(input.SHA256))
	if !validSHA256(checksum) {
		return nil, fmt.Errorf("%w: invalid sha256", ErrInvalidProblemInput)
	}
	var result *FinalizeTestDataUploadResult
	err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		upload := new(testDataUploadRow)
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("upload_id = ?", input.UploadID).Take(upload).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrTestDataUploadNotFound
		}
		if err != nil {
			return fmt.Errorf("lock test data upload: %w", err)
		}
		if upload.ProblemID != input.ProblemID {
			return ErrTestDataUploadConflict
		}
		if upload.Checksum != checksum {
			if upload.Status == TestDataUploadStatusFinalized {
				return ErrTestDataUploadFinalized
			}
			return ErrTestDataUploadConflict
		}
		if upload.ContentLength != input.ContentLength {
			if upload.Status == TestDataUploadStatusFinalized {
				return ErrTestDataUploadFinalized
			}
			return ErrTestDataUploadConflict
		}
		if upload.Status == TestDataUploadStatusFinalized {
			var loadErr error
			result, loadErr = loadFinalizeResult(tx, upload)
			return loadErr
		}
		if upload.Status != TestDataUploadStatusPending {
			return ErrTestDataUploadConflict
		}
		if !m.now().Before(upload.ExpiresAt) {
			return ErrTestDataUploadExpired
		}
		if err := validateTestCases(input.TestCases); err != nil {
			return err
		}

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

		now := m.now()
		version := &TestDataVersions{
			ProblemId: problem.Id, Version: "upload-" + upload.UploadID,
			PackageKey: upload.ObjectKey, Checksum: upload.Checksum, CreatedAt: now,
		}
		if err := tx.Create(version).Error; err != nil {
			return fmt.Errorf("create test data version: %w", err)
		}
		cases := make([]TestCases, 0, len(input.TestCases))
		for _, testCase := range input.TestCases {
			cases = append(cases, TestCases{
				TestDataVersionId: version.Id, Name: testCase.Name, Score: testCase.Score,
				Ordinal: testCase.Ordinal, Enabled: testCase.Enabled, GroupName: testCase.GroupName,
			})
		}
		if err := tx.Create(&cases).Error; err != nil {
			return fmt.Errorf("create test cases: %w", err)
		}

		problem.CurrentTestDataVersionId = sql.NullInt64{Int64: version.Id, Valid: true}
		problem.Revision++
		problem.UpdatedAt = now
		updates := currentProblemUpdates(problem, editable)
		updates["current_test_data_version_id"] = version.Id
		if err := conditionalProblemUpdate(tx, problem.Id, input.ExpectedRevision, false, updates); err != nil {
			return err
		}
		if _, err := insertRevision(tx, problem, editable, input.ActorID, now); err != nil {
			return err
		}

		updateResult := tx.Model(&testDataUploadRow{}).
			Where("upload_id = ? AND status = ?", upload.UploadID, TestDataUploadStatusPending).
			Updates(map[string]any{
				"status": TestDataUploadStatusFinalized, "finalized_at": now,
				"finalized_test_data_version_id": version.Id, "finalized_revision": problem.Revision,
			})
		if updateResult.Error != nil {
			return fmt.Errorf("finalize test data upload: %w", updateResult.Error)
		}
		if updateResult.RowsAffected != 1 {
			return ErrTestDataUploadConflict
		}

		records := make([]TestCaseRecord, 0, len(cases))
		for i := range cases {
			records = append(records, TestCaseRecord{ID: cases[i].Id, TestCaseInput: input.TestCases[i]})
		}
		result = &FinalizeTestDataUploadResult{TestDataVersion: version.Version, Revision: problem.Revision, TestCases: records}
		return nil
	})
	return result, err
}

func validateTestCases(cases []TestCaseInput) error {
	if len(cases) == 0 {
		return fmt.Errorf("%w: at least one test case is required", ErrInvalidProblemInput)
	}
	ordinals := make(map[int64]struct{}, len(cases))
	for i := range cases {
		cases[i].Name = strings.TrimSpace(cases[i].Name)
		if cases[i].Name == "" || utf8.RuneCountInString(cases[i].Name) > 128 {
			return fmt.Errorf("%w: invalid test case name", ErrInvalidProblemInput)
		}
		if cases[i].Score < 0 || cases[i].Ordinal < 0 {
			return fmt.Errorf("%w: test case score and ordinal must be non-negative", ErrInvalidProblemInput)
		}
		if utf8.RuneCountInString(cases[i].GroupName) > 128 {
			return fmt.Errorf("%w: test case group name is too long", ErrInvalidProblemInput)
		}
		if _, exists := ordinals[cases[i].Ordinal]; exists {
			return fmt.Errorf("%w: duplicate test case ordinal", ErrInvalidProblemInput)
		}
		ordinals[cases[i].Ordinal] = struct{}{}
	}
	return nil
}

func loadFinalizeResult(tx *gorm.DB, upload *testDataUploadRow) (*FinalizeTestDataUploadResult, error) {
	if !upload.FinalizedTestDataVersionID.Valid || !upload.FinalizedRevision.Valid {
		return nil, ErrTestDataUploadConflict
	}
	var version TestDataVersions
	if err := tx.Where("id = ? AND problem_id = ?", upload.FinalizedTestDataVersionID.Int64, upload.ProblemID).Take(&version).Error; err != nil {
		return nil, fmt.Errorf("load finalized test data version: %w", err)
	}
	var cases []TestCases
	if err := tx.Where("test_data_version_id = ?", version.Id).Order("ordinal ASC, id ASC").Find(&cases).Error; err != nil {
		return nil, fmt.Errorf("load finalized test cases: %w", err)
	}
	records := make([]TestCaseRecord, 0, len(cases))
	for _, testCase := range cases {
		records = append(records, TestCaseRecord{ID: testCase.Id, TestCaseInput: TestCaseInput{
			Name: testCase.Name, Score: testCase.Score, Ordinal: testCase.Ordinal,
			Enabled: testCase.Enabled, GroupName: testCase.GroupName,
		}})
	}
	return &FinalizeTestDataUploadResult{TestDataVersion: version.Version, Revision: upload.FinalizedRevision.Int64, TestCases: records}, nil
}

func uploadFromRow(row *testDataUploadRow) *TestDataUpload {
	return &TestDataUpload{
		UploadID: row.UploadID, ProblemID: row.ProblemID, ObjectKey: row.ObjectKey,
		FileName: row.FileName, ContentLength: row.ContentLength, SHA256: row.Checksum,
		Status: row.Status, ExpiresAt: row.ExpiresAt, CreatedAt: row.CreatedAt,
		FinalizedAt: nullTimePointer(row.FinalizedAt),
	}
}
