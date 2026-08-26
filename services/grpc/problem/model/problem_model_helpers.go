package model

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type problemTagDefinitionRow struct {
	ID             int64  `gorm:"column:id;primaryKey"`
	Name           string `gorm:"column:name"`
	NormalizedName string `gorm:"column:normalized_name"`
}

func (problemTagDefinitionRow) TableName() string { return "problem_tag_definitions" }

type problemRevisionTagRow struct {
	ProblemRevisionID int64 `gorm:"column:problem_revision_id;primaryKey"`
	TagDefinitionID   int64 `gorm:"column:tag_definition_id;primaryKey"`
}

func (problemRevisionTagRow) TableName() string { return "problem_revision_tags" }

func validateEditable(editable *EditableProblem) error {
	editable.Title = strings.TrimSpace(editable.Title)
	editable.Source = strings.TrimSpace(editable.Source)
	if editable.Title == "" || utf8.RuneCountInString(editable.Title) > 200 {
		return fmt.Errorf("%w: title is required and must be at most 200 characters", ErrInvalidProblemInput)
	}
	if editable.Difficulty < 1 || editable.Difficulty > 5 {
		return fmt.Errorf("%w: difficulty must be between 1 and 5", ErrInvalidProblemInput)
	}
	if editable.Visibility < ProblemVisibilityPrivate || editable.Visibility > ProblemVisibilityContestOnly {
		return fmt.Errorf("%w: invalid visibility", ErrInvalidProblemInput)
	}
	if editable.TimeLimitMS <= 0 || editable.MemoryLimitMB <= 0 {
		return fmt.Errorf("%w: time and memory limits must be positive", ErrInvalidProblemInput)
	}
	if editable.CheckerType < 1 || editable.CheckerType > 3 {
		return fmt.Errorf("%w: invalid checker type", ErrInvalidProblemInput)
	}
	if utf8.RuneCountInString(editable.Source) > 255 {
		return fmt.Errorf("%w: source must be at most 255 characters", ErrInvalidProblemInput)
	}

	languages := make([]string, 0, len(editable.AllowedLanguages))
	seenLanguages := make(map[string]struct{}, len(editable.AllowedLanguages))
	for _, language := range editable.AllowedLanguages {
		language = strings.TrimSpace(language)
		if language == "" {
			return fmt.Errorf("%w: allowed language cannot be empty", ErrInvalidProblemInput)
		}
		if _, ok := seenLanguages[language]; ok {
			continue
		}
		seenLanguages[language] = struct{}{}
		languages = append(languages, language)
	}
	if len(languages) == 0 {
		return fmt.Errorf("%w: at least one allowed language is required", ErrInvalidProblemInput)
	}
	editable.AllowedLanguages = languages

	tags, err := canonicalTags(editable.Tags)
	if err != nil {
		return err
	}
	editable.Tags = tags
	return nil
}

func canonicalTags(tags []string) ([]string, error) {
	byNormalized := make(map[string]string, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			return nil, fmt.Errorf("%w: tag cannot be empty", ErrInvalidProblemInput)
		}
		if utf8.RuneCountInString(tag) > 64 {
			return nil, fmt.Errorf("%w: tag must be at most 64 characters", ErrInvalidProblemInput)
		}
		normalized := strings.ToLower(tag)
		if _, exists := byNormalized[normalized]; !exists {
			byNormalized[normalized] = tag
		}
	}
	normalized := make([]string, 0, len(byNormalized))
	for value := range byNormalized {
		normalized = append(normalized, value)
	}
	sort.Strings(normalized)
	result := make([]string, 0, len(normalized))
	for _, value := range normalized {
		result = append(result, byNormalized[value])
	}
	return result, nil
}

func normalizedTags(tags []string) ([]string, error) {
	canonical, err := canonicalTags(tags)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(canonical))
	for _, tag := range canonical {
		result = append(result, strings.ToLower(tag))
	}
	return result, nil
}

func applyPatch(editable *EditableProblem, patch ProblemPatch) {
	if patch.Title != nil {
		editable.Title = *patch.Title
	}
	if patch.Description != nil {
		editable.Description = *patch.Description
	}
	if patch.InputDescription != nil {
		editable.InputDescription = *patch.InputDescription
	}
	if patch.OutputDescription != nil {
		editable.OutputDescription = *patch.OutputDescription
	}
	if patch.Samples != nil {
		editable.Samples = append([]ProblemSample(nil), (*patch.Samples)...)
	}
	if patch.Hint != nil {
		editable.Hint = *patch.Hint
	}
	if patch.Difficulty != nil {
		editable.Difficulty = *patch.Difficulty
	}
	if patch.Tags != nil {
		editable.Tags = append([]string(nil), (*patch.Tags)...)
	}
	if patch.Source != nil {
		editable.Source = *patch.Source
	}
	if patch.TimeLimitMS != nil {
		editable.TimeLimitMS = *patch.TimeLimitMS
	}
	if patch.MemoryLimitMB != nil {
		editable.MemoryLimitMB = *patch.MemoryLimitMB
	}
	if patch.AllowedLanguages != nil {
		editable.AllowedLanguages = append([]string(nil), (*patch.AllowedLanguages)...)
	}
	if patch.CheckerType != nil {
		editable.CheckerType = *patch.CheckerType
	}
	if patch.CheckerFileKey != nil {
		editable.CheckerFileKey = cloneStringPointer(*patch.CheckerFileKey)
	}
	if patch.InteractorFileKey != nil {
		editable.InteractorFileKey = cloneStringPointer(*patch.InteractorFileKey)
	}
	if patch.Visibility != nil {
		editable.Visibility = *patch.Visibility
	}
}

func cloneStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}

func pointerToNullString(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *value, Valid: true}
}

func nullStringPointer(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return cloneStringPointer(&value.String)
}

func nullIntPointer(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	result := value.Int64
	return &result
}

func nullTimePointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}

func encodeJSON(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode problem JSON: %w", err)
	}
	return string(encoded), nil
}

func decodeEditable(problem *Problems, tags []string) (EditableProblem, error) {
	var samples []ProblemSample
	if err := json.Unmarshal([]byte(problem.SamplesJson), &samples); err != nil {
		return EditableProblem{}, fmt.Errorf("decode samples: %w", err)
	}
	var languages []string
	if err := json.Unmarshal([]byte(problem.AllowedLanguagesJson), &languages); err != nil {
		return EditableProblem{}, fmt.Errorf("decode allowed languages: %w", err)
	}
	return EditableProblem{
		Title: problem.Title, Description: problem.Description,
		InputDescription: problem.InputDescription, OutputDescription: problem.OutputDescription,
		Samples: samples, Hint: problem.Hint, Difficulty: problem.Difficulty,
		Tags: append([]string(nil), tags...), Source: problem.Source,
		TimeLimitMS: problem.TimeLimitMs, MemoryLimitMB: problem.MemoryLimitMb,
		AllowedLanguages: languages, CheckerType: problem.CheckerType,
		CheckerFileKey:    nullStringPointer(problem.CheckerFileKey),
		InteractorFileKey: nullStringPointer(problem.InteractorFileKey),
		Visibility:        problem.Visibility,
	}, nil
}

func decodeRevision(row *problemRevisionRow, tags []string) (*ProblemRevision, error) {
	problem := &Problems{
		Title: row.Title, Description: row.Description,
		InputDescription: row.InputDescription, OutputDescription: row.OutputDescription,
		SamplesJson: row.SamplesJSON, Hint: row.Hint, Difficulty: row.Difficulty,
		Source: row.Source, TimeLimitMs: row.TimeLimitMS, MemoryLimitMb: row.MemoryLimitMB,
		AllowedLanguagesJson: row.AllowedLanguagesJSON, CheckerType: row.CheckerType,
		CheckerFileKey: row.CheckerFileKey, InteractorFileKey: row.InteractorFileKey,
		Visibility: row.Visibility,
	}
	editable, err := decodeEditable(problem, tags)
	if err != nil {
		return nil, err
	}
	return &ProblemRevision{
		ID: row.ID, ProblemID: row.ProblemID, Revision: row.Revision,
		Editable: editable, TestDataVersionID: nullIntPointer(row.TestDataVersionID),
		Status: row.Status, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt,
	}, nil
}

func aggregateFromProblem(problem *Problems, tags []string) (*ProblemAggregate, error) {
	editable, err := decodeEditable(problem, tags)
	if err != nil {
		return nil, err
	}
	return &ProblemAggregate{
		ProblemID: problem.Id, AuthorID: problem.AuthorId, Editable: editable,
		CurrentTestDataVersionID: nullIntPointer(problem.CurrentTestDataVersionId),
		LatestPublishedRevision:  nullIntPointer(problem.LatestPublishedRevision),
		Status:                   problem.Status, Revision: problem.Revision,
		PublishedAt: nullTimePointer(problem.PublishedAt), CreatedAt: problem.CreatedAt,
		UpdatedAt: problem.UpdatedAt, DeletedAt: nullTimePointer(problem.DeletedAt),
	}, nil
}

func editableJSON(editable EditableProblem) (samplesJSON, languagesJSON string, err error) {
	samplesJSON, err = encodeJSON(editable.Samples)
	if err != nil {
		return "", "", err
	}
	languagesJSON, err = encodeJSON(editable.AllowedLanguages)
	return samplesJSON, languagesJSON, err
}

func insertRevision(tx *gorm.DB, problem *Problems, editable EditableProblem, actorID int64, now time.Time) (*problemRevisionRow, error) {
	samplesJSON, languagesJSON, err := editableJSON(editable)
	if err != nil {
		return nil, err
	}
	row := &problemRevisionRow{
		ProblemID: problem.Id, Revision: problem.Revision,
		Title: editable.Title, Description: editable.Description,
		InputDescription: editable.InputDescription, OutputDescription: editable.OutputDescription,
		SamplesJSON: samplesJSON, Hint: editable.Hint, Difficulty: editable.Difficulty,
		Source: editable.Source, TimeLimitMS: editable.TimeLimitMS,
		MemoryLimitMB: editable.MemoryLimitMB, AllowedLanguagesJSON: languagesJSON,
		CheckerType:       editable.CheckerType,
		CheckerFileKey:    pointerToNullString(editable.CheckerFileKey),
		InteractorFileKey: pointerToNullString(editable.InteractorFileKey),
		TestDataVersionID: problem.CurrentTestDataVersionId,
		Status:            problem.Status, Visibility: editable.Visibility,
		CreatedBy: actorID, CreatedAt: now,
	}
	if err := tx.Create(row).Error; err != nil {
		return nil, fmt.Errorf("insert problem revision: %w", err)
	}
	if err := replaceRevisionTags(tx, row.ID, editable.Tags); err != nil {
		return nil, err
	}
	return row, nil
}

func replaceRevisionTags(tx *gorm.DB, revisionID int64, tags []string) error {
	if len(tags) == 0 {
		return nil
	}
	definitions := make([]problemTagDefinitionRow, 0, len(tags))
	for _, tag := range tags {
		definitions = append(definitions, problemTagDefinitionRow{Name: tag, NormalizedName: strings.ToLower(tag)})
	}
	if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "normalized_name"}}, DoNothing: true}).Create(&definitions).Error; err != nil {
		return fmt.Errorf("upsert problem tags: %w", err)
	}
	names := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		names = append(names, definition.NormalizedName)
	}
	var stored []problemTagDefinitionRow
	if err := tx.Where("normalized_name IN ?", names).Find(&stored).Error; err != nil {
		return fmt.Errorf("load problem tags: %w", err)
	}
	links := make([]problemRevisionTagRow, 0, len(stored))
	for _, definition := range stored {
		links = append(links, problemRevisionTagRow{ProblemRevisionID: revisionID, TagDefinitionID: definition.ID})
	}
	if len(links) > 0 {
		if err := tx.Create(&links).Error; err != nil {
			return fmt.Errorf("link problem revision tags: %w", err)
		}
	}
	return nil
}

func loadTagsByRevisionIDs(db *gorm.DB, revisionIDs []int64) (map[int64][]string, error) {
	result := make(map[int64][]string, len(revisionIDs))
	if len(revisionIDs) == 0 {
		return result, nil
	}
	var rows []struct {
		ProblemRevisionID int64  `gorm:"column:problem_revision_id"`
		Name              string `gorm:"column:name"`
	}
	err := db.Table("problem_revision_tags AS prt").
		Select("prt.problem_revision_id, td.name").
		Joins("JOIN problem_tag_definitions AS td ON td.id = prt.tag_definition_id").
		Where("prt.problem_revision_id IN ?", revisionIDs).
		Order("prt.problem_revision_id ASC, td.normalized_name ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("load revision tags: %w", err)
	}
	for _, row := range rows {
		result[row.ProblemRevisionID] = append(result[row.ProblemRevisionID], row.Name)
	}
	return result, nil
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func newUUID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate upload id: %w", err)
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", value[0:4], value[4:6], value[6:8], value[8:10], value[10:16]), nil
}

func safeFileName(value string) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	value = path.Base(value)
	if value == "" || value == "." || strings.ContainsRune(value, '\x00') || utf8.RuneCountInString(value) > 255 {
		return "", fmt.Errorf("%w: invalid file name", ErrInvalidProblemInput)
	}
	return value, nil
}
