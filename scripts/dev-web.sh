#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
RUNTIME_DIR=${RUNTIME_DIR:-"$ROOT_DIR/.run"}
WEB_PORT=${WEB_PORT:-5173}
VITE="$ROOT_DIR/web/node_modules/vite/bin/vite.js"
PID_FILE="$RUNTIME_DIR/pids/web.pid"
LOG_FILE="$RUNTIME_DIR/logs/web.log"

fail() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

check_tools() {
  local command
  for command in node npm curl timeout; do
    command -v "$command" >/dev/null || fail "$command is required"
  done
  node -e 'const [major, minor] = process.versions.node.split(".").map(Number); if (!((major === 22 && minor >= 13) || major >= 24)) process.exit(1)' || fail 'Node >=22.13 <23 or >=24 is required (see web/package.json)'
}

read_pid() {
  [[ -s "$PID_FILE" ]] || return 1
  read -r PID <"$PID_FILE"
  [[ "$PID" =~ ^[0-9]+$ && "$PID" -gt 1 ]]
}

managed() {
  local arguments=()
  [[ -r "/proc/$PID/cmdline" ]] || return 1
  mapfile -d '' -t arguments <"/proc/$PID/cmdline"
  [[ ${#arguments[@]} -eq 7 && ${arguments[1]} == "$VITE" && ${arguments[2]} == --host && ${arguments[3]} == 127.0.0.1 && ${arguments[4]} == --port && ${arguments[6]} == --strictPort ]]
}

stop_web() {
  if ! read_pid || ! kill -0 "$PID" 2>/dev/null; then
    rm -f "$PID_FILE"
    printf '[dev] frontend is not running\n'
    return
  fi
  managed || fail "refusing to stop unmanaged PID $PID"
  kill "$PID"
  local attempt
  for ((attempt = 0; attempt < 40; attempt++)); do
    if ! managed; then
      rm -f "$PID_FILE"
      printf '[dev] frontend stopped\n'
      return
    fi
    sleep 0.25
  done
  managed && kill -KILL "$PID"
  rm -f "$PID_FILE"
}

start_web() {
  check_tools
  [[ "$WEB_PORT" =~ ^[1-9][0-9]{0,4}$ ]] && ((WEB_PORT <= 65535)) || fail 'WEB_PORT must be between 1 and 65535'
  [[ -f "$VITE" ]] || fail "frontend dependencies missing; run 'make web-install'"
  mkdir -p "$RUNTIME_DIR/pids" "$RUNTIME_DIR/logs"
  if read_pid && kill -0 "$PID" 2>/dev/null; then
    managed || fail "PID file points to unmanaged PID $PID"
    fail "frontend is already running; use 'make web-restart'"
  fi
  if timeout 1 bash -c "</dev/tcp/127.0.0.1/$WEB_PORT" >/dev/null 2>&1; then
    fail "port $WEB_PORT is already in use; choose WEB_PORT or stop its owner"
  fi
  (
    cd "$ROOT_DIR/web"
    nohup node "$VITE" --host 127.0.0.1 --port "$WEB_PORT" --strictPort </dev/null >"$LOG_FILE" 2>&1 &
    echo $! >"$PID_FILE"
  )
  read_pid
  local attempt
  for ((attempt = 0; attempt < 60; attempt++)); do
    if ! kill -0 "$PID" 2>/dev/null; then
      rm -f "$PID_FILE"
      fail "frontend exited during startup; inspect $LOG_FILE"
    fi
    if managed && curl -fsS --max-time 1 "http://127.0.0.1:$WEB_PORT/@vite/client" >/dev/null 2>&1; then
      printf '[dev] frontend ready: http://127.0.0.1:%s (PID %s)\n' "$WEB_PORT" "$PID"
      return
    fi
    sleep 0.5
  done
  stop_web
  fail "frontend startup timed out; inspect $LOG_FILE"
}

case "${1:-}" in
  check) check_tools ;;
  start) start_web ;;
  stop) stop_web ;;
  status)
    if read_pid && managed; then
      printf '[dev] frontend running (PID %s)\n' "$PID"
    else
      printf '[dev] frontend stopped or PID file stale\n'
    fi
    ;;
  logs) tail -n 100 -F "$LOG_FILE" ;;
  *) fail 'usage: bash scripts/dev-web.sh {check|start|stop|status|logs}' ;;
esac
