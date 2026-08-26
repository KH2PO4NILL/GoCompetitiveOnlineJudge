-- 001 was initially applied with composite primary keys, then changed to use
-- surrogate IDs so goctl can generate models. Keep existing databases aligned
-- and restore referential constraints without making 001 unparsable by goctl.

DROP PROCEDURE IF EXISTS migrate_problem_schema_002;

DELIMITER //

CREATE PROCEDURE migrate_problem_schema_002()
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = DATABASE()
          AND table_name = 'problem_tags'
          AND column_name = 'id'
    ) THEN
        ALTER TABLE problem_tags
            DROP PRIMARY KEY,
            ADD COLUMN id BIGINT NOT NULL AUTO_INCREMENT FIRST,
            ADD PRIMARY KEY (id),
            ADD UNIQUE KEY uk_problem_tags_problem_tag (problem_id, tag);
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = DATABASE()
          AND table_name = 'problem_judge_revisions'
          AND column_name = 'id'
    ) THEN
        ALTER TABLE problem_judge_revisions
            DROP PRIMARY KEY,
            ADD COLUMN id BIGINT NOT NULL AUTO_INCREMENT FIRST,
            ADD PRIMARY KEY (id),
            ADD UNIQUE KEY uk_judge_revisions_problem_revision (problem_id, revision);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE constraint_schema = DATABASE()
          AND table_name = 'problem_tags'
          AND constraint_name = 'fk_problem_tags_problem'
    ) THEN
        ALTER TABLE problem_tags
            ADD CONSTRAINT fk_problem_tags_problem
            FOREIGN KEY (problem_id) REFERENCES problems(id) ON DELETE CASCADE;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE constraint_schema = DATABASE()
          AND table_name = 'test_data_versions'
          AND constraint_name = 'fk_test_data_versions_problem'
    ) THEN
        ALTER TABLE test_data_versions
            ADD CONSTRAINT fk_test_data_versions_problem
            FOREIGN KEY (problem_id) REFERENCES problems(id) ON DELETE CASCADE;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE constraint_schema = DATABASE()
          AND table_name = 'test_cases'
          AND constraint_name = 'fk_test_cases_version'
    ) THEN
        ALTER TABLE test_cases
            ADD CONSTRAINT fk_test_cases_version
            FOREIGN KEY (test_data_version_id) REFERENCES test_data_versions(id) ON DELETE CASCADE;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE constraint_schema = DATABASE()
          AND table_name = 'problem_judge_revisions'
          AND constraint_name = 'fk_judge_revisions_problem'
    ) THEN
        ALTER TABLE problem_judge_revisions
            ADD CONSTRAINT fk_judge_revisions_problem
            FOREIGN KEY (problem_id) REFERENCES problems(id) ON DELETE CASCADE;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE constraint_schema = DATABASE()
          AND table_name = 'problem_judge_revisions'
          AND constraint_name = 'fk_judge_revisions_test_data_version'
    ) THEN
        ALTER TABLE problem_judge_revisions
            ADD CONSTRAINT fk_judge_revisions_test_data_version
            FOREIGN KEY (test_data_version_id) REFERENCES test_data_versions(id);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE constraint_schema = DATABASE()
          AND table_name = 'test_data_uploads'
          AND constraint_name = 'fk_test_data_uploads_problem'
    ) THEN
        ALTER TABLE test_data_uploads
            ADD CONSTRAINT fk_test_data_uploads_problem
            FOREIGN KEY (problem_id) REFERENCES problems(id) ON DELETE CASCADE;
    END IF;
END//

DELIMITER ;

CALL migrate_problem_schema_002();
DROP PROCEDURE migrate_problem_schema_002;
