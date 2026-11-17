-- Repair legacy orphan references before adding the constraint.
UPDATE users AS u LEFT JOIN schools AS s ON s.id = u.school_id
SET u.school_id = NULL WHERE u.school_id IS NOT NULL AND s.id IS NULL;
ALTER TABLE users ADD CONSTRAINT fk_users_school
FOREIGN KEY (school_id) REFERENCES schools(id)
ON UPDATE CASCADE ON DELETE SET NULL;
