#!/bin/sh

set -eu

mysql_query() {
    MYSQL_PWD="$MYSQL_PASSWORD" mysql \
        --protocol=tcp \
        --host=mysql \
        --user="$MYSQL_USER" \
        --database="$MYSQL_DATABASE" \
        --batch \
        --skip-column-names \
        --execute="$1"
}

file_checksum() {
    sha256sum "$1" | awk '{print $1}'
}

record_migration() {
    version=$1
    checksum=$2
    mysql_query "INSERT INTO infra_schema_migrations (version, checksum) VALUES ('$version', '$checksum')"
}

apply_migration() {
    version=$1
    file=$2
    baseline_query=$3
    checksum=$(file_checksum "$file")
    stored_checksum=$(mysql_query "SELECT checksum FROM infra_schema_migrations WHERE version = '$version'" || true)

    if [ -n "$stored_checksum" ]; then
        if [ "$stored_checksum" != "$checksum" ]; then
            echo "error: applied migration $version has changed" >&2
            exit 1
        fi
        echo "Migration $version already applied."
        return
    fi

    if [ "$(mysql_query "$baseline_query")" = "1" ]; then
        record_migration "$version" "$checksum"
        echo "Baselined existing migration $version."
        return
    fi

    echo "Applying migration $version..."
    MYSQL_PWD="$MYSQL_PASSWORD" mysql \
        --protocol=tcp \
        --host=mysql \
        --user="$MYSQL_USER" \
        --database="$MYSQL_DATABASE" < "$file"
    record_migration "$version" "$checksum"
}

mysql_query 'CREATE TABLE IF NOT EXISTS infra_schema_migrations (
    version VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    checksum CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    applied_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (version)
) ENGINE=InnoDB'

apply_migration \
    'auth/001_create_user.sql' \
    '/migrations/auth/001_create_user.sql' \
    "SELECT COUNT(*) = 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'users'"

apply_migration \
    'auth/002_create_school.sql' \
    '/migrations/auth/002_create_school.sql' \
    "SELECT COUNT(*) = 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'schools'"

apply_migration \
    'auth/003_create_user_sessions.sql' \
    '/migrations/auth/003_create_user_sessions.sql' \
    "SELECT COUNT(*) = 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'user_sessions'"

apply_migration \
    'problem/001_create_problem.sql' \
    '/migrations/problem/001_create_problem.sql' \
    "SELECT COUNT(*) = 6 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name IN ('problems', 'problem_tags', 'test_data_versions', 'test_cases', 'problem_judge_revisions', 'test_data_uploads')"

apply_migration \
    'problem/002_align_problem_schema.sql' \
    '/migrations/problem/002_align_problem_schema.sql' \
    "SELECT COUNT(*) = 6 FROM information_schema.table_constraints WHERE constraint_schema = DATABASE() AND constraint_name IN ('fk_problem_tags_problem', 'fk_test_data_versions_problem', 'fk_test_cases_version', 'fk_judge_revisions_problem', 'fk_judge_revisions_test_data_version', 'fk_test_data_uploads_problem')"

apply_migration \
    'problem/003_add_problem_revision_aggregate.sql' \
    '/migrations/problem/003_add_problem_revision_aggregate.sql' \
    "SELECT
        (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name IN ('problem_revisions', 'problem_tag_definitions', 'problem_revision_tags')) = 3
        AND
        (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'test_data_uploads' AND column_name IN ('finalized_test_data_version_id', 'finalized_revision')) = 2"

echo 'MySQL migrations are up to date.'
