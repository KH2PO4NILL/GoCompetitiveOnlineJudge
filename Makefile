SHELL := /bin/sh
.DEFAULT_GOAL := help

GO ?= go
GOCTL ?= goctl
GOCTL_STYLE ?= go_zero
COMPOSE ?= docker compose
RUNTIME_DIR ?= .run
DEV_SERVICES ?= ./scripts/dev-services.sh
DEV_ENV ?= ./scripts/dev-env.sh
TEST_COUNT ?= 1

API_SERVICES := admin app
RPC_ROOT := services/grpc
RPC_PROTO_FILES := $(wildcard $(RPC_ROOT)/*/*.proto $(RPC_ROOT)/*/proto/*.proto)
RPC_SERVICES := $(sort $(notdir $(basename $(RPC_PROTO_FILES))))
RPC_MULTIPLE_SERVICES := auth
PROTOBUF_INCLUDE ?= $(shell $(GO) list -m -f '{{.Dir}}/third_party' github.com/google/gnostic-models 2>/dev/null)

.PHONY: help tools tools-rpc tools-infra tools-services generate api rpc \
	$(addprefix api-,$(API_SERVICES)) \
	$(addprefix rpc-,$(RPC_SERVICES)) \
	problem-rpc \
	fmt vet test test-http test-grpc test-integration test-services test-all coverage build tidy clean \
	infra-env env-check infra-up infra-migrate infra-down infra-reset infra-logs infra-ps \
	mysql-shell redis-shell \
	run-auth run-problem run-contest run-submit run-app run-admin \
	services-build services-start services-stop services-restart services-status services-logs services-up \
	connectivity-infra connectivity-services connectivity smoke \
	up down restart stop dev-up dev-down

help:
	@printf '%s\n' \
		'Usage: make <target>' \
		'' \
		'One-click local development:' \
		'  up               Start infrastructure, rebuild/restart services, then run smoke tests' \
		'  down             Stop managed services and infrastructure, preserving data' \
		'  restart          Rebuild/restart all services and verify them' \
		'  stop             Stop only the managed HTTP and gRPC services' \
		'  dev-up/dev-down  Compatibility aliases for up/down' \
		'' \
		'Service lifecycle:' \
		'  services-build   Build six managed HTTP/gRPC binaries into $(RUNTIME_DIR)/bin' \
		'  services-start   Start Auth, Problem, Contest, Submit, App and Admin' \
		'  services-stop    Stop all managed HTTP and gRPC services' \
		'  services-restart Restart all managed HTTP and gRPC services' \
		'  services-status  Show service PID and listening-port status' \
		'  services-logs    Follow logs under $(RUNTIME_DIR)/logs' \
		'' \
		'Tests and checks:' \
		'  test             Run all Go tests once (DB integration tests skip without DSNs)' \
		'  test-http        Test HTTP packages under ./cmd/...' \
		'  test-grpc        Test gRPC packages under ./services/grpc/...' \
		'  test-integration Run isolated MySQL model integration tests' \
		'  test-services    Run HTTP, gRPC and MySQL integration tests' \
		'  test-all         Run vet, service tests and build' \
		'  connectivity     Check infrastructure and all service ports' \
		'  smoke            Check health endpoints and HTTP-to-gRPC read paths' \
		'' \
		'Go workflow:' \
		'  fmt              Format all Go packages' \
		'  vet              Run go vet' \
		'  coverage         Run tests and write coverage.out' \
		'  build            Compile all packages' \
		'  tidy             Update go.mod and go.sum' \
		'  clean            Stop services and remove local build/test artifacts' \
		'' \
		'Local infrastructure:' \
		'  infra-env        Create .env or append missing local variables without overwriting values' \
		'  env-check        Validate variables required by all HTTP/gRPC services' \
		'  infra-up         Start MySQL, Redis, etcd and MinIO and wait until healthy' \
		'  infra-migrate    Apply pending Auth, Problem, Contest and Submit MySQL migrations' \
		'  infra-down       Stop local infrastructure and keep its data' \
		'  infra-reset      Stop infrastructure and delete all local data' \
		'  infra-logs       Follow infrastructure logs' \
		'  infra-ps         Show infrastructure status' \
		'  mysql-shell      Open a MySQL shell as the application user' \
		'  redis-shell      Open an authenticated Redis CLI' \
		'' \
		'Code generation:' \
		'  api              Generate all HTTP services' \
		'  api-<service>    Generate one HTTP service (admin or app)' \
		'  rpc              Generate all gRPC services' \
		'  rpc-<service>    Generate one gRPC service ($(RPC_SERVICES))' \
		'  problem-rpc      Generate the problem gRPC service' \
		'  generate         Generate all HTTP and gRPC services' \
		'' \
		'Tool checks:' \
		'  tools            Check Go and goctl' \
		'  tools-rpc        Check the protobuf toolchain' \
		'  tools-infra      Check Docker Compose and OpenSSL' \
		'  tools-services   Check local service-management tools'

tools:
	@command -v "$(GO)" >/dev/null || { echo 'error: go is not installed or not in PATH' >&2; exit 1; }
	@command -v "$(GOCTL)" >/dev/null || { echo 'error: goctl is not installed or not in PATH' >&2; exit 1; }

tools-rpc: tools
	@command -v protoc >/dev/null || { echo 'error: protoc is not installed or not in PATH' >&2; exit 1; }
	@command -v protoc-gen-go >/dev/null || { echo 'error: protoc-gen-go is not installed or not in PATH' >&2; exit 1; }
	@command -v protoc-gen-go-grpc >/dev/null || { echo 'error: protoc-gen-go-grpc is not installed or not in PATH' >&2; exit 1; }
	@test -f "$(PROTOBUF_INCLUDE)/google/protobuf/empty.proto" || { echo 'error: Google well-known protobuf definitions are unavailable' >&2; exit 1; }

tools-infra:
	@command -v docker >/dev/null || { echo 'error: docker is not installed' >&2; exit 1; }
	@docker compose version >/dev/null || { echo 'error: docker compose is not available' >&2; exit 1; }
	@command -v openssl >/dev/null || { echo 'error: openssl is not installed' >&2; exit 1; }

tools-services: tools-infra
	@command -v bash >/dev/null || { echo 'error: bash is not installed' >&2; exit 1; }
	@command -v curl >/dev/null || { echo 'error: curl is not installed' >&2; exit 1; }
	@command -v timeout >/dev/null || { echo 'error: GNU timeout is not installed' >&2; exit 1; }

.env: | tools-infra
	@umask 077; \
	mysql_root_password=$$(openssl rand -hex 16); \
	mysql_password=$$(openssl rand -hex 16); \
	redis_password=$$(openssl rand -hex 16); \
	token_secret=$$(openssl rand -hex 32); \
	minio_password=$$(openssl rand -hex 16); \
	submit_service_token=$$(openssl rand -hex 32); \
	judge_service_token=$$(openssl rand -hex 32); \
	contest_problem_service_token=$$(openssl rand -hex 32); \
	problem_contest_service_token=$$(openssl rand -hex 32); \
	contest_submit_service_token=$$(openssl rand -hex 32); \
	auth_problem_service_token=$$(openssl rand -hex 32); \
	auth_contest_service_token=$$(openssl rand -hex 32); \
	auth_submit_service_token=$$(openssl rand -hex 32); \
	submit_judge_service_token=$$(openssl rand -hex 32); \
	{ \
		printf '%s\n' 'MYSQL_PORT=3307'; \
		printf '%s\n' 'MYSQL_DATABASE=go_coj'; \
		printf '%s\n' 'MYSQL_USER=go_coj'; \
		printf '%s\n' "MYSQL_PASSWORD=$$mysql_password"; \
		printf '%s\n' "MYSQL_ROOT_PASSWORD=$$mysql_root_password"; \
		printf '%s\n' 'REDIS_PORT=6379'; \
		printf '%s\n' "REDIS_PASSWORD=$$redis_password"; \
		printf '%s\n' 'ETCD_PORT=2379'; \
		printf '%s\n' 'MINIO_PORT=9000'; \
		printf '%s\n' 'MINIO_CONSOLE_PORT=9001'; \
		printf '%s\n' 'MINIO_ROOT_USER=localadmin'; \
		printf '%s\n' "MINIO_ROOT_PASSWORD=$$minio_password"; \
		printf '%s\n' 'MINIO_BUCKET=problem-test-data'; \
		printf '%s\n' "AUTH_MYSQL_DSN='go_coj:$$mysql_password@tcp(127.0.0.1:3307)/go_coj?charset=utf8mb4&parseTime=true&loc=UTC'"; \
		printf '%s\n' "AUTH_REDIS_PASSWORD=$$redis_password"; \
		printf '%s\n' "AUTH_TOKEN_SECRET=$$token_secret"; \
		printf '%s\n' "AUTH_INTERNAL_PROBLEM_TOKEN=$$auth_problem_service_token"; \
		printf '%s\n' "AUTH_INTERNAL_CONTEST_TOKEN=$$auth_contest_service_token"; \
		printf '%s\n' "AUTH_INTERNAL_SUBMIT_TOKEN=$$auth_submit_service_token"; \
		printf '%s\n' "PROBLEM_MYSQL_DSN='go_coj:$$mysql_password@tcp(127.0.0.1:3307)/go_coj?charset=utf8mb4&parseTime=true&loc=UTC'"; \
		printf '%s\n' "PROBLEM_REDIS_PASSWORD=$$redis_password"; \
		printf '%s\n' 'PROBLEM_OBJECT_STORAGE_ENDPOINT=http://127.0.0.1:9000'; \
		printf '%s\n' 'PROBLEM_OBJECT_STORAGE_REGION=us-east-1'; \
		printf '%s\n' 'PROBLEM_OBJECT_STORAGE_BUCKET=problem-test-data'; \
		printf '%s\n' 'PROBLEM_OBJECT_STORAGE_ACCESS_KEY=localadmin'; \
		printf '%s\n' "PROBLEM_OBJECT_STORAGE_SECRET_KEY=$$minio_password"; \
		printf '%s\n' "PROBLEM_INTERNAL_SUBMIT_TOKEN=$$submit_service_token"; \
		printf '%s\n' "PROBLEM_INTERNAL_JUDGE_TOKEN=$$judge_service_token"; \
		printf '%s\n' "PROBLEM_INTERNAL_CONTEST_TOKEN=$$problem_contest_service_token"; \
		printf '%s\n' "CONTEST_MYSQL_DSN='go_coj:$$mysql_password@tcp(127.0.0.1:3307)/go_coj?charset=utf8mb4&parseTime=true&loc=UTC'"; \
		printf '%s\n' "CONTEST_INTERNAL_PROBLEM_TOKEN=$$contest_problem_service_token"; \
		printf '%s\n' "CONTEST_INTERNAL_SUBMIT_TOKEN=$$contest_submit_service_token"; \
		printf '%s\n' "SUBMIT_MYSQL_DSN='go_coj:$$mysql_password@tcp(127.0.0.1:3307)/go_coj?charset=utf8mb4&parseTime=true&loc=UTC'"; \
		printf '%s\n' "SUBMIT_REDIS_PASSWORD=$$redis_password"; \
		printf '%s\n' "SUBMIT_INTERNAL_JUDGE_TOKEN=$$submit_judge_service_token"; \
	} > .env
	@echo 'Created .env with random local-development secrets.'

infra-env: .env
	@"$(DEV_ENV)" sync

env-check: infra-env
	@"$(DEV_ENV)" check

infra-up: infra-env
	$(COMPOSE) up -d --wait --wait-timeout 120 mysql redis etcd minio
	$(COMPOSE) run --rm mysql-migrate
	$(COMPOSE) run --rm minio-init

infra-migrate: infra-env
	$(COMPOSE) up -d --wait --wait-timeout 120 mysql
	$(COMPOSE) run --rm mysql-migrate

infra-down:
	$(COMPOSE) down

infra-reset:
	@printf '%s\n' 'Deleting MySQL, Redis, etcd and MinIO local data volumes...'
	$(COMPOSE) down --volumes --remove-orphans

infra-logs:
	$(COMPOSE) logs --follow

infra-ps:
	$(COMPOSE) ps

mysql-shell: .env
	@set -a; . ./.env; set +a; \
	$(COMPOSE) exec mysql mysql -u"$$MYSQL_USER" -p"$$MYSQL_PASSWORD" "$$MYSQL_DATABASE"

redis-shell: .env
	@set -a; . ./.env; set +a; \
	$(COMPOSE) exec redis redis-cli -a "$$REDIS_PASSWORD" --no-auth-warning

run-auth: infra-up
	@set -a; . ./.env; set +a; \
	"$(GO)" run ./services/grpc/auth -f services/grpc/auth/etc/auth.yaml

run-problem: infra-up
	@set -a; . ./.env; set +a; \
	"$(GO)" run ./services/grpc/problem -f services/grpc/problem/etc/problem.yaml

run-contest: infra-up
	@set -a; . ./.env; set +a; \
	"$(GO)" run ./services/grpc/contest -f services/grpc/contest/etc/contest.yaml

run-submit: infra-up
	@set -a; . ./.env; set +a; \
	"$(GO)" run ./services/grpc/submit -f services/grpc/submit/etc/submit.yaml

run-app: infra-up
	@set -a; . ./.env; set +a; \
	"$(GO)" run ./cmd/app -f cmd/app/etc/app-api.yaml

run-admin: infra-up
	@set -a; . ./.env; set +a; \
	"$(GO)" run ./cmd/admin -f cmd/admin/etc/admin-api.yaml

services-build: tools-services infra-env
	@GO="$(GO)" RUNTIME_DIR="$(abspath $(RUNTIME_DIR))" "$(DEV_SERVICES)" build

services-start: tools-services infra-env services-build
	@GO="$(GO)" RUNTIME_DIR="$(abspath $(RUNTIME_DIR))" "$(DEV_SERVICES)" start

services-stop:
	@RUNTIME_DIR="$(abspath $(RUNTIME_DIR))" "$(DEV_SERVICES)" stop

services-restart: tools-services infra-env services-build
	@GO="$(GO)" RUNTIME_DIR="$(abspath $(RUNTIME_DIR))" "$(DEV_SERVICES)" restart

services-status:
	@RUNTIME_DIR="$(abspath $(RUNTIME_DIR))" "$(DEV_SERVICES)" status

services-logs:
	@RUNTIME_DIR="$(abspath $(RUNTIME_DIR))" "$(DEV_SERVICES)" logs

connectivity-infra: infra-env
	@RUNTIME_DIR="$(abspath $(RUNTIME_DIR))" "$(DEV_SERVICES)" connectivity-infra

connectivity-services:
	@RUNTIME_DIR="$(abspath $(RUNTIME_DIR))" "$(DEV_SERVICES)" connectivity-services

connectivity: infra-env
	@RUNTIME_DIR="$(abspath $(RUNTIME_DIR))" "$(DEV_SERVICES)" connectivity

smoke: infra-env
	@RUNTIME_DIR="$(abspath $(RUNTIME_DIR))" "$(DEV_SERVICES)" smoke

up:
	@$(MAKE) infra-up
	@$(MAKE) services-restart
	@$(MAKE) smoke

services-up: up
dev-up: up
restart: up

stop: services-stop

down:
	@$(MAKE) services-stop
	@$(MAKE) infra-down

dev-down: down

generate: api rpc

api: tools $(addprefix api-,$(API_SERVICES))

$(addprefix api-,$(API_SERVICES)): api-%: tools
	@api_file="cmd/$*/desc/$*.api"; \
	if [ ! -s "$$api_file" ]; then \
		echo "error: $$api_file is missing or empty" >&2; \
		exit 1; \
	fi; \
	echo "Generating HTTP service: $*"; \
	"$(GOCTL)" api go \
		--api "$$api_file" \
		--dir "cmd/$*" \
		--home tpls \
		--style "$(GOCTL_STYLE)"

rpc: tools-rpc $(addprefix rpc-,$(RPC_SERVICES))

$(addprefix rpc-,$(RPC_SERVICES)): rpc-%: tools-rpc
	@service_dir="$(RPC_ROOT)/$*"; \
	proto_path="$*.proto"; \
	if [ -f "$$service_dir/proto/$*.proto" ]; then \
		proto_path="proto/$*.proto"; \
	fi; \
	proto_file="$$service_dir/$$proto_path"; \
	if [ ! -s "$$proto_file" ]; then \
		echo "error: $$proto_file is missing or empty" >&2; \
		exit 1; \
	fi; \
	multiple_flag=""; \
	case " $(RPC_MULTIPLE_SERVICES) " in *" $* "*) multiple_flag="--multiple" ;; esac; \
	echo "Generating gRPC service: $*"; \
	cd "$$service_dir"; \
	"$(GOCTL)" rpc protoc -I . -I "$(PROTOBUF_INCLUDE)" "$$proto_path" \
		--go_out=. \
		--go-grpc_out=. \
		--zrpc_out=. \
		--style "$(GOCTL_STYLE)" \
		$$multiple_flag

# Compatibility alias using the <service>-rpc naming style.
problem-rpc: rpc-problem

fmt:
	@test -f go.mod || { echo 'error: go.mod does not exist' >&2; exit 1; }
	"$(GO)" fmt ./...

vet:
	@test -f go.mod || { echo 'error: go.mod does not exist' >&2; exit 1; }
	"$(GO)" vet ./...

test:
	@test -f go.mod || { echo 'error: go.mod does not exist' >&2; exit 1; }
	"$(GO)" test -count="$(TEST_COUNT)" ./...

test-http:
	@test -f go.mod || { echo 'error: go.mod does not exist' >&2; exit 1; }
	"$(GO)" test -count="$(TEST_COUNT)" ./cmd/...

test-grpc:
	@test -f go.mod || { echo 'error: go.mod does not exist' >&2; exit 1; }
	"$(GO)" test -count="$(TEST_COUNT)" ./services/grpc/...

test-integration: infra-up
	@GO="$(GO)" TEST_COUNT="$(TEST_COUNT)" RUNTIME_DIR="$(abspath $(RUNTIME_DIR))" "$(DEV_SERVICES)" test-integration

test-services:
	@$(MAKE) test-http
	@$(MAKE) test-grpc
	@$(MAKE) test-integration

test-all:
	@$(MAKE) vet
	@$(MAKE) test-services
	@$(MAKE) build

coverage:
	@test -f go.mod || { echo 'error: go.mod does not exist' >&2; exit 1; }
	"$(GO)" test -coverprofile=coverage.out ./...

build:
	@test -f go.mod || { echo 'error: go.mod does not exist' >&2; exit 1; }
	"$(GO)" build ./...

tidy:
	@test -f go.mod || { echo 'error: go.mod does not exist' >&2; exit 1; }
	"$(GO)" mod tidy

clean: services-stop
	rm -rf bin coverage.out "$(RUNTIME_DIR)"
