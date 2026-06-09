SHELL := /bin/sh

APP_NAME := cccad-sketches
IMAGE := dnonakolesax/cccad-sketches:0.0.1
BIN_DIR := bin
API_BIN := $(BIN_DIR)/$(APP_NAME)
CONFIGS_DIR := ./configs
MIGRATIONS_DIR := ./migrations

PROTOC := protoc
GO_PACKAGES := ./...
PROTO_FILES := internal/proto/auth/v1/auth.proto proto/solver/v1/sketch_solver.proto proto/3d/v1/geometry_kernel.proto
GEOMETRY_PROTO_GO_PACKAGE := github.com/dnonakolesax/cccad-locks/internal/proto/geometry/v1
EASYJSON_PACKAGES := ./internal/model

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show available targets
	@awk 'BEGIN {FS = ":.*##"; printf "Targets:\n"} /^[a-zA-Z0-9_.-]+:.*##/ {printf "  %-18s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: fmt
fmt: ## Format Go sources
	gofmt -w $$(find . -name '*.go' -not -path './.git/*')

.PHONY: tidy
tidy: ## Tidy Go modules
	go mod tidy

.PHONY: test
test: ## Run all Go tests
	go test $(GO_PACKAGES)

.PHONY: build
build: ## Build the API binary
	mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 go build -o $(API_BIN) ./cmd/api

.PHONY: run
run: ## Run the API locally
	go run ./cmd/api -configs $(CONFIGS_DIR)

.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run

.PHONY: proto
proto: ## Regenerate protobuf Go files
	$(PROTOC) -I . --go_out=. --go_opt=module=github.com/dnonakolesax/cccad-locks --go_opt=Mproto/3d/v1/geometry_kernel.proto=$(GEOMETRY_PROTO_GO_PACKAGE) --go-grpc_out=. --go-grpc_opt=module=github.com/dnonakolesax/cccad-locks --go-grpc_opt=Mproto/3d/v1/geometry_kernel.proto=$(GEOMETRY_PROTO_GO_PACKAGE) $(PROTO_FILES)

.PHONY: easyjson
easyjson: ## Regenerate easyjson Go files
	go generate $(EASYJSON_PACKAGES)

.PHONY: compose-config
compose-config: ## Validate docker-compose.yml
	docker compose config

.PHONY: docker-build
docker-build: ## Build the Docker image with compose
	docker compose build sketcher

.PHONY: docker-up
docker-up: ## Start services with compose
	docker compose up

.PHONY: docker-down
docker-down: ## Stop services with compose
	docker compose down

.PHONY: migrate-up
migrate-up: ## Apply database migrations
	go run ./cmd/migrate -configs $(CONFIGS_DIR) -dir $(MIGRATIONS_DIR) up

.PHONY: migrate-down
migrate-down: ## Roll back one database migration
	go run ./cmd/migrate -configs $(CONFIGS_DIR) -dir $(MIGRATIONS_DIR) down

.PHONY: check
check: fmt tidy proto easyjson test compose-config ## Run local validation
