CREATE TABLE IF NOT EXISTS contests (
    id BIGINT NOT NULL AUTO_INCREMENT,
    creator_id BIGINT NOT NULL,
    title VARCHAR(200) NOT NULL,
    description LONGTEXT NOT NULL,
    mode TINYINT UNSIGNED NOT NULL,
    start_at DATETIME(6) NOT NULL,
    end_at DATETIME(6) NOT NULL,
    registration_start_at DATETIME(6) NULL,
    registration_end_at DATETIME(6) NULL,
    registration_policy TINYINT UNSIGNED NOT NULL,
    visibility TINYINT UNSIGNED NOT NULL,
    scoreboard_visibility TINYINT UNSIGNED NOT NULL,
    scoreboard_freeze_at DATETIME(6) NULL,
    scoreboard_freeze_before_end_ms BIGINT UNSIGNED NULL,
    invite_code_hash VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NULL,
    status TINYINT UNSIGNED NOT NULL DEFAULT 1,
    revision BIGINT NOT NULL DEFAULT 1,
    participant_count BIGINT NOT NULL DEFAULT 0,
    cancel_reason VARCHAR(500) NOT NULL DEFAULT '',
    published_at DATETIME(6) NULL,
    cancelled_at DATETIME(6) NULL,
    archived_at DATETIME(6) NULL,
    deleted_at DATETIME(6) NULL,
    deleted_from_status TINYINT UNSIGNED NULL,
    scoreboard_frozen BOOLEAN NOT NULL DEFAULT FALSE,
    scoreboard_frozen_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id),
    KEY idx_contests_public_list (visibility, status, start_at, id),
    KEY idx_contests_creator (creator_id, status, updated_at, id),
    KEY idx_contests_auto_freeze (status, scoreboard_frozen, scoreboard_freeze_at, id),
    KEY idx_contests_auto_freeze_before_end (status, scoreboard_frozen, end_at, id),
    KEY idx_contests_deleted_at (deleted_at),
    CONSTRAINT chk_contests_mode CHECK (mode IN (1, 2, 3)),
    CONSTRAINT chk_contests_status CHECK (status IN (1, 2, 5, 6, 7)),
    CONSTRAINT chk_contests_registration_policy CHECK (registration_policy IN (1, 2, 3)),
    CONSTRAINT chk_contests_visibility CHECK (visibility IN (1, 2, 3)),
    CONSTRAINT chk_contests_scoreboard_visibility CHECK (scoreboard_visibility IN (1, 2, 3)),
    CONSTRAINT chk_contests_revision CHECK (revision > 0),
    CONSTRAINT chk_contests_participant_count CHECK (participant_count >= 0),
    CONSTRAINT chk_contests_time_window CHECK (start_at < end_at),
    CONSTRAINT chk_contests_registration_window CHECK (
        (registration_start_at IS NULL OR registration_start_at < start_at)
        AND (registration_end_at IS NULL OR registration_end_at <= start_at)
        AND (registration_start_at IS NULL OR registration_end_at IS NULL OR registration_start_at < registration_end_at)
    ),
    CONSTRAINT chk_contests_freeze_policy CHECK (
        NOT (scoreboard_freeze_at IS NOT NULL AND scoreboard_freeze_before_end_ms IS NOT NULL)
        AND (mode = 1 OR (scoreboard_freeze_at IS NULL AND scoreboard_freeze_before_end_ms IS NULL))
        AND (scoreboard_freeze_at IS NULL OR (scoreboard_freeze_at > start_at AND scoreboard_freeze_at < end_at))
        AND (scoreboard_freeze_before_end_ms IS NULL OR scoreboard_freeze_before_end_ms > 0)
    ),
    CONSTRAINT chk_contests_deleted_state CHECK (
        (status = 7 AND deleted_at IS NOT NULL AND deleted_from_status IN (1, 2, 5, 6))
        OR (status <> 7 AND deleted_at IS NULL AND deleted_from_status IS NULL)
    ),
    CONSTRAINT chk_contests_frozen_state CHECK (
        (scoreboard_frozen = TRUE AND scoreboard_frozen_at IS NOT NULL)
        OR (scoreboard_frozen = FALSE AND scoreboard_frozen_at IS NULL)
    )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS contest_problems (
    contest_id BIGINT NOT NULL,
    problem_id BIGINT NOT NULL,
    label VARCHAR(32) NOT NULL,
    display_order INT NOT NULL,
    max_score BIGINT NOT NULL DEFAULT 0,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    PRIMARY KEY (contest_id, problem_id),
    UNIQUE KEY uk_contest_problems_label (contest_id, label),
    UNIQUE KEY uk_contest_problems_order (contest_id, display_order),
    KEY idx_contest_problems_problem (problem_id, contest_id),
    CONSTRAINT fk_contest_problems_contest FOREIGN KEY (contest_id) REFERENCES contests(id) ON DELETE CASCADE,
    CONSTRAINT chk_contest_problems_order CHECK (display_order >= 0),
    CONSTRAINT chk_contest_problems_score CHECK (max_score >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS contest_registrations (
    id BIGINT NOT NULL AUTO_INCREMENT,
    contest_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    status TINYINT UNSIGNED NOT NULL,
    application_note VARCHAR(2000) NOT NULL DEFAULT '',
    reviewer_id BIGINT NULL,
    review_note VARCHAR(2000) NOT NULL DEFAULT '',
    revision BIGINT NOT NULL DEFAULT 1,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    reviewed_at DATETIME(6) NULL,
    cancelled_at DATETIME(6) NULL,

    PRIMARY KEY (id),
    UNIQUE KEY uk_contest_registrations_user (contest_id, user_id),
    KEY idx_contest_registrations_list (contest_id, status, created_at, id),
    CONSTRAINT fk_contest_registrations_contest FOREIGN KEY (contest_id) REFERENCES contests(id) ON DELETE CASCADE,
    CONSTRAINT chk_contest_registrations_status CHECK (status IN (1, 2, 3, 4)),
    CONSTRAINT chk_contest_registrations_revision CHECK (revision > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS contest_participants (
    contest_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    registration_id BIGINT NOT NULL,
    status TINYINT UNSIGNED NOT NULL DEFAULT 1,
    qualified_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    PRIMARY KEY (contest_id, user_id),
    UNIQUE KEY uk_contest_participants_registration (registration_id),
    KEY idx_contest_participants_list (contest_id, status, qualified_at, user_id),
    CONSTRAINT fk_contest_participants_contest FOREIGN KEY (contest_id) REFERENCES contests(id) ON DELETE CASCADE,
    CONSTRAINT fk_contest_participants_registration FOREIGN KEY (registration_id) REFERENCES contest_registrations(id) ON DELETE RESTRICT,
    CONSTRAINT chk_contest_participants_status CHECK (status IN (1, 2, 3))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS contest_scoreboard_state (
    contest_id BIGINT NOT NULL,
    mode TINYINT UNSIGNED NOT NULL,
    source_version BIGINT UNSIGNED NOT NULL DEFAULT 0,
    generated_at DATETIME(6) NULL,
    active_frozen_snapshot_id BIGINT NULL,
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    PRIMARY KEY (contest_id),
    CONSTRAINT fk_contest_scoreboard_state_contest FOREIGN KEY (contest_id) REFERENCES contests(id) ON DELETE CASCADE,
    CONSTRAINT chk_contest_scoreboard_state_mode CHECK (mode IN (1, 2, 3))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS contest_scoreboard_rows (
    contest_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    display_name VARCHAR(200) NOT NULL,
    mode TINYINT UNSIGNED NOT NULL,
    rank_value BIGINT NOT NULL,
    ordinal BIGINT NOT NULL,
    solved_count INT NULL,
    total_penalty_ms BIGINT NULL,
    last_solved_at DATETIME(6) NULL,
    total_score BIGINT NULL,
    last_effective_submission_at DATETIME(6) NULL,

    PRIMARY KEY (contest_id, user_id),
    UNIQUE KEY uk_contest_scoreboard_rows_ordinal (contest_id, ordinal),
    KEY idx_contest_scoreboard_rows_rank (contest_id, rank_value, ordinal),
    CONSTRAINT fk_contest_scoreboard_rows_contest FOREIGN KEY (contest_id) REFERENCES contests(id) ON DELETE CASCADE,
    CONSTRAINT chk_contest_scoreboard_rows_mode CHECK (mode IN (1, 2, 3)),
    CONSTRAINT chk_contest_scoreboard_rows_rank CHECK (rank_value > 0 AND ordinal > 0),
    CONSTRAINT chk_contest_scoreboard_rows_payload CHECK (
        (mode = 1 AND solved_count IS NOT NULL AND solved_count >= 0
            AND total_penalty_ms IS NOT NULL AND total_penalty_ms >= 0 AND total_score IS NULL)
        OR (mode IN (2, 3) AND solved_count IS NULL AND total_penalty_ms IS NULL
            AND total_score IS NOT NULL AND total_score >= 0)
    )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS contest_scoreboard_problem_results (
    contest_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    problem_id BIGINT NOT NULL,
    label VARCHAR(32) NOT NULL,
    mode TINYINT UNSIGNED NOT NULL,
    solved BOOLEAN NULL,
    wrong_attempts INT NULL,
    penalty_ms BIGINT NULL,
    accepted_at DATETIME(6) NULL,
    first_to_solve BOOLEAN NULL,
    score BIGINT NULL,
    max_score BIGINT NULL,
    submission_count INT NULL,
    last_effective_submission_at DATETIME(6) NULL,

    PRIMARY KEY (contest_id, user_id, problem_id),
    KEY idx_contest_scoreboard_results_problem (contest_id, problem_id, user_id),
    CONSTRAINT fk_contest_scoreboard_results_row FOREIGN KEY (contest_id, user_id)
        REFERENCES contest_scoreboard_rows(contest_id, user_id) ON DELETE CASCADE,
    CONSTRAINT chk_contest_scoreboard_results_mode CHECK (mode IN (1, 2, 3)),
    CONSTRAINT chk_contest_scoreboard_results_payload CHECK (
        (mode = 1 AND solved IS NOT NULL AND wrong_attempts IS NOT NULL AND penalty_ms IS NOT NULL
            AND wrong_attempts >= 0 AND penalty_ms >= 0 AND first_to_solve IS NOT NULL
            AND score IS NULL AND max_score IS NULL AND submission_count IS NULL)
        OR (mode IN (2, 3) AND solved IS NULL AND wrong_attempts IS NULL AND penalty_ms IS NULL
            AND first_to_solve IS NULL AND score IS NOT NULL AND score >= 0
            AND max_score IS NOT NULL AND max_score > 0 AND score <= max_score
            AND submission_count IS NOT NULL AND submission_count >= 0)
    )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS contest_scoreboard_snapshots (
    id BIGINT NOT NULL AUTO_INCREMENT,
    contest_id BIGINT NOT NULL,
    source_version BIGINT UNSIGNED NOT NULL,
    mode TINYINT UNSIGNED NOT NULL,
    generated_at DATETIME(6) NOT NULL,
    frozen_at DATETIME(6) NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id),
    UNIQUE KEY uk_contest_scoreboard_snapshots_version (contest_id, source_version, frozen_at),
    KEY idx_contest_scoreboard_snapshots_contest (contest_id, created_at, id),
    CONSTRAINT fk_contest_scoreboard_snapshots_contest FOREIGN KEY (contest_id) REFERENCES contests(id) ON DELETE CASCADE,
    CONSTRAINT chk_contest_scoreboard_snapshots_mode CHECK (mode IN (1, 2, 3))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS contest_scoreboard_snapshot_rows (
    snapshot_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    display_name VARCHAR(200) NOT NULL,
    mode TINYINT UNSIGNED NOT NULL,
    rank_value BIGINT NOT NULL,
    ordinal BIGINT NOT NULL,
    solved_count INT NULL,
    total_penalty_ms BIGINT NULL,
    last_solved_at DATETIME(6) NULL,
    total_score BIGINT NULL,
    last_effective_submission_at DATETIME(6) NULL,

    PRIMARY KEY (snapshot_id, user_id),
    UNIQUE KEY uk_contest_scoreboard_snapshot_rows_ordinal (snapshot_id, ordinal),
    KEY idx_contest_scoreboard_snapshot_rows_rank (snapshot_id, rank_value, ordinal),
    CONSTRAINT fk_contest_scoreboard_snapshot_rows_snapshot FOREIGN KEY (snapshot_id)
        REFERENCES contest_scoreboard_snapshots(id) ON DELETE CASCADE,
    CONSTRAINT chk_contest_scoreboard_snapshot_rows_mode CHECK (mode IN (1, 2, 3)),
    CONSTRAINT chk_contest_scoreboard_snapshot_rows_rank CHECK (rank_value > 0 AND ordinal > 0),
    CONSTRAINT chk_contest_scoreboard_snapshot_rows_payload CHECK (
        (mode = 1 AND solved_count IS NOT NULL AND solved_count >= 0
            AND total_penalty_ms IS NOT NULL AND total_penalty_ms >= 0 AND total_score IS NULL)
        OR (mode IN (2, 3) AND solved_count IS NULL AND total_penalty_ms IS NULL
            AND total_score IS NOT NULL AND total_score >= 0)
    )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS contest_scoreboard_snapshot_problem_results (
    snapshot_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    problem_id BIGINT NOT NULL,
    label VARCHAR(32) NOT NULL,
    mode TINYINT UNSIGNED NOT NULL,
    solved BOOLEAN NULL,
    wrong_attempts INT NULL,
    penalty_ms BIGINT NULL,
    accepted_at DATETIME(6) NULL,
    first_to_solve BOOLEAN NULL,
    score BIGINT NULL,
    max_score BIGINT NULL,
    submission_count INT NULL,
    last_effective_submission_at DATETIME(6) NULL,

    PRIMARY KEY (snapshot_id, user_id, problem_id),
    KEY idx_contest_scoreboard_snapshot_results_problem (snapshot_id, problem_id, user_id),
    CONSTRAINT fk_contest_scoreboard_snapshot_results_row FOREIGN KEY (snapshot_id, user_id)
        REFERENCES contest_scoreboard_snapshot_rows(snapshot_id, user_id) ON DELETE CASCADE,
    CONSTRAINT chk_contest_scoreboard_snapshot_results_mode CHECK (mode IN (1, 2, 3)),
    CONSTRAINT chk_contest_scoreboard_snapshot_results_payload CHECK (
        (mode = 1 AND solved IS NOT NULL AND wrong_attempts IS NOT NULL AND penalty_ms IS NOT NULL
            AND wrong_attempts >= 0 AND penalty_ms >= 0 AND first_to_solve IS NOT NULL
            AND score IS NULL AND max_score IS NULL AND submission_count IS NULL)
        OR (mode IN (2, 3) AND solved IS NULL AND wrong_attempts IS NULL AND penalty_ms IS NULL
            AND first_to_solve IS NULL AND score IS NOT NULL AND score >= 0
            AND max_score IS NOT NULL AND max_score > 0 AND score <= max_score
            AND submission_count IS NOT NULL AND submission_count >= 0)
    )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

ALTER TABLE contest_scoreboard_state
    ADD CONSTRAINT fk_contest_scoreboard_state_snapshot FOREIGN KEY (active_frozen_snapshot_id)
        REFERENCES contest_scoreboard_snapshots(id) ON DELETE SET NULL;
