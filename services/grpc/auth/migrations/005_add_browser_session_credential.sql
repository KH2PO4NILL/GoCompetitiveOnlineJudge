-- Browser sessions survive interrupted responses; API refresh tokens remain single use.
ALTER TABLE user_sessions
ADD COLUMN browser_session_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
ADD UNIQUE KEY uk_user_sessions_browser_session_hash (browser_session_hash);
