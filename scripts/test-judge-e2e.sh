#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$root"
for command in docker go curl openssl; do command -v "$command" >/dev/null; done
project="judge-e2e-$(date +%s)-$$"
logs="$root/.run/judge-e2e/$project"
mkdir -p "$logs"
export JUDGE_E2E_PROXY_CONFIG
JUDGE_E2E_PROXY_CONFIG=$(mktemp /tmp/judge-e2e-proxy.XXXXXX.json)
export JUDGE_E2E_MQ_PORT=${JUDGE_E2E_MQ_PORT:-28081}
export JUDGE_E2E_MYSQL_PORT=${JUDGE_E2E_MYSQL_PORT:-23316}
export JUDGE_E2E_MINIO_PORT=${JUDGE_E2E_MINIO_PORT:-29000}
for port in "$JUDGE_E2E_MQ_PORT" "$JUDGE_E2E_MYSQL_PORT" "$JUDGE_E2E_MINIO_PORT"; do
  [[ "$port" =~ ^[0-9]+$ ]] && ((port > 1024 && port < 65536))
done
printf '{"grpcServerPort":%s,"remotingListenPort":18080}\n' "$JUDGE_E2E_MQ_PORT" > "$JUDGE_E2E_PROXY_CONFIG"
chmod 644 "$JUDGE_E2E_PROXY_CONFIG"
export JUDGE_E2E_MYSQL_PASSWORD
JUDGE_E2E_MYSQL_PASSWORD=$(openssl rand -hex 24)
export JUDGE_E2E_MINIO_ACCESS_KEY=judge-e2e-access
export JUDGE_E2E_MINIO_SECRET_KEY
JUDGE_E2E_MINIO_SECRET_KEY=$(openssl rand -hex 24)
compose=(docker compose --env-file /dev/null -p "$project" -f infra/e2e/compose.yaml)
cleanup() {
  result=$?
  trap - EXIT
  "${compose[@]}" ps > "$logs/services.log" 2>&1 || true
  if ((result != 0)); then
    "${compose[@]}" logs --tail 100 broker namesrv > "$logs/broker.log" 2>&1 || true
  fi
  "${compose[@]}" down --volumes --remove-orphans >> "$logs/cleanup.log" 2>&1 || result=1
  rm -f "$JUDGE_E2E_PROXY_CONFIG"
  printf 'Judge E2E exit=%s; diagnostics: %s\n' "$result" "$logs"
  exit "$result"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
"${compose[@]}" up -d --wait namesrv broker mysql minio > "$logs/startup.log" 2>&1
"${compose[@]}" --profile init run --rm topics > "$logs/topics.log" 2>&1
export ROCKETMQ_ENDPOINT="127.0.0.1:$JUDGE_E2E_MQ_PORT"
export SUBMIT_MODEL_TEST_DSN="root:$JUDGE_E2E_MYSQL_PASSWORD@tcp(127.0.0.1:$JUDGE_E2E_MYSQL_PORT)/?parseTime=true&timeout=5s&readTimeout=10s&writeTimeout=10s"
export PROBLEM_MODEL_TEST_DSN="$SUBMIT_MODEL_TEST_DSN"
export CONTEST_MODEL_TEST_DSN="$SUBMIT_MODEL_TEST_DSN"
export JUDGE_E2E_MINIO_ENDPOINT="http://127.0.0.1:$JUDGE_E2E_MINIO_PORT"
export JUDGE_E2E=1 JUDGE_E2E_ISOLATED=1
go test -race github.com/apache/rocketmq-clients/golang/v5 -run TestLocal -count=1 2>&1 | tee "$logs/sdk.log"
go test -race ./judge-worker/internal/e2e -count=1 -timeout=15m -v 2>&1 | tee "$logs/e2e.log"
go test -race ./services/grpc/problem/model ./services/grpc/contest/model ./services/grpc/submit/model -count=1 -timeout=5m -v 2>&1 | tee "$logs/models.log"
go test -race ./judge-worker ./services/grpc/submit/internal/outbox ./judge-worker/internal/consumer ./judge-worker/internal/processor ./judge-worker/internal/testdatacache ./judge-worker/internal/checker ./pkg/testdatamanifest -count=1 2>&1 | tee "$logs/recovery.log"
JUDGE_MQ_INTEGRATION=1 go test -race ./pkg/judgemq ./services/grpc/submit/internal/outbox -count=1 -v 2>&1 | tee "$logs/mq.log"
if [[ "${JUDGE_E2E_SLOW:-0}" == 1 ]]; then
  JUDGE_LANGUAGE_INTEGRATION=1 go test ./judge-worker/internal/engine -count=1 -timeout=15m -v 2>&1 | tee "$logs/languages.log"
  JUDGE_DOCKER_INTEGRATION=1 JUDGE_SANDBOX_TEST_IMAGE=gcoj/judge-go:1.2.0-runtime go test ./judge-worker/internal/sandbox ./judge-worker/internal/consumer -count=1 -timeout=5m -v 2>&1 | tee "$logs/security.log"
fi
