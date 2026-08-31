SHELL := /bin/sh
.DEFAULT_GOAL := help

GO ?= go
GOCTL ?= goctl
GOCTL_STYLE ?= go_zero
COMPOSE ?= docker compose

API_SERVICES := admin app
RPC_ROOT := services/grpc
RPC_PROTO_FILES := $(wildcard $(RPC_ROOT)/*/*.proto $(RPC_ROOT)/*/proto/*.proto)
RPC_SERVICES := $(sort $(notdir $(basename $(RPC_PROTO_FILES))))
RPC_MULTIPLE_SERVICES := auth
PROTOBUF_INCLUDE ?= $(shell $(GO) list -m -f '{{.Dir}}/third_party' github.com/google/gnostic-models 2>/dev/null)

.PHONY: help tools tools-rpc generate api rpc \
	$(addprefix api-,$(API_SERVICES)) \
	$(addprefix rpc-,$(RPC_SERVICES)) \
	problem-rpc \
	fmt vet test coverage build tidy clean \
	tools-infra infra-env infra-up infra-migrate infra-down infra-reset infra-logs infra-ps \
	mysql-shell redis-shell run-auth run-problem run-contest

help:
	@printf '%s\n' \
		'Usage: make <target>' \
		'' \
		'Code generation:' \
		'  api              Generate all HTTP services' \
		'  api-<service>    Generate one HTTP service (admin or app)' \
		'  rpc              Generate all gRPC services' \
		'  rpc-<service>    Generate one gRPC service ($(RPC_SERVICES))' \
		'  problem-rpc      Generate the problem gRPC service' \
		'  generate         Generate all HTTP and gRPC services' \
		'' \
		'Go workflow:' \
		'  fmt              Format all Go packages' \
		'  vet              Run go vet' \
		'  test             Run unit tests' \
		'  coverage         Run tests and write coverage.out' \
		'  build            Compile all packages' \
		'  tidy             Update go.mod and go.sum' \
		'  clean            Remove local build and coverage artifacts' \
		'' \
		'Local infrastructure:' \
		'  infra-up         Start MySQL, Redis, etcd and MinIO and wait until healthy' \
		'  infra-migrate    Apply pending Auth, Problem and Contest MySQL migrations' \
		'  infra-down       Stop local infrastructure and keep its data' \
		'  infra-reset      Stop infrastructure and delete all local data' \
		'  infra-logs       Follow infrastructure logs' \
		'  infra-ps         Show infrastructure status' \
		'  mysql-shell      Open a MySQL shell as the application user' \
		'  redis-shell      Open an authenticated Redis CLI' \
		'  run-auth         Start infrastructure, then run the auth RPC service' \
		'  run-problem      Start infrastructure, then run the problem RPC service' \
		'  run-contest      Start infrastructure, then run the contest RPC service' \
		'' \
		'Checks:' \
		'  tools            Check Go and goctl' \
		'  tools-rpc        Check the protobuf toolchain' \
		'  tools-infra      Check Docker Compose and OpenSSL'

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
	} > .env
	@echo 'Created .env with random local-development secrets.'

infra-env: .env

infra-up: .env
	$(COMPOSE) up -d --wait --wait-timeout 120 mysql redis etcd minio
	$(COMPOSE) run --rm mysql-migrate
	$(COMPOSE) run --rm minio-init

infra-migrate: .env
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
	"$(GO)" test ./...

coverage:
	@test -f go.mod || { echo 'error: go.mod does not exist' >&2; exit 1; }
	"$(GO)" test -coverprofile=coverage.out ./...

build:
	@test -f go.mod || { echo 'error: go.mod does not exist' >&2; exit 1; }
	"$(GO)" build ./...

tidy:
	@test -f go.mod || { echo 'error: go.mod does not exist' >&2; exit 1; }
	"$(GO)" mod tidy

clean:
	rm -rf bin coverage.out
