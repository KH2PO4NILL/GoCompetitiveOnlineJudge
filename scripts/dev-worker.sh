#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
RUNTIME_DIR=${RUNTIME_DIR:-"$ROOT_DIR/.run"}
[[ "$RUNTIME_DIR" == /* ]] || RUNTIME_DIR="$ROOT_DIR/$RUNTIME_DIR"
WORKER_BINARY="$RUNTIME_DIR/bin/judge-worker"
ENV_FILE=${ENV_FILE:-"$ROOT_DIR/.env"}
WORKER_CONFIG=${WORKER_CONFIG:-"$ROOT_DIR/judge-worker/etc/judge-worker.yaml"}
GO_BIN=${GO:-go}

fail() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

load_env() {
  [[ -f "$ENV_FILE" && -r "$ENV_FILE" ]] || fail 'worker environment file is missing or unreadable; set ENV_FILE to a trusted shell environment file'
  set -a
  source "$ENV_FILE"
  set +a
}

case "${1:-help}" in
  build)
    command -v "$GO_BIN" >/dev/null 2>&1 || fail 'Go is required'
    mkdir -p "$RUNTIME_DIR/bin"
    BUILD_FILE=$(mktemp "$RUNTIME_DIR/bin/.judge-worker.XXXXXXXX")
    trap 'rm -f -- "$BUILD_FILE"' EXIT
    cd "$ROOT_DIR"
    "$GO_BIN" build -o "$BUILD_FILE" ./judge-worker
    mv -f -- "$BUILD_FILE" "$WORKER_BINARY"
    printf 'Worker binary built.\n'
    ;;
  run)
    load_env
    [[ -x "$WORKER_BINARY" ]] || fail 'worker binary is missing; run make worker-build first'
    [[ -f "$WORKER_CONFIG" ]] || fail 'worker configuration file is missing'
    export JUDGE_CONSUMER_ENABLED=${JUDGE_CONSUMER_ENABLED:-true}
    export JUDGE_METRICS_ENABLED=${JUDGE_METRICS_ENABLED:-true}
    export JUDGE_METRICS_LISTEN_ON=${JUDGE_METRICS_LISTEN_ON:-127.0.0.1:9100}
    cd "$ROOT_DIR"
    exec "$WORKER_BINARY" -f "$WORKER_CONFIG"
    ;;
  status)
    load_env
    command -v curl >/dev/null 2>&1 || fail 'curl is required'
    ADDRESS=${JUDGE_METRICS_LISTEN_ON:-127.0.0.1:9100}
    [[ "$ADDRESS" =~ ^(127\.0\.0\.1|localhost|\[::1\]):([0-9]{1,5})$ ]] || fail 'local worker status requires a loopback host and explicit port'
    PORT=$((10#${BASH_REMATCH[2]}))
    (( PORT >= 1 && PORT <= 65535 )) || fail 'worker monitoring port is outside 1..65535'
    if ! STATUS=$(curl --disable --noproxy '*' --silent --connect-timeout 1 --max-time 2 --output /dev/null --write-out '%{http_code}' -- "http://$ADDRESS/readyz"); then
      fail 'worker monitoring endpoint is unreachable'
    fi
    case "$STATUS" in
      200) printf 'Worker consumer is ready (not an end-to-end judging check).\n' ;;
      503) fail 'worker is live but its consumer is not ready' ;;
      *) fail 'worker readiness endpoint returned an unexpected HTTP status' ;;
    esac
    ;;
  help)
    printf '%s\n' 'Usage: bash scripts/dev-worker.sh build|run|status' \
      'run replaces this shell with the foreground Worker; Ctrl-C or SIGTERM drains tasks.' \
      'No infrastructure is started, environment file rewritten, or unrelated process stopped.'
    ;;
  *) fail 'unknown worker command; expected build, run or status' ;;
esac
