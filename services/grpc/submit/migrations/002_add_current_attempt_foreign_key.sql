ALTER TABLE submissions
    ADD CONSTRAINT fk_submissions_current_attempt
    FOREIGN KEY (id, current_attempt_id)
    REFERENCES submission_attempts(submission_id, id)
    ON DELETE RESTRICT;
