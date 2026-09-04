#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
ENV_FILE=${ENV_FILE:-"$ROOT_DIR/.env"}
COMMAND=${1:-check}

fail() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

load_env() {
  [[ -f "$ENV_FILE" ]] || fail "$ENV_FILE does not exist; run 'make infra-env' first"
  set -a
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  set +a
}

append_env() {
  local key=$1 value=$2
  [[ "$value" != *"'"* ]] || fail "cannot safely write $key because its value contains a single quote"
  printf "%s='%s'\n" "$key" "$value" >>"$ENV_FILE"
  export "$key=$value"
  ADDED_KEYS+=("$key")
}

ensure_value() {
  local key=$1 value=$2
  if [[ -z ${!key:-} ]]; then
    append_env "$key" "$value"
  fi
}

random_hex() {
  local bytes=$1
  openssl rand -hex "$bytes"
}

require_values() {
  local missing=() key
  for key in "$@"; do
    if [[ -z ${!key:-} ]]; then
      missing+=("$key")
    fi
  done
  if ((${#missing[@]})); then
    printf 'error: missing required variables in %s:\n' "$ENV_FILE" >&2
    printf '  - %s\n' "${missing[@]}" >&2
    exit 1
  fi
}

sync_env() {
  command -v openssl >/dev/null 2>&1 || fail "openssl is required to generate missing local secrets"
  require_values MYSQL_PORT MYSQL_DATABASE MYSQL_USER MYSQL_PASSWORD MYSQL_ROOT_PASSWORD \
    REDIS_PORT REDIS_PASSWORD ETCD_PORT MINIO_PORT MINIO_CONSOLE_PORT \
    MINIO_ROOT_USER MINIO_ROOT_PASSWORD MINIO_BUCKET

  local mysql_dsn
  mysql_dsn="${MYSQL_USER}:${MYSQL_PASSWORD}@tcp(127.0.0.1:${MYSQL_PORT})/${MYSQL_DATABASE}?charset=utf8mb4&parseTime=true&loc=UTC"

  ensure_value AUTH_MYSQL_DSN "$mysql_dsn"
  ensure_value AUTH_REDIS_PASSWORD "$REDIS_PASSWORD"
  ensure_value AUTH_TOKEN_SECRET "$(random_hex 32)"
  ensure_value AUTH_INTERNAL_PROBLEM_TOKEN "$(random_hex 32)"
  ensure_value AUTH_INTERNAL_CONTEST_TOKEN "$(random_hex 32)"
  ensure_value AUTH_INTERNAL_SUBMIT_TOKEN "$(random_hex 32)"

  ensure_value PROBLEM_MYSQL_DSN "$mysql_dsn"
  ensure_value PROBLEM_REDIS_PASSWORD "$REDIS_PASSWORD"
  ensure_value PROBLEM_OBJECT_STORAGE_ENDPOINT "http://127.0.0.1:${MINIO_PORT}"
  ensure_value PROBLEM_OBJECT_STORAGE_REGION "us-east-1"
  ensure_value PROBLEM_OBJECT_STORAGE_BUCKET "$MINIO_BUCKET"
  ensure_value PROBLEM_OBJECT_STORAGE_ACCESS_KEY "$MINIO_ROOT_USER"
  ensure_value PROBLEM_OBJECT_STORAGE_SECRET_KEY "$MINIO_ROOT_PASSWORD"
  ensure_value PROBLEM_INTERNAL_SUBMIT_TOKEN "$(random_hex 32)"
  ensure_value PROBLEM_INTERNAL_JUDGE_TOKEN "$(random_hex 32)"
  ensure_value PROBLEM_INTERNAL_CONTEST_TOKEN "$(random_hex 32)"

  ensure_value CONTEST_MYSQL_DSN "$mysql_dsn"
  ensure_value CONTEST_INTERNAL_PROBLEM_TOKEN "$(random_hex 32)"
  ensure_value CONTEST_INTERNAL_SUBMIT_TOKEN "$(random_hex 32)"

  ensure_value SUBMIT_MYSQL_DSN "$mysql_dsn"
  ensure_value SUBMIT_REDIS_PASSWORD "$REDIS_PASSWORD"
  ensure_value SUBMIT_INTERNAL_JUDGE_TOKEN "$(random_hex 32)"

  ensure_value JUDGE_WORKER_ID "judge-worker-local-1"
  ensure_value JUDGE_SUBMIT_RPC_TARGET "127.0.0.1:8004"
  ensure_value JUDGE_PROBLEM_RPC_TARGET "127.0.0.1:8006"
  ensure_value JUDGE_TEST_DATA_CACHE_DIR ".run/judge-worker/cache"
  ensure_value JUDGE_SANDBOX_WORK_DIR ".run/judge-worker/work"
  ensure_value JUDGE_DOCKER_ENDPOINT "unix:///var/run/docker.sock"

  if ((${#ADDED_KEYS[@]})); then
    chmod 600 "$ENV_FILE"
    printf 'Updated %s with missing local-development variables:\n' "$ENV_FILE"
    printf '  + %s\n' "${ADDED_KEYS[@]}"
  else
    printf '%s is complete; no variables were added.\n' "$ENV_FILE"
  fi
}

check_env() {
  require_values \
    MYSQL_PORT MYSQL_DATABASE MYSQL_USER MYSQL_PASSWORD MYSQL_ROOT_PASSWORD \
    REDIS_PORT REDIS_PASSWORD ETCD_PORT MINIO_PORT MINIO_CONSOLE_PORT \
    MINIO_ROOT_USER MINIO_ROOT_PASSWORD MINIO_BUCKET \
    AUTH_MYSQL_DSN AUTH_REDIS_PASSWORD AUTH_TOKEN_SECRET \
    AUTH_INTERNAL_PROBLEM_TOKEN AUTH_INTERNAL_CONTEST_TOKEN AUTH_INTERNAL_SUBMIT_TOKEN \
    PROBLEM_MYSQL_DSN PROBLEM_REDIS_PASSWORD \
    PROBLEM_OBJECT_STORAGE_ENDPOINT PROBLEM_OBJECT_STORAGE_REGION \
    PROBLEM_OBJECT_STORAGE_BUCKET PROBLEM_OBJECT_STORAGE_ACCESS_KEY \
    PROBLEM_OBJECT_STORAGE_SECRET_KEY PROBLEM_INTERNAL_SUBMIT_TOKEN \
    PROBLEM_INTERNAL_JUDGE_TOKEN PROBLEM_INTERNAL_CONTEST_TOKEN \
    CONTEST_MYSQL_DSN CONTEST_INTERNAL_PROBLEM_TOKEN CONTEST_INTERNAL_SUBMIT_TOKEN \
    SUBMIT_MYSQL_DSN SUBMIT_REDIS_PASSWORD SUBMIT_INTERNAL_JUDGE_TOKEN \
    JUDGE_WORKER_ID JUDGE_SUBMIT_RPC_TARGET JUDGE_PROBLEM_RPC_TARGET \
    JUDGE_TEST_DATA_CACHE_DIR JUDGE_SANDBOX_WORK_DIR JUDGE_DOCKER_ENDPOINT
  printf '%s contains all required local service variables.\n' "$ENV_FILE"
}

ADDED_KEYS=()
load_env
case "$COMMAND" in
  sync)
    sync_env
    ;;
  check)
    check_env
    ;;
  *)
    fail "unknown command '$COMMAND' (expected: sync or check)"
    ;;
esac
