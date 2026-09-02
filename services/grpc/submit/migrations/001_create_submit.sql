CREATE TABLE IF NOT EXISTS submissions (
    id BIGINT NOT NULL AUTO_INCREMENT,
    user_id BIGINT NOT NULL,
    problem_id BIGINT NOT NULL,
    contest_id BIGINT NULL,
    problem_revision BIGINT NOT NULL,
    language VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    source_code LONGTEXT NOT NULL,
    client_request_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    request_fingerprint BINARY(32) NOT NULL,
    status TINYINT UNSIGNED NOT NULL,
    current_attempt_id BIGINT NULL,
    attempt_count INT UNSIGNED NOT NULL DEFAULT 1,
    verdict TINYINT UNSIGNED NULL,
    score BIGINT NULL,
    max_score BIGINT NULL,
    judging_started_at DATETIME(6) NULL,
    finished_at DATETIME(6) NULL,
    submitted_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    version BIGINT UNSIGNED NOT NULL DEFAULT 1,

    PRIMARY KEY (id),
    UNIQUE KEY uk_submissions_user_request (user_id, client_request_id),
    UNIQUE KEY uk_submissions_current_attempt_owner (id, current_attempt_id),
    KEY idx_submissions_user_list (user_id, submitted_at DESC, id DESC),
    KEY idx_submissions_problem_list (problem_id, submitted_at DESC, id DESC),
    KEY idx_submissions_contest_list (contest_id, submitted_at DESC, id DESC),
    KEY idx_submissions_admin_list (status, verdict, submitted_at DESC, id DESC),
    KEY idx_submissions_current_attempt (current_attempt_id),
    CONSTRAINT chk_submissions_ids CHECK (
        user_id > 0 AND problem_id > 0 AND (contest_id IS NULL OR contest_id > 0)
    ),
    CONSTRAINT chk_submissions_problem_revision CHECK (problem_revision > 0),
    CONSTRAINT chk_submissions_language CHECK (CHAR_LENGTH(language) > 0),
    CONSTRAINT chk_submissions_client_request CHECK (CHAR_LENGTH(client_request_id) > 0),
    CONSTRAINT chk_submissions_status CHECK (status IN (1, 2, 3)),
    CONSTRAINT chk_submissions_attempt_count CHECK (attempt_count > 0),
    CONSTRAINT chk_submissions_version CHECK (version > 0),
    CONSTRAINT chk_submissions_result_state CHECK (
        (
            status = 3
            AND verdict IN (1, 2, 3, 4, 5, 6, 7, 8, 9)
            AND score IS NOT NULL AND score >= 0
            AND max_score IS NOT NULL AND max_score >= 0
            AND score <= max_score
            AND judging_started_at IS NOT NULL
            AND finished_at IS NOT NULL
        )
        OR
        (
            status IN (1, 2)
            AND verdict IS NULL
            AND score IS NULL
            AND max_score IS NULL
            AND finished_at IS NULL
        )
    ),
    CONSTRAINT chk_submissions_judging_state CHECK (
        (status = 1 AND judging_started_at IS NULL)
        OR (status IN (2, 3) AND judging_started_at IS NOT NULL)
    )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS submission_attempts (
    id BIGINT NOT NULL AUTO_INCREMENT,
    submission_id BIGINT NOT NULL,
    attempt_no INT UNSIGNED NOT NULL,
    kind TINYINT UNSIGNED NOT NULL,
    status TINYINT UNSIGNED NOT NULL,
    requested_by BIGINT NULL,
    reason VARCHAR(500) NOT NULL DEFAULT '',
    worker_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
    delivery_id VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
    lease_token_hash BINARY(32) NULL,
    lease_expires_at DATETIME(6) NULL,
    last_heartbeat_at DATETIME(6) NULL,
    completed_test_cases INT UNSIGNED NOT NULL DEFAULT 0,
    report_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NULL,
    report_fingerprint BINARY(32) NULL,
    verdict TINYINT UNSIGNED NULL,
    score BIGINT NULL,
    max_score BIGINT NULL,
    compile_output LONGTEXT NOT NULL,
    enqueued_at DATETIME(6) NOT NULL,
    started_at DATETIME(6) NULL,
    finished_at DATETIME(6) NULL,
    superseded_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,

    PRIMARY KEY (id),
    UNIQUE KEY uk_submission_attempt_number (submission_id, attempt_no),
    UNIQUE KEY uk_submission_attempt_owner (submission_id, id),
    KEY idx_submission_attempt_expired_lease (status, lease_expires_at, id),
    CONSTRAINT fk_submission_attempt_submission
        FOREIGN KEY (submission_id) REFERENCES submissions(id) ON DELETE RESTRICT,
    CONSTRAINT chk_submission_attempt_number CHECK (attempt_no > 0),
    CONSTRAINT chk_submission_attempt_kind CHECK (kind IN (1, 2, 3)),
    CONSTRAINT chk_submission_attempt_status CHECK (status IN (1, 2, 3, 4)),
    CONSTRAINT chk_submission_attempt_requester CHECK (requested_by IS NULL OR requested_by > 0),
    CONSTRAINT chk_submission_attempt_completed CHECK (completed_test_cases >= 0),
    CONSTRAINT chk_submission_attempt_state CHECK (
        (
            status = 1
            AND worker_id = ''
            AND lease_token_hash IS NULL
            AND lease_expires_at IS NULL
            AND started_at IS NULL
            AND report_id IS NULL
            AND report_fingerprint IS NULL
            AND verdict IS NULL
            AND score IS NULL
            AND max_score IS NULL
            AND finished_at IS NULL
            AND superseded_at IS NULL
        )
        OR
        (
            status = 2
            AND worker_id <> ''
            AND lease_token_hash IS NOT NULL
            AND lease_expires_at IS NOT NULL
            AND started_at IS NOT NULL
            AND report_id IS NULL
            AND report_fingerprint IS NULL
            AND verdict IS NULL
            AND score IS NULL
            AND max_score IS NULL
            AND finished_at IS NULL
            AND superseded_at IS NULL
        )
        OR
        (
            status = 3
            AND worker_id <> ''
            AND lease_token_hash IS NOT NULL
            AND lease_expires_at IS NOT NULL
            AND started_at IS NOT NULL
            AND report_id IS NOT NULL
            AND report_fingerprint IS NOT NULL
            AND verdict IN (1, 2, 3, 4, 5, 6, 7, 8, 9)
            AND score IS NOT NULL AND score >= 0
            AND max_score IS NOT NULL AND max_score >= 0
            AND score <= max_score
            AND finished_at IS NOT NULL
            AND superseded_at IS NULL
        )
        OR
        (
            status = 4
            AND superseded_at IS NOT NULL
        )
    )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS submission_case_results (
    id BIGINT NOT NULL AUTO_INCREMENT,
    attempt_id BIGINT NOT NULL,
    test_case_id BIGINT NOT NULL,
    ordinal INT UNSIGNED NOT NULL,
    group_name VARCHAR(128) NOT NULL DEFAULT '',
    verdict TINYINT UNSIGNED NOT NULL,
    score BIGINT NOT NULL,
    time_used_ms BIGINT NOT NULL,
    memory_used_kb BIGINT NOT NULL,
    judge_message LONGTEXT NOT NULL,
    created_at DATETIME(6) NOT NULL,

    PRIMARY KEY (id),
    UNIQUE KEY uk_submission_case_attempt_case (attempt_id, test_case_id),
    UNIQUE KEY uk_submission_case_attempt_ordinal (attempt_id, ordinal),
    CONSTRAINT fk_submission_case_attempt
        FOREIGN KEY (attempt_id) REFERENCES submission_attempts(id) ON DELETE RESTRICT,
    CONSTRAINT chk_submission_case_ids CHECK (test_case_id > 0),
    CONSTRAINT chk_submission_case_ordinal CHECK (ordinal >= 0),
    CONSTRAINT chk_submission_case_verdict CHECK (verdict IN (1, 2, 3, 4, 5, 6, 7, 8, 9)),
    CONSTRAINT chk_submission_case_resources CHECK (
        score >= 0 AND time_used_ms >= 0 AND memory_used_kb >= 0
    )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS submit_outbox_events (
    id BIGINT NOT NULL AUTO_INCREMENT,
    event_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    event_type TINYINT UNSIGNED NOT NULL,
    aggregate_id BIGINT NOT NULL,
    attempt_id BIGINT NOT NULL,
    event_key VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    payload JSON NOT NULL,
    status TINYINT UNSIGNED NOT NULL DEFAULT 1,
    available_at DATETIME(6) NOT NULL,
    publish_attempts INT UNSIGNED NOT NULL DEFAULT 0,
    locked_by VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NULL,
    locked_until DATETIME(6) NULL,
    last_error VARCHAR(1000) NOT NULL DEFAULT '',
    created_at DATETIME(6) NOT NULL,
    published_at DATETIME(6) NULL,

    PRIMARY KEY (id),
    UNIQUE KEY uk_submit_outbox_event_id (event_id),
    UNIQUE KEY uk_submit_outbox_attempt_type (attempt_id, event_type),
    KEY idx_submit_outbox_pending (status, available_at, id),
    KEY idx_submit_outbox_lock (status, locked_until, id),
    CONSTRAINT fk_submit_outbox_attempt
        FOREIGN KEY (aggregate_id, attempt_id)
        REFERENCES submission_attempts(submission_id, id)
        ON DELETE RESTRICT,
    CONSTRAINT chk_submit_outbox_event_type CHECK (event_type IN (1, 2)),
    CONSTRAINT chk_submit_outbox_status CHECK (status IN (1, 2, 3)),
    CONSTRAINT chk_submit_outbox_attempts CHECK (publish_attempts >= 0),
    CONSTRAINT chk_submit_outbox_lock CHECK (
        (locked_by IS NULL AND locked_until IS NULL)
        OR (locked_by IS NOT NULL AND locked_until IS NOT NULL)
    ),
    CONSTRAINT chk_submit_outbox_publish_state CHECK (
        (status = 2 AND published_at IS NOT NULL AND locked_by IS NULL AND locked_until IS NULL)
        OR (status IN (1, 3) AND published_at IS NULL)
    )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
