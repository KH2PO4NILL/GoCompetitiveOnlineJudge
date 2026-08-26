CREATE TABLE IF NOT EXISTS problems (
    id BIGINT NOT NULL AUTO_INCREMENT,
    author_id BIGINT NOT NULL,

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

    current_test_data_version_id BIGINT NULL,
    latest_published_revision BIGINT NULL,
    status TINYINT UNSIGNED NOT NULL DEFAULT 1,
    visibility TINYINT UNSIGNED NOT NULL DEFAULT 1,
    revision BIGINT NOT NULL DEFAULT 1,
    published_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
        ON UPDATE CURRENT_TIMESTAMP(6),
    deleted_at DATETIME(6) NULL,
    deleted_from_status TINYINT UNSIGNED NULL,

    PRIMARY KEY (id),
    KEY idx_problems_public_list (status, visibility, updated_at, id),
    KEY idx_problems_author (author_id, status, updated_at),
    KEY idx_problems_difficulty (difficulty, status),
    KEY idx_problems_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS problem_tags (
    id BIGINT NOT NULL AUTO_INCREMENT,
    problem_id BIGINT NOT NULL,
    tag VARCHAR(64) NOT NULL,

    PRIMARY KEY (id),
    UNIQUE KEY uk_problem_tags_problem_tag (problem_id, tag),
    KEY idx_problem_tags_tag (tag, problem_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS test_data_versions (
    id BIGINT NOT NULL AUTO_INCREMENT,
    problem_id BIGINT NOT NULL,
    version VARCHAR(64) NOT NULL,
    package_key VARCHAR(512) NOT NULL,
    checksum CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id),
    UNIQUE KEY uk_test_data_versions_problem_version (problem_id, version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS test_cases (
    id BIGINT NOT NULL AUTO_INCREMENT,
    test_data_version_id BIGINT NOT NULL,
    name VARCHAR(128) NOT NULL,
    score INT NOT NULL DEFAULT 0,
    ordinal INT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    group_name VARCHAR(128) NOT NULL DEFAULT '',

    PRIMARY KEY (id),
    UNIQUE KEY uk_test_cases_version_ordinal (test_data_version_id, ordinal)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS problem_judge_revisions (
    id BIGINT NOT NULL AUTO_INCREMENT,
    problem_id BIGINT NOT NULL,
    revision BIGINT NOT NULL,
    time_limit_ms INT UNSIGNED NOT NULL,
    memory_limit_mb INT UNSIGNED NOT NULL,
    allowed_languages_json JSON NOT NULL,
    checker_type TINYINT UNSIGNED NOT NULL,
    checker_file_key VARCHAR(512) NULL,
    interactor_file_key VARCHAR(512) NULL,
    test_data_version_id BIGINT NOT NULL,
    published_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id),
    UNIQUE KEY uk_judge_revisions_problem_revision (problem_id, revision),
    KEY idx_judge_revisions_test_data_version (test_data_version_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS test_data_uploads (
    upload_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    problem_id BIGINT NOT NULL,
    object_key VARCHAR(512) NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    content_length BIGINT NOT NULL,
    checksum CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    status TINYINT UNSIGNED NOT NULL DEFAULT 1,
    expires_at DATETIME(6) NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    finalized_at DATETIME(6) NULL,

    PRIMARY KEY (upload_id),
    UNIQUE KEY uk_test_data_uploads_object_key (object_key),
    KEY idx_test_data_uploads_problem (problem_id, created_at),
    KEY idx_test_data_uploads_expiry (status, expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
