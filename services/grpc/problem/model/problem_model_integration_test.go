package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestProblemModelRevisionLifecycle(t *testing.T) {
	repository, _ := newProblemModelTestDatabase(t)
	ctx := context.Background()

	created, err := repository.Create(ctx, CreateProblemInput{AuthorID: 101, Editable: testEditableProblem()})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Revision != 1 || created.Status != ProblemStatusDraft {
		t.Fatalf("created state = revision %d status %d", created.Revision, created.Status)
	}
	if _, total, err := repository.ListPublic(ctx, ProblemListFilter{}); err != nil || total != 0 {
		t.Fatalf("draft leaked through ListPublic: total=%d err=%v", total, err)
	}

	upload, err := repository.CreateTestDataUpload(ctx, CreateTestDataUploadInput{
		ProblemID: created.ProblemID, FileName: "tests.zip", ContentLength: 1024,
		SHA256: strings.Repeat("a", 64), TTL: time.Hour,
	})
	if err != nil {
		t.Fatalf("CreateTestDataUpload() error = %v", err)
	}
	finalizeInput := FinalizeTestDataUploadInput{
		ProblemID: created.ProblemID, UploadID: upload.UploadID, SHA256: strings.Repeat("a", 64),
		ContentLength: 1024, PackageKey: "problem-test-data/immutable/first.zip", ExpectedRevision: 1, ActorID: 101,
		TestCases: []TestCaseInput{
			{Name: "case-2", Score: 40, Ordinal: 1, Enabled: true},
			{Name: "case-1", Score: 60, Ordinal: 0, Enabled: true},
		},
	}
	finalized, err := repository.FinalizeTestDataUpload(ctx, finalizeInput)
	if err != nil {
		t.Fatalf("FinalizeTestDataUpload() error = %v", err)
	}
	if finalized.Revision != 2 {
		t.Fatalf("finalized revision = %d, want 2", finalized.Revision)
	}

	published, err := repository.Publish(ctx, created.ProblemID, 2, 101)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if published.Revision != 3 || published.LatestPublishedRevision == nil || *published.LatestPublishedRevision != 3 {
		t.Fatalf("published state = %+v", published)
	}

	spec, err := repository.GetJudgeSpec(ctx, created.ProblemID, 0)
	if err != nil {
		t.Fatalf("GetJudgeSpec(0) error = %v", err)
	}
	if spec.Revision != 3 || spec.TestDataPackageKey != finalizeInput.PackageKey ||
		len(spec.TestCases) != 2 || spec.TestCases[0].Ordinal != 0 || spec.TestCases[1].Ordinal != 1 {
		t.Fatalf("judge spec = revision %d cases %+v", spec.Revision, spec.TestCases)
	}

	editedTitle := "Edited but not published"
	editedTags := []string{"new-tag"}
	edited, err := repository.Update(ctx, UpdateProblemInput{
		ProblemID: created.ProblemID, ExpectedRevision: 3, ActorID: 101,
		Patch: ProblemPatch{Title: &editedTitle, Tags: &editedTags},
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if edited.Revision != 4 {
		t.Fatalf("edited revision = %d, want 4", edited.Revision)
	}

	publicBeforeRepublish, err := repository.GetPublic(ctx, created.ProblemID)
	if err != nil {
		t.Fatalf("GetPublic() error = %v", err)
	}
	if publicBeforeRepublish.Revision != 3 || publicBeforeRepublish.Editable.Title != "Original title" {
		t.Fatalf("unpublished edit leaked to public view: revision %d title %q", publicBeforeRepublish.Revision, publicBeforeRepublish.Editable.Title)
	}
	oldRevision, err := repository.GetRevision(ctx, created.ProblemID, 3)
	if err != nil {
		t.Fatalf("GetRevision(3) error = %v", err)
	}
	if oldRevision.Editable.Title != "Original title" {
		t.Fatalf("revision 3 title = %q", oldRevision.Editable.Title)
	}

	graphList, graphTotal, err := repository.ListPublic(ctx, ProblemListFilter{Tags: []string{"graph"}})
	if err != nil {
		t.Fatalf("ListPublic(graph) error = %v", err)
	}
	if graphTotal != 1 || len(graphList) != 1 || graphList[0].Revision != 3 {
		t.Fatalf("graph list = total %d rows %+v", graphTotal, graphList)
	}
	_, bothTagsTotal, err := repository.ListPublic(ctx, ProblemListFilter{Tags: []string{"graph", "DP"}})
	if err != nil || bothTagsTotal != 1 {
		t.Fatalf("all-tag filter total=%d err=%v", bothTagsTotal, err)
	}
	_, missingTagTotal, err := repository.ListPublic(ctx, ProblemListFilter{Tags: []string{"graph", "missing"}})
	if err != nil || missingTagTotal != 0 {
		t.Fatalf("all-tag filter accepted a partial match: total=%d err=%v", missingTagTotal, err)
	}
	_, newTagTotal, err := repository.ListPublic(ctx, ProblemListFilter{Tags: []string{"new-tag"}})
	if err != nil {
		t.Fatalf("ListPublic(new-tag) error = %v", err)
	}
	if newTagTotal != 0 {
		t.Fatalf("unpublished tag leaked, total = %d", newTagTotal)
	}

	republished, err := repository.Publish(ctx, created.ProblemID, 4, 101)
	if err != nil {
		t.Fatalf("republish error = %v", err)
	}
	if republished.Revision != 5 {
		t.Fatalf("republished revision = %d, want 5", republished.Revision)
	}
	publicAfterRepublish, err := repository.GetPublic(ctx, created.ProblemID)
	if err != nil {
		t.Fatalf("GetPublic() after republish error = %v", err)
	}
	if publicAfterRepublish.Revision != 5 || publicAfterRepublish.Editable.Title != editedTitle {
		t.Fatalf("republished public problem = %+v", publicAfterRepublish)
	}

	batch, err := repository.BatchGetPublicSummaries(ctx, []int64{created.ProblemID, created.ProblemID + 999, created.ProblemID})
	if err != nil {
		t.Fatalf("BatchGetPublicSummaries() error = %v", err)
	}
	if len(batch) != 3 || batch[0] == nil || batch[1] != nil || batch[2] == nil || batch[0].ProblemID != batch[2].ProblemID {
		t.Fatalf("batch result = %+v", batch)
	}

	// A retry of the same finalized upload is idempotent even after later problem revisions.
	retried, err := repository.FinalizeTestDataUpload(ctx, finalizeInput)
	if err != nil {
		t.Fatalf("idempotent finalize error = %v", err)
	}
	if retried.Revision != finalized.Revision || retried.TestDataVersion != finalized.TestDataVersion {
		t.Fatalf("idempotent result = %+v, original %+v", retried, finalized)
	}

	concurrentTitles := []string{"concurrent-a", "concurrent-b"}
	errs := make([]error, 2)
	results := make([]*ProblemAggregate, 2)
	start := make(chan struct{})
	done := make(chan int, 2)
	for i := range concurrentTitles {
		go func(index int) {
			<-start
			results[index], errs[index] = repository.Update(ctx, UpdateProblemInput{
				ProblemID: created.ProblemID, ExpectedRevision: 5, ActorID: int64(201 + index),
				Patch: ProblemPatch{Title: &concurrentTitles[index]},
			})
			done <- index
		}(i)
	}
	close(start)
	<-done
	<-done
	successes, conflicts := 0, 0
	for i := range errs {
		switch {
		case errs[i] == nil:
			successes++
			if results[i].Revision != 6 {
				t.Fatalf("concurrent success revision = %d", results[i].Revision)
			}
		case errors.Is(errs[i], ErrProblemRevisionConflict):
			conflicts++
		default:
			t.Fatalf("concurrent update error = %v", errs[i])
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent updates successes=%d conflicts=%d", successes, conflicts)
	}

	archived, err := repository.Archive(ctx, created.ProblemID, 6, 101)
	if err != nil {
		t.Fatalf("Archive() error = %v", err)
	}
	if archived.Revision != 7 || archived.Status != ProblemStatusArchived {
		t.Fatalf("archived state = revision %d status %d", archived.Revision, archived.Status)
	}
	if _, err := repository.GetPublic(ctx, created.ProblemID); !errors.Is(err, ErrProblemNotFound) {
		t.Fatalf("GetPublic(archived) error = %v", err)
	}
	republishedAfterArchive, err := repository.Publish(ctx, created.ProblemID, 7, 101)
	if err != nil {
		t.Fatalf("Publish(archived) error = %v", err)
	}
	if republishedAfterArchive.Revision != 8 || republishedAfterArchive.Status != ProblemStatusPublished {
		t.Fatalf("republished archived state = revision %d status %d", republishedAfterArchive.Revision, republishedAfterArchive.Status)
	}

	deletedRevision, err := repository.SoftDelete(ctx, created.ProblemID, 8, 99)
	if err != nil {
		t.Fatalf("SoftDelete() error = %v", err)
	}
	if deletedRevision != 9 {
		t.Fatalf("deleted revision = %d", deletedRevision)
	}
	if _, err := repository.GetPublic(ctx, created.ProblemID); !errors.Is(err, ErrProblemNotFound) {
		t.Fatalf("GetPublic(deleted) error = %v", err)
	}
	restored, err := repository.Restore(ctx, created.ProblemID, 9, 101)
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if restored.Revision != 10 || restored.Status != ProblemStatusPublished {
		t.Fatalf("restored state = revision %d status %d", restored.Revision, restored.Status)
	}
}

func TestFinalizeTestDataUploadRollsBackOnRevisionConflict(t *testing.T) {
	repository, db := newProblemModelTestDatabase(t)
	ctx := context.Background()
	created, err := repository.Create(ctx, CreateProblemInput{AuthorID: 1, Editable: testEditableProblem()})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	upload, err := repository.CreateTestDataUpload(ctx, CreateTestDataUploadInput{
		ProblemID: created.ProblemID, FileName: "tests.zip", SHA256: strings.Repeat("b", 64), TTL: time.Hour,
	})
	if err != nil {
		t.Fatalf("CreateTestDataUpload() error = %v", err)
	}
	_, err = repository.FinalizeTestDataUpload(ctx, FinalizeTestDataUploadInput{
		ProblemID: created.ProblemID, UploadID: upload.UploadID, SHA256: strings.Repeat("b", 64),
		PackageKey: "problem-test-data/immutable/conflict.zip", ExpectedRevision: 99, ActorID: 1,
		TestCases: []TestCaseInput{{Name: "case", Score: 100, Ordinal: 0, Enabled: true}},
	})
	if !errors.Is(err, ErrProblemRevisionConflict) {
		t.Fatalf("FinalizeTestDataUpload() error = %v", err)
	}
	var versionCount, caseCount int64
	if err := db.Model(&TestDataVersions{}).Where("problem_id = ?", created.ProblemID).Count(&versionCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&TestCases{}).Count(&caseCount).Error; err != nil {
		t.Fatal(err)
	}
	var uploadRow testDataUploadRow
	if err := db.Where("upload_id = ?", upload.UploadID).Take(&uploadRow).Error; err != nil {
		t.Fatal(err)
	}
	if versionCount != 0 || caseCount != 0 || uploadRow.Status != TestDataUploadStatusPending {
		t.Fatalf("partial finalize state: versions=%d cases=%d upload_status=%d", versionCount, caseCount, uploadRow.Status)
	}
}

func testEditableProblem() EditableProblem {
	return EditableProblem{
		Title: "Original title", Description: "Description",
		InputDescription: "Input", OutputDescription: "Output",
		Samples: []ProblemSample{{Input: "1", Output: "2"}}, Hint: "Hint",
		Difficulty: 2, Tags: []string{"Graph", "dp"}, Source: "source",
		TimeLimitMS: 1000, MemoryLimitMB: 256,
		AllowedLanguages: []string{"go", "cpp"}, CheckerType: 1,
		Visibility: ProblemVisibilityPublic,
	}
}

func newProblemModelTestDatabase(t *testing.T) (ProblemModel, *gorm.DB) {
	t.Helper()
	dsn := os.Getenv("PROBLEM_MODEL_TEST_DSN")
	if dsn == "" {
		t.Skip("set PROBLEM_MODEL_TEST_DSN to run isolated MySQL integration tests")
	}
	config, err := mysqlDriver.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("parse PROBLEM_MODEL_TEST_DSN: %v", err)
	}
	config.DBName = ""
	config.ParseTime = true
	config.Loc = time.UTC
	admin, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatalf("open MySQL admin connection: %v", err)
	}
	databaseName := fmt.Sprintf("problem_model_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec("CREATE DATABASE `" + databaseName + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"); err != nil {
		admin.Close()
		t.Fatalf("create isolated database: %v", err)
	}
	var testSQLDB *sql.DB
	t.Cleanup(func() {
		if testSQLDB != nil {
			_ = testSQLDB.Close()
		}
		_, _ = admin.Exec("DROP DATABASE IF EXISTS `" + databaseName + "`")
		_ = admin.Close()
	})

	config.DBName = databaseName
	testSQLDB, err = sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatalf("open isolated database: %v", err)
	}
	applyMigrationFile(t, testSQLDB, filepath.Join("..", "migrations", "001_create_problem.sql"))
	applyLegacyForeignKeys(t, testSQLDB)
	applyMigrationFile(t, testSQLDB, filepath.Join("..", "migrations", "003_add_problem_revision_aggregate.sql"))

	db, err := gorm.Open(mysql.New(mysql.Config{Conn: testSQLDB}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open GORM test database: %v", err)
	}
	return NewProblemModel(db), db
}

func applyMigrationFile(t *testing.T, db *sql.DB, fileName string) {
	t.Helper()
	contents, err := os.ReadFile(fileName)
	if err != nil {
		t.Fatalf("read migration %s: %v", fileName, err)
	}
	lines := strings.Split(string(contents), "\n")
	filtered := lines[:0]
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		filtered = append(filtered, line)
	}
	for _, statement := range strings.Split(strings.Join(filtered, "\n"), ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("apply migration %s: %v\nSQL: %s", fileName, err, statement)
		}
	}
}

func applyLegacyForeignKeys(t *testing.T, db *sql.DB) {
	t.Helper()
	statements := []string{
		"ALTER TABLE problem_tags ADD CONSTRAINT fk_problem_tags_problem FOREIGN KEY (problem_id) REFERENCES problems(id) ON DELETE CASCADE",
		"ALTER TABLE test_data_versions ADD CONSTRAINT fk_test_data_versions_problem FOREIGN KEY (problem_id) REFERENCES problems(id) ON DELETE CASCADE",
		"ALTER TABLE test_cases ADD CONSTRAINT fk_test_cases_version FOREIGN KEY (test_data_version_id) REFERENCES test_data_versions(id) ON DELETE CASCADE",
		"ALTER TABLE problem_judge_revisions ADD CONSTRAINT fk_judge_revisions_problem FOREIGN KEY (problem_id) REFERENCES problems(id) ON DELETE CASCADE",
		"ALTER TABLE problem_judge_revisions ADD CONSTRAINT fk_judge_revisions_test_data_version FOREIGN KEY (test_data_version_id) REFERENCES test_data_versions(id)",
		"ALTER TABLE test_data_uploads ADD CONSTRAINT fk_test_data_uploads_problem FOREIGN KEY (problem_id) REFERENCES problems(id) ON DELETE CASCADE",
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("apply legacy foreign key: %v\nSQL: %s", err, statement)
		}
	}
}
