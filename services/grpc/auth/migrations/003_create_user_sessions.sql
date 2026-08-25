CREATE TABLE user_sessions (
    id BIGINT NOT NULL AUTO_INCREMENT,
    user_id BIGINT NOT NULL,
    refresh_token_hash CHAR(64)
        CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    device_id VARCHAR(128) NULL,
    device_name VARCHAR(128) NULL,
    ip_address VARCHAR(45) NULL,
    user_agent VARCHAR(512) NULL,
    expires_at DATETIME(3) NOT NULL,
    last_used_at DATETIME(3) NULL,
    revoked_at DATETIME(3) NULL,
    revoked_reason VARCHAR(64) NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
        ON UPDATE CURRENT_TIMESTAMP(3),

    PRIMARY KEY (id),
    UNIQUE KEY uk_user_sessions_refresh_token_hash (refresh_token_hash),
    KEY idx_user_sessions_user_state (user_id, revoked_at, expires_at),
    KEY idx_user_sessions_user_device (user_id, device_id),
    KEY idx_user_sessions_expires_at (expires_at),

    CONSTRAINT fk_user_sessions_user
        FOREIGN KEY (user_id) REFERENCES users (id)
        ON UPDATE CASCADE
        ON DELETE CASCADE
);
