package model

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

func (m *defaultProblemModel) GetForEdit(ctx context.Context, problemID int64) (*ProblemAggregate, error) {
	if problemID <= 0 {
		return nil, fmt.Errorf("%w: problem id must be positive", ErrInvalidProblemInput)
	}
	problem := new(Problems)
	err := m.db.WithContext(ctx).Where("id = ?", problemID).Take(problem).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrProblemNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get problem for edit: %w", err)
	}
	tags, err := loadTagsForProblemRevision(m.db.WithContext(ctx), problem.Id, problem.Revision)
	if err != nil {
		return nil, err
	}
	return aggregateFromProblem(problem, tags)
}

func (m *defaultProblemModel) GetPublic(ctx context.Context, problemID int64) (*ProblemAggregate, error) {
	if problemID <= 0 {
		return nil, fmt.Errorf("%w: problem id must be positive", ErrInvalidProblemInput)
	}
	var row struct {
		Problems
		PublishedRevisionID int64 `gorm:"column:published_revision_id"`
	}
	err := m.db.WithContext(ctx).Table("problems AS p").
		Select("p.*, pr.id AS published_revision_id").
		Joins("JOIN problem_revisions AS pr ON pr.problem_id = p.id AND pr.revision = p.latest_published_revision").
		Where("p.id = ? AND p.status = ? AND p.deleted_at IS NULL AND pr.visibility = ?", problemID, ProblemStatusPublished, ProblemVisibilityPublic).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrProblemNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get public problem: %w", err)
	}
	var revisionRow problemRevisionRow
	if err := m.db.WithContext(ctx).Where("id = ?", row.PublishedRevisionID).Take(&revisionRow).Error; err != nil {
		return nil, fmt.Errorf("get published problem revision: %w", err)
	}
	tagsByRevision, err := loadTagsByRevisionIDs(m.db.WithContext(ctx), []int64{revisionRow.ID})
	if err != nil {
		return nil, err
	}
	revision, err := decodeRevision(&revisionRow, tagsByRevision[revisionRow.ID])
	if err != nil {
		return nil, err
	}
	root := row.Problems
	root.Revision = revision.Revision
	root.UpdatedAt = revision.CreatedAt
	root.Title = revision.Editable.Title
	root.Description = revision.Editable.Description
	root.InputDescription = revision.Editable.InputDescription
	root.OutputDescription = revision.Editable.OutputDescription
	root.Hint = revision.Editable.Hint
	root.Difficulty = revision.Editable.Difficulty
	root.Source = revision.Editable.Source
	root.TimeLimitMs = revision.Editable.TimeLimitMS
	root.MemoryLimitMb = revision.Editable.MemoryLimitMB
	root.CheckerType = revision.Editable.CheckerType
	root.CheckerFileKey = pointerToNullString(revision.Editable.CheckerFileKey)
	root.InteractorFileKey = pointerToNullString(revision.Editable.InteractorFileKey)
	root.Visibility = revision.Editable.Visibility
	root.CurrentTestDataVersionId.Valid = revision.TestDataVersionID != nil
	if revision.TestDataVersionID != nil {
		root.CurrentTestDataVersionId.Int64 = *revision.TestDataVersionID
	}
	root.SamplesJson, root.AllowedLanguagesJson, err = editableJSON(revision.Editable)
	if err != nil {
		return nil, err
	}
	return aggregateFromProblem(&root, revision.Editable.Tags)
}

func (m *defaultProblemModel) GetRevision(ctx context.Context, problemID, revision int64) (*ProblemRevision, error) {
	if problemID <= 0 || revision <= 0 {
		return nil, fmt.Errorf("%w: problem and revision must be positive", ErrInvalidProblemInput)
	}
	row := new(problemRevisionRow)
	err := m.db.WithContext(ctx).Where("problem_id = ? AND revision = ?", problemID, revision).Take(row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrProblemNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get problem revision: %w", err)
	}
	tags, err := loadTagsByRevisionIDs(m.db.WithContext(ctx), []int64{row.ID})
	if err != nil {
		return nil, err
	}
	return decodeRevision(row, tags[row.ID])
}

func (m *defaultProblemModel) GetJudgeSpec(ctx context.Context, problemID, revision int64) (*JudgeProblemSpec, error) {
	if problemID <= 0 || revision < 0 {
		return nil, fmt.Errorf("%w: invalid problem or revision", ErrInvalidProblemInput)
	}
	db := m.db.WithContext(ctx)
	if revision == 0 {
		var published sql.NullInt64
		err := db.Table("problems").Select("latest_published_revision").Where("id = ?", problemID).Scan(&published).Error
		if err != nil {
			return nil, fmt.Errorf("resolve published revision: %w", err)
		}
		if !published.Valid {
			return nil, ErrProblemNotPublishable
		}
		revision = published.Int64
	}
	var row problemRevisionRow
	err := db.Where("problem_id = ? AND revision = ?", problemID, revision).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrProblemNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get judge revision: %w", err)
	}
	var publishedCount int64
	if err := db.Table("problem_judge_revisions").Where("problem_id = ? AND revision = ?", problemID, revision).Count(&publishedCount).Error; err != nil {
		return nil, fmt.Errorf("verify published judge revision: %w", err)
	}
	if publishedCount != 1 || !row.TestDataVersionID.Valid {
		return nil, ErrProblemNotPublishable
	}

	var version TestDataVersions
	if err := db.Where("id = ? AND problem_id = ?", row.TestDataVersionID.Int64, problemID).Take(&version).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProblemNotFound
		}
		return nil, fmt.Errorf("get test data version: %w", err)
	}
	var cases []TestCases
	if err := db.Where("test_data_version_id = ?", version.Id).Order("ordinal ASC, id ASC").Find(&cases).Error; err != nil {
		return nil, fmt.Errorf("get test cases: %w", err)
	}
	var languages []string
	if err := json.Unmarshal([]byte(row.AllowedLanguagesJSON), &languages); err != nil {
		return nil, fmt.Errorf("decode judge languages: %w", err)
	}
	records := make([]TestCaseRecord, 0, len(cases))
	for _, testCase := range cases {
		records = append(records, TestCaseRecord{ID: testCase.Id, TestCaseInput: TestCaseInput{
			Name: testCase.Name, Score: testCase.Score, Ordinal: testCase.Ordinal,
			Enabled: testCase.Enabled, GroupName: testCase.GroupName,
		}})
	}
	return &JudgeProblemSpec{
		ProblemID: problemID, Revision: revision,
		TimeLimitMS: row.TimeLimitMS, MemoryLimitMB: row.MemoryLimitMB,
		AllowedLanguages: languages, CheckerType: row.CheckerType,
		CheckerFileKey:    nullStringPointer(row.CheckerFileKey),
		InteractorFileKey: nullStringPointer(row.InteractorFileKey),
		TestDataVersion:   version.Version, TestDataPackageKey: version.PackageKey,
		TestDataChecksum: version.Checksum, TestCases: records,
	}, nil
}

func (m *defaultProblemModel) ListPublic(ctx context.Context, filter ProblemListFilter) ([]ProblemSummary, int64, error) {
	return m.list(ctx, filter, true)
}

func (m *defaultProblemModel) ListForManagement(ctx context.Context, filter ProblemListFilter) ([]ProblemSummary, int64, error) {
	return m.list(ctx, filter, false)
}

func (m *defaultProblemModel) list(ctx context.Context, filter ProblemListFilter, publicOnly bool) ([]ProblemSummary, int64, error) {
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	normalized, err := normalizedTags(filter.Tags)
	if err != nil {
		return nil, 0, err
	}
	revisionExpr := "p.revision"
	if publicOnly {
		revisionExpr = "p.latest_published_revision"
	}
	base := m.db.WithContext(ctx).Table("problems AS p").
		Joins("JOIN problem_revisions AS pr ON pr.problem_id = p.id AND pr.revision = " + revisionExpr)
	if publicOnly {
		base = base.Where("p.status = ? AND p.deleted_at IS NULL AND pr.visibility = ?", ProblemStatusPublished, ProblemVisibilityPublic)
	} else if filter.Status != nil && *filter.Status == ProblemStatusDeleted {
		base = base.Where("p.status = ? AND p.deleted_at IS NOT NULL", ProblemStatusDeleted)
	} else {
		base = base.Where("p.deleted_at IS NULL")
		if filter.Status != nil {
			base = base.Where("p.status = ?", *filter.Status)
		}
	}
	if !publicOnly && filter.Visibility != nil {
		base = base.Where("pr.visibility = ?", *filter.Visibility)
	}
	if filter.Difficulty != nil {
		base = base.Where("pr.difficulty = ?", *filter.Difficulty)
	}
	if filter.AuthorID != nil {
		base = base.Where("p.author_id = ?", *filter.AuthorID)
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		base = base.Where("pr.title LIKE ?", "%"+keyword+"%")
	}
	if len(normalized) > 0 {
		base = base.Where(`pr.id IN (
			SELECT prt.problem_revision_id
			FROM problem_revision_tags AS prt
			JOIN problem_tag_definitions AS td ON td.id = prt.tag_definition_id
			WHERE td.normalized_name IN ?
			GROUP BY prt.problem_revision_id
			HAVING COUNT(DISTINCT td.normalized_name) = ?
		)`, normalized, len(normalized))
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count problems: %w", err)
	}
	if total == 0 {
		return []ProblemSummary{}, 0, nil
	}
	updatedAtExpression := "p.updated_at"
	if publicOnly {
		updatedAtExpression = "pr.created_at"
	}
	var rows []summaryRow
	err = base.Select(`p.id AS problem_id, pr.id AS problem_revision_id, pr.title, pr.difficulty,
		p.status, pr.visibility, p.author_id, ` + updatedAtExpression + ` AS updated_at, pr.revision`).
		Order("p.updated_at DESC, p.id DESC").Limit(pageSize).Offset((page - 1) * pageSize).Scan(&rows).Error
	if err != nil {
		return nil, 0, fmt.Errorf("list problems: %w", err)
	}
	return summariesFromRows(m.db.WithContext(ctx), rows, total)
}

func (m *defaultProblemModel) BatchGetPublicSummaries(ctx context.Context, problemIDs []int64) ([]*ProblemSummary, error) {
	result := make([]*ProblemSummary, len(problemIDs))
	if len(problemIDs) == 0 {
		return result, nil
	}
	unique := make([]int64, 0, len(problemIDs))
	seen := make(map[int64]struct{}, len(problemIDs))
	for _, id := range problemIDs {
		if id <= 0 {
			return nil, fmt.Errorf("%w: problem id must be positive", ErrInvalidProblemInput)
		}
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			unique = append(unique, id)
		}
	}
	var rows []summaryRow
	err := m.db.WithContext(ctx).Table("problems AS p").
		Select(`p.id AS problem_id, pr.id AS problem_revision_id, pr.title, pr.difficulty,
			p.status, pr.visibility, p.author_id, pr.created_at AS updated_at, pr.revision`).
		Joins("JOIN problem_revisions AS pr ON pr.problem_id = p.id AND pr.revision = p.latest_published_revision").
		Where("p.id IN ? AND p.status = ? AND p.deleted_at IS NULL AND pr.visibility = ?", unique, ProblemStatusPublished, ProblemVisibilityPublic).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("batch get public problems: %w", err)
	}
	summaries, _, err := summariesFromRows(m.db.WithContext(ctx), rows, int64(len(rows)))
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]*ProblemSummary, len(summaries))
	for i := range summaries {
		summary := summaries[i]
		byID[summary.ProblemID] = &summary
	}
	for i, id := range problemIDs {
		if summary := byID[id]; summary != nil {
			copySummary := *summary
			copySummary.Tags = append([]string(nil), summary.Tags...)
			result[i] = &copySummary
		}
	}
	return result, nil
}

type summaryRow struct {
	ProblemID         int64     `gorm:"column:problem_id"`
	ProblemRevisionID int64     `gorm:"column:problem_revision_id"`
	Title             string    `gorm:"column:title"`
	Difficulty        int64     `gorm:"column:difficulty"`
	Status            int64     `gorm:"column:status"`
	Visibility        int64     `gorm:"column:visibility"`
	AuthorID          int64     `gorm:"column:author_id"`
	UpdatedAt         time.Time `gorm:"column:updated_at"`
	Revision          int64     `gorm:"column:revision"`
}

func summariesFromRows(db *gorm.DB, rows []summaryRow, total int64) ([]ProblemSummary, int64, error) {
	revisionIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		revisionIDs = append(revisionIDs, row.ProblemRevisionID)
	}
	tags, err := loadTagsByRevisionIDs(db, revisionIDs)
	if err != nil {
		return nil, 0, err
	}
	result := make([]ProblemSummary, 0, len(rows))
	for _, row := range rows {
		result = append(result, ProblemSummary{
			ProblemID: row.ProblemID, Title: row.Title, Difficulty: row.Difficulty,
			Tags: tags[row.ProblemRevisionID], Status: row.Status, Visibility: row.Visibility,
			AuthorID: row.AuthorID, UpdatedAt: row.UpdatedAt, Revision: row.Revision,
		})
	}
	return result, total, nil
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
