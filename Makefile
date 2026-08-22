SHELL := /bin/sh
.DEFAULT_GOAL := help

GO ?= go
GOCTL ?= goctl
GOCTL_STYLE ?= go_zero

API_SERVICES := admin app
RPC_ROOT := services/grpc
RPC_PROTO_FILES := $(wildcard $(RPC_ROOT)/*/*.proto $(RPC_ROOT)/*/proto/*.proto)
RPC_SERVICES := $(sort $(notdir $(basename $(RPC_PROTO_FILES))))
RPC_MULTIPLE_SERVICES := auth

.PHONY: help tools tools-rpc generate api rpc \
	$(addprefix api-,$(API_SERVICES)) \
	$(addprefix rpc-,$(RPC_SERVICES)) \
	fmt vet test coverage build tidy clean

help:
	@printf '%s\n' \
		'Usage: make <target>' \
		'' \
		'Code generation:' \
		'  api              Generate all HTTP services' \
		'  api-<service>    Generate one HTTP service (admin or app)' \
		'  rpc              Generate all gRPC services' \
		'  rpc-<service>    Generate one gRPC service ($(RPC_SERVICES))' \
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
		'Checks:' \
		'  tools            Check Go and goctl' \
		'  tools-rpc        Check the protobuf toolchain'

tools:
	@command -v "$(GO)" >/dev/null || { echo 'error: go is not installed or not in PATH' >&2; exit 1; }
	@command -v "$(GOCTL)" >/dev/null || { echo 'error: goctl is not installed or not in PATH' >&2; exit 1; }

tools-rpc: tools
	@command -v protoc >/dev/null || { echo 'error: protoc is not installed or not in PATH' >&2; exit 1; }
	@command -v protoc-gen-go >/dev/null || { echo 'error: protoc-gen-go is not installed or not in PATH' >&2; exit 1; }
	@command -v protoc-gen-go-grpc >/dev/null || { echo 'error: protoc-gen-go-grpc is not installed or not in PATH' >&2; exit 1; }

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
	"$(GOCTL)" rpc protoc "$$proto_path" \
		--go_out=. \
		--go-grpc_out=. \
		--zrpc_out=. \
		--style "$(GOCTL_STYLE)" \
		$$multiple_flag

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
