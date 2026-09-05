#!/usr/bin/env bash
set -euo pipefail
umask 077
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$root"
for command in docker go npm curl openssl; do command -v "$command" >/dev/null; done
mkdir -p .run
runtime=$(mktemp -d "$root/.run/web-auth.XXXXXX")
project="gcoj-web-auth-$(date +%s)-$$"
pids=()
cleanup() {
  result=$?
  trap - EXIT
  for process in "${pids[@]}"; do kill -TERM "$process" 2>/dev/null || true; done
  for process in "${pids[@]}"; do
    for ((attempt = 0; attempt < 50; attempt++)); do
      kill -0 "$process" 2>/dev/null || break
      sleep .1
    done
    if kill -0 "$process" 2>/dev/null; then kill -KILL "$process" 2>/dev/null || true; fi
    wait "$process" 2>/dev/null || true
  done
  docker rm -fv "$project-mysql" "$project-redis" >/dev/null 2>&1 || true
  rm -rf -- "$runtime"
  printf 'Isolated browser auth exit=%s; private runtime and test credentials removed.\n' "$result"
  exit "$result"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
export MYSQL_ROOT_PASSWORD MYSQL_PWD GCOJ_AUTH_E2E_PASSWORD
MYSQL_ROOT_PASSWORD=$(openssl rand -hex 24)
MYSQL_PWD="$MYSQL_ROOT_PASSWORD"
GCOJ_AUTH_E2E_PASSWORD=$(openssl rand -hex 16)
docker run -d --name "$project-mysql" --publish 127.0.0.1::3306 --env MYSQL_ROOT_PASSWORD --env MYSQL_DATABASE=auth_e2e mysql:8.4 >/dev/null
docker run -d --name "$project-redis" --publish 127.0.0.1::6379 redis:8.4-alpine >/dev/null
ready=0
for ((attempt = 0; attempt < 90; attempt++)); do
  if docker exec --env MYSQL_PWD "$project-mysql" mysqladmin ping --host=127.0.0.1 --protocol=TCP --silent >/dev/null 2>&1; then ready=1; break; fi
  sleep 1
done
[[ "$ready" == 1 ]] || { echo 'Isolated MySQL did not become ready' >&2; exit 1; }
mysql_port=$(docker port "$project-mysql" 3306/tcp | cut -d: -f2)
redis_port=$(docker port "$project-redis" 6379/tcp | cut -d: -f2)
export GCOJ_AUTH_E2E=1 GCOJ_AUTH_E2E_RUNTIME="$runtime"
export GCOJ_AUTH_E2E_DSN="root:$MYSQL_ROOT_PASSWORD@tcp(127.0.0.1:$mysql_port)/auth_e2e?parseTime=true&timeout=5s&readTimeout=10s&writeTimeout=10s"
export GCOJ_AUTH_E2E_REDIS="127.0.0.1:$redis_port"
go run ./scripts/webauth
read -r auth_port app_port admin_port < "$runtime/ports"
for service in auth app admin; do
  package="./cmd/$service"
  [[ "$service" != auth ]] || package=./services/grpc/auth
  go build -o "$runtime/$service" "$package"
  "$runtime/$service" -f "$runtime/$service.json" > "$runtime/$service.log" 2>&1 &
  pids+=("$!")
  port="$auth_port"
  [[ "$service" != app ]] || port="$app_port"
  [[ "$service" != admin ]] || port="$admin_port"
  ready=0
  for ((attempt = 0; attempt < 60; attempt++)); do
    kill -0 "${pids[-1]}" 2>/dev/null || { echo "Isolated $service exited during startup" >&2; exit 1; }
    if [[ "$service" == auth ]]; then
      if timeout 1 bash -c "</dev/tcp/127.0.0.1/$port" >/dev/null 2>&1; then ready=1; break; fi
    elif curl --noproxy '*' -fsS "http://127.0.0.1:$port/healthz" >/dev/null 2>&1; then ready=1; break
    fi
    sleep .5
  done
  [[ "$ready" == 1 ]] || { echo "Isolated $service readiness timeout" >&2; exit 1; }
done
export GCOJ_APP_UPSTREAM="http://127.0.0.1:$app_port" GCOJ_ADMIN_UPSTREAM="http://127.0.0.1:$admin_port"
export GCOJ_E2E_OUTPUT_DIR="$runtime/browser"
cd web
npm run test:browser
