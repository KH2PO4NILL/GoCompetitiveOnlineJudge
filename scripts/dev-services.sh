#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
RUNTIME_DIR=${RUNTIME_DIR:-"$ROOT_DIR/.run"}
BIN_DIR="$RUNTIME_DIR/bin"
PID_DIR="$RUNTIME_DIR/pids"
LOG_DIR="$RUNTIME_DIR/logs"
ENV_FILE=${ENV_FILE:-"$ROOT_DIR/.env"}
GO_BIN=${GO:-go}
TEST_COUNT=${TEST_COUNT:-1}

SERVICES=(auth problem contest submit app admin)
declare -A PACKAGE=(
  [auth]="./services/grpc/auth"
  [problem]="./services/grpc/problem"
  [contest]="./services/grpc/contest"
  [submit]="./services/grpc/submit"
  [app]="./cmd/app"
  [admin]="./cmd/admin"
)
declare -A CONFIG=(
  [auth]="services/grpc/auth/etc/auth.yaml"
  [problem]="services/grpc/problem/etc/problem.yaml"
  [contest]="services/grpc/contest/etc/contest.yaml"
  [submit]="services/grpc/submit/etc/submit.yaml"
  [app]="cmd/app/etc/app-api.yaml"
  [admin]="cmd/admin/etc/admin-api.yaml"
)
declare -A PORT=(
  [auth]=8003
  [problem]=8006
  [contest]=8007
  [submit]=8004
  [app]=8002
  [admin]=8001
)

log() {
  printf '[dev] %s\n' "$*"
}

fail() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "$1 is required"
}

load_env() {
  [[ -f "$ENV_FILE" ]] || fail "$ENV_FILE does not exist; run 'make infra-env' first"
  set -a
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  set +a
}

check_env() {
  "$ROOT_DIR/scripts/dev-env.sh" check
}

prepare_runtime() {
  mkdir -p "$BIN_DIR" "$PID_DIR" "$LOG_DIR"
}

port_open() {
  local host=$1 port=$2
  timeout 1 bash -c "</dev/tcp/$host/$port" >/dev/null 2>&1
}

pid_file() {
  printf '%s/%s.pid\n' "$PID_DIR" "$1"
}

read_pid() {
  local file
  file=$(pid_file "$1")
  [[ -s "$file" ]] || return 1
  local pid
  read -r pid <"$file"
  [[ "$pid" =~ ^[0-9]+$ ]] || return 1
  printf '%s\n' "$pid"
}

is_managed_pid() {
  local name=$1 pid=$2
  [[ -r "/proc/$pid/cmdline" ]] || return 1
  local cmdline expected
  cmdline=$(tr '\0' ' ' <"/proc/$pid/cmdline")
  expected="$BIN_DIR/$name"
  [[ "$cmdline" == "$expected "* || "$cmdline" == "$expected" ]]
}

build_services() {
  require_command "$GO_BIN"
  prepare_runtime
  local name
  for name in "${SERVICES[@]}"; do
    log "building $name"
    (cd "$ROOT_DIR" && "$GO_BIN" build -o "$BIN_DIR/$name" "${PACKAGE[$name]}")
  done
}

start_service() {
  local name=$1
  local file pid port=${PORT[$name]}
  file=$(pid_file "$name")
  STARTED_NOW=0

  if pid=$(read_pid "$name" 2>/dev/null); then
    if kill -0 "$pid" 2>/dev/null && is_managed_pid "$name" "$pid"; then
      if port_open 127.0.0.1 "$port"; then
        log "$name is already running (pid=$pid, port=$port)"
        return 0
      fi
      fail "$name has a live managed process but port $port is not listening"
    fi
    rm -f "$file"
  fi

  [[ -x "$BIN_DIR/$name" ]] || fail "$BIN_DIR/$name is missing; run 'make services-build'"
  if port_open 127.0.0.1 "$port"; then
    fail "port $port for $name is already in use by an unmanaged process"
  fi

  log "starting $name on port $port"
  (
    cd "$ROOT_DIR"
    nohup "$BIN_DIR/$name" -f "${CONFIG[$name]}" >"$LOG_DIR/$name.log" 2>&1 &
    echo $! >"$file"
  )
  pid=$(read_pid "$name")
  STARTED_NOW=1

  local i
  for ((i = 0; i < 60; i++)); do
    if ! kill -0 "$pid" 2>/dev/null; then
      printf 'error: %s exited during startup; recent log output:\n' "$name" >&2
      tail -n 40 "$LOG_DIR/$name.log" >&2 || true
      rm -f "$file"
      return 1
    fi
    if port_open 127.0.0.1 "$port"; then
      log "$name is ready (pid=$pid, port=$port)"
      return 0
    fi
    sleep 0.5
  done

  printf 'error: %s did not listen on port %s within 30 seconds; recent log output:\n' "$name" "$port" >&2
  tail -n 40 "$LOG_DIR/$name.log" >&2 || true
  return 1
}

stop_service() {
  local name=$1
  local file pid
  file=$(pid_file "$name")
  if ! pid=$(read_pid "$name" 2>/dev/null); then
    rm -f "$file"
    log "$name is not running"
    return 0
  fi
  if ! kill -0 "$pid" 2>/dev/null; then
    rm -f "$file"
    log "$name had a stale pid file"
    return 0
  fi
  if ! is_managed_pid "$name" "$pid"; then
    printf 'error: refusing to stop pid %s for %s because it is not a managed %s binary\n' "$pid" "$name" "$BIN_DIR/$name" >&2
    return 1
  fi

  log "stopping $name (pid=$pid)"
  kill "$pid"
  local i
  for ((i = 0; i < 40; i++)); do
    if ! kill -0 "$pid" 2>/dev/null; then
      rm -f "$file"
      log "$name stopped"
      return 0
    fi
    sleep 0.25
  done
  log "$name did not stop gracefully; sending SIGKILL"
  kill -KILL "$pid" 2>/dev/null || true
  rm -f "$file"
}

start_all() {
  require_command timeout
  load_env
  check_env
  prepare_runtime
  local started=() name
  for name in "${SERVICES[@]}"; do
    if ! start_service "$name"; then
      local rollback
      printf 'error: startup failed; stopping services started by this invocation\n' >&2
      for ((rollback = ${#started[@]} - 1; rollback >= 0; rollback--)); do
        stop_service "${started[$rollback]}" || true
      done
      return 1
    fi
    if ((STARTED_NOW)); then
      started+=("$name")
    fi
  done
}

stop_all() {
  prepare_runtime
  local failed=0 i
  for ((i = ${#SERVICES[@]} - 1; i >= 0; i--)); do
    stop_service "${SERVICES[$i]}" || failed=1
  done
  return "$failed"
}

status_all() {
  prepare_runtime
  local name pid state
  printf '%-10s %-10s %-8s %s\n' SERVICE STATE PORT PID
  for name in "${SERVICES[@]}"; do
    state=stopped
    pid=-
    if pid=$(read_pid "$name" 2>/dev/null); then
      if kill -0 "$pid" 2>/dev/null && is_managed_pid "$name" "$pid"; then
        if port_open 127.0.0.1 "${PORT[$name]}"; then
          state=running
        else
          state=starting
        fi
      else
        state=stale
      fi
    fi
    printf '%-10s %-10s %-8s %s\n' "$name" "$state" "${PORT[$name]}" "$pid"
  done
}

check_tcp() {
  local name=$1 host=$2 port=$3
  if port_open "$host" "$port"; then
    printf 'OK   %-12s %s:%s\n' "$name" "$host" "$port"
  else
    printf 'FAIL %-12s %s:%s\n' "$name" "$host" "$port" >&2
    return 1
  fi
}

check_http() {
  local name=$1 url=$2
  if curl -fsS --max-time 5 "$url" >/dev/null; then
    printf 'OK   %-12s %s\n' "$name" "$url"
  else
    printf 'FAIL %-12s %s\n' "$name" "$url" >&2
    return 1
  fi
}

connectivity_infra() {
  load_env
  require_command curl
  require_command timeout
  local failed=0
  check_tcp mysql 127.0.0.1 "$MYSQL_PORT" || failed=1
  check_tcp redis 127.0.0.1 "$REDIS_PORT" || failed=1
  check_http etcd "http://127.0.0.1:${ETCD_PORT}/health" || failed=1
  check_http minio "http://127.0.0.1:${MINIO_PORT}/minio/health/live" || failed=1
  return "$failed"
}

connectivity_services() {
  require_command timeout
  local failed=0 name
  for name in "${SERVICES[@]}"; do
    check_tcp "$name" 127.0.0.1 "${PORT[$name]}" || failed=1
  done
  return "$failed"
}

smoke_test() {
  require_command curl
  connectivity_infra
  connectivity_services

  local response

  check_http admin-health "http://127.0.0.1:${PORT[admin]}/healthz"
  check_http app-health "http://127.0.0.1:${PORT[app]}/healthz"

  response=$(curl -fsS --max-time 8 "http://127.0.0.1:${PORT[app]}/api/v1/problems")
  printf 'OK   %-12s %s\n' app-problems "$response"
  response=$(curl -fsS --max-time 8 "http://127.0.0.1:${PORT[app]}/api/v1/contests")
  printf 'OK   %-12s %s\n' app-contests "$response"
}

integration_test() {
  require_command "$GO_BIN"
  load_env
  check_env
  local dsn
  dsn="root:${MYSQL_ROOT_PASSWORD}@tcp(127.0.0.1:${MYSQL_PORT})/?charset=utf8mb4&parseTime=true&loc=UTC"
  cd "$ROOT_DIR"
  CONTEST_MODEL_TEST_DSN="$dsn" \
  PROBLEM_MODEL_TEST_DSN="$dsn" \
  SUBMIT_MODEL_TEST_DSN="$dsn" \
    "$GO_BIN" test -count="$TEST_COUNT" -v \
      ./services/grpc/contest/model \
      ./services/grpc/problem/model \
      ./services/grpc/submit/model
}

follow_logs() {
  prepare_runtime
  compgen -G "$LOG_DIR/*.log" >/dev/null || fail "no service logs found in $LOG_DIR"
  tail -n 100 -F "$LOG_DIR"/*.log
}

usage() {
  cat <<USAGE
Usage: scripts/dev-services.sh <command>

Commands:
  build                Build all HTTP and gRPC service binaries
  start                Start all managed service binaries
  stop                 Stop all managed service binaries
  restart              Restart all managed service binaries
  status               Show managed service process status
  connectivity-infra   Check MySQL, Redis, etcd and MinIO connectivity
  connectivity-services Check all HTTP and gRPC listening ports
  connectivity         Run all connectivity checks
  smoke                Check health endpoints and HTTP-to-gRPC read paths
  test-integration     Run isolated MySQL model integration tests
  logs                 Follow all managed service logs
USAGE
}

cd "$ROOT_DIR"
case "${1:-}" in
  build)
    build_services
    ;;
  start)
    start_all
    ;;
  stop)
    stop_all
    ;;
  restart)
    stop_all
    start_all
    ;;
  status)
    status_all
    ;;
  connectivity-infra)
    connectivity_infra
    ;;
  connectivity-services)
    connectivity_services
    ;;
  connectivity)
    connectivity_infra
    connectivity_services
    ;;
  smoke)
    smoke_test
    ;;
  test-integration)
    integration_test
    ;;
  logs)
    follow_logs
    ;;
  *)
    usage
    exit 2
    ;;
esac
