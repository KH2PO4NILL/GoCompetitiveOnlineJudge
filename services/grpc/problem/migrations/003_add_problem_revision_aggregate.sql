-- Complete immutable problem snapshots and revision-scoped tags.
-- Existing installations can only approximate pre-migration statement/tag history;
-- all writes after this migration are lossless and revisioned by ProblemModel.

CREATE TABLE problem_revisions (
    id BIGINT NOT NULL AUTO_INCREMENT,
    problem_id BIGINT NOT NULL,
    revision BIGINT NOT NULL,

    title VARCHAR(200) NOT NULL,
    description LONGTEXT NOT NULL,
    input_description LONGTEXT NOT NULL,
    output_description LONGTEXT NOT NULL,
    samples_json JSON NOT NULL,
    hint LONGTEXT NOT NULL,

    difficulty TINYINT UNSIGNED NOT NULL,
    source VARCHAR(255) NOT NULL DEFAULT '',
    time_limit_ms INT UNSIGNED NOT NULL,
    memory_limit_mb INT UNSIGNED NOT NULL,
    allowed_languages_json JSON NOT NULL,
    checker_type TINYINT UNSIGNED NOT NULL,
    checker_file_key VARCHAR(512) NULL,
    interactor_file_key VARCHAR(512) NULL,
    test_data_version_id BIGINT NULL,

    status TINYINT UNSIGNED NOT NULL,
    visibility TINYINT UNSIGNED NOT NULL,
    created_by BIGINT NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id),
    UNIQUE KEY uk_problem_revisions_problem_revision (problem_id, revision),
    KEY idx_problem_revisions_test_data_version (test_data_version_id),
    CONSTRAINT chk_problem_revisions_revision CHECK (revision > 0),
    CONSTRAINT chk_problem_revisions_difficulty CHECK (difficulty BETWEEN 1 AND 5),
    CONSTRAINT chk_problem_revisions_limits CHECK (time_limit_ms > 0 AND memory_limit_mb > 0),
    CONSTRAINT chk_problem_revisions_checker CHECK (checker_type BETWEEN 1 AND 3),
    CONSTRAINT chk_problem_revisions_status CHECK (status BETWEEN 1 AND 4),
    CONSTRAINT chk_problem_revisions_visibility CHECK (visibility BETWEEN 1 AND 3),
    CONSTRAINT fk_problem_revisions_problem
        FOREIGN KEY (problem_id) REFERENCES problems(id) ON DELETE CASCADE,
    CONSTRAINT fk_problem_revisions_test_data_version
        FOREIGN KEY (test_data_version_id) REFERENCES test_data_versions(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE problem_tag_definitions (
    id BIGINT NOT NULL AUTO_INCREMENT,
    name VARCHAR(64) NOT NULL,
    normalized_name VARCHAR(64) NOT NULL,

    PRIMARY KEY (id),
    UNIQUE KEY uk_problem_tag_definitions_normalized_name (normalized_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE problem_revision_tags (
    problem_revision_id BIGINT NOT NULL,
    tag_definition_id BIGINT NOT NULL,

    PRIMARY KEY (problem_revision_id, tag_definition_id),
    KEY idx_problem_revision_tags_tag (tag_definition_id, problem_revision_id),
    CONSTRAINT fk_problem_revision_tags_revision
        FOREIGN KEY (problem_revision_id) REFERENCES problem_revisions(id) ON DELETE CASCADE,
    CONSTRAINT fk_problem_revision_tags_definition
        FOREIGN KEY (tag_definition_id) REFERENCES problem_tag_definitions(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

ALTER TABLE test_data_uploads
    ADD COLUMN finalized_test_data_version_id BIGINT NULL AFTER finalized_at,
    ADD COLUMN finalized_revision BIGINT NULL AFTER finalized_test_data_version_id,
    ADD KEY idx_test_data_uploads_finalized_version (finalized_test_data_version_id),
    ADD CONSTRAINT fk_test_data_uploads_finalized_version
        FOREIGN KEY (finalized_test_data_version_id) REFERENCES test_data_versions(id),
    ADD CONSTRAINT chk_test_data_uploads_content_length CHECK (content_length >= 0),
    ADD CONSTRAINT chk_test_data_uploads_status CHECK (status BETWEEN 1 AND 4);

UPDATE problems
SET deleted_at = COALESCE(deleted_at, updated_at),
    deleted_from_status = CASE
        WHEN deleted_from_status BETWEEN 1 AND 3 THEN deleted_from_status
        ELSE 1
    END
WHERE status = 4;

UPDATE problems
SET deleted_at = NULL,
    deleted_from_status = NULL
WHERE status <> 4;

ALTER TABLE problems
    ADD CONSTRAINT chk_problems_revision CHECK (revision > 0),
    ADD CONSTRAINT chk_problems_published_revision CHECK (
        latest_published_revision IS NULL
        OR (latest_published_revision > 0 AND latest_published_revision <= revision)
    ),
    ADD CONSTRAINT chk_problems_status CHECK (status BETWEEN 1 AND 4),
    ADD CONSTRAINT chk_problems_visibility CHECK (visibility BETWEEN 1 AND 3),
    ADD CONSTRAINT chk_problems_difficulty CHECK (difficulty BETWEEN 1 AND 5),
    ADD CONSTRAINT chk_problems_limits CHECK (time_limit_ms > 0 AND memory_limit_mb > 0),
    ADD CONSTRAINT chk_problems_checker CHECK (checker_type BETWEEN 1 AND 3),
    ADD CONSTRAINT chk_problems_deleted_state CHECK (
        (status = 4 AND deleted_at IS NOT NULL AND deleted_from_status BETWEEN 1 AND 3)
        OR (status <> 4 AND deleted_at IS NULL AND deleted_from_status IS NULL)
    ),
    ADD CONSTRAINT fk_problems_current_test_data_version
        FOREIGN KEY (current_test_data_version_id) REFERENCES test_data_versions(id);

ALTER TABLE test_cases
    ADD CONSTRAINT chk_test_cases_score CHECK (score >= 0),
    ADD CONSTRAINT chk_test_cases_ordinal CHECK (ordinal >= 0);

-- Recover every legacy judge revision that still exists. Statement and tag data
-- were not historically versioned, so the current values are the only possible
-- backfill for those old rows.
INSERT IGNORE INTO problem_revisions (
    problem_id, revision,
    title, description, input_description, output_description, samples_json, hint,
    difficulty, source, time_limit_ms, memory_limit_mb, allowed_languages_json,
    checker_type, checker_file_key, interactor_file_key, test_data_version_id,
    status, visibility, created_by, created_at
)
SELECT
    p.id, jr.revision,
    p.title, p.description, p.input_description, p.output_description, p.samples_json, p.hint,
    p.difficulty, p.source, jr.time_limit_ms, jr.memory_limit_mb, jr.allowed_languages_json,
    jr.checker_type, jr.checker_file_key, jr.interactor_file_key, jr.test_data_version_id,
    2, p.visibility, p.author_id, jr.published_at
FROM problem_judge_revisions AS jr
JOIN problems AS p ON p.id = jr.problem_id;

INSERT IGNORE INTO problem_revisions (
    problem_id, revision,
    title, description, input_description, output_description, samples_json, hint,
    difficulty, source, time_limit_ms, memory_limit_mb, allowed_languages_json,
    checker_type, checker_file_key, interactor_file_key, test_data_version_id,
    status, visibility, created_by, created_at
)
SELECT
    id, revision,
    title, description, input_description, output_description, samples_json, hint,
    difficulty, source, time_limit_ms, memory_limit_mb, allowed_languages_json,
    checker_type, checker_file_key, interactor_file_key, current_test_data_version_id,
    status, visibility, author_id, updated_at
FROM problems;

INSERT IGNORE INTO problem_tag_definitions (name, normalized_name)
SELECT MIN(TRIM(tag)), LOWER(TRIM(tag))
FROM problem_tags
WHERE TRIM(tag) <> ''
GROUP BY LOWER(TRIM(tag));

INSERT IGNORE INTO problem_revision_tags (problem_revision_id, tag_definition_id)
SELECT pr.id, td.id
FROM problem_revisions AS pr
JOIN problem_tags AS legacy_tag ON legacy_tag.problem_id = pr.problem_id
JOIN problem_tag_definitions AS td
  ON td.normalized_name = LOWER(TRIM(legacy_tag.tag));

-- Recover idempotency metadata for uploads finalized before this migration.
UPDATE test_data_uploads AS upload
JOIN test_data_versions AS version
  ON version.problem_id = upload.problem_id
 AND version.package_key = upload.object_key
JOIN problems AS problem ON problem.id = upload.problem_id
SET upload.finalized_test_data_version_id = version.id,
    upload.finalized_revision = COALESCE(
        (SELECT MAX(revision)
         FROM problem_revisions
         WHERE problem_id = upload.problem_id
           AND test_data_version_id = version.id),
        problem.revision
    ),
    upload.finalized_at = COALESCE(upload.finalized_at, upload.created_at)
WHERE upload.status = 2;

-- A legacy finalized row without a matching version cannot be replayed safely.
-- Mark it failed instead of retaining an invalid finalized state.
UPDATE test_data_uploads
SET status = 4, finalized_at = NULL
WHERE status = 2
  AND (finalized_test_data_version_id IS NULL OR finalized_revision IS NULL);

UPDATE test_data_uploads
SET finalized_at = NULL,
    finalized_test_data_version_id = NULL,
    finalized_revision = NULL
WHERE status <> 2;

ALTER TABLE test_data_uploads
    ADD CONSTRAINT chk_test_data_uploads_finalized_result CHECK (
        (status = 2 AND finalized_at IS NOT NULL
            AND finalized_test_data_version_id IS NOT NULL AND finalized_revision IS NOT NULL)
        OR
        (status <> 2 AND finalized_at IS NULL
            AND finalized_test_data_version_id IS NULL AND finalized_revision IS NULL)
    );

ALTER TABLE problems
    ADD KEY idx_problems_latest_published_revision (id, latest_published_revision),
    ADD CONSTRAINT fk_problems_latest_published_revision
        FOREIGN KEY (id, latest_published_revision)
        REFERENCES problem_revisions(problem_id, revision);
