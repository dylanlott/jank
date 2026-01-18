.PHONY: help build run run-sqlite dev test fmt tidy clean backup-db

BINARY_NAME := jank
BIN_DIR := bin
BIN_PATH := $(BIN_DIR)/$(BINARY_NAME)
AIR_BIN := $(shell go env GOPATH)/bin/air

help:
	@printf "Targets:\n"
	@printf "  build        Build the binary into %s\n" "$(BIN_PATH)"
	@printf "  run          Run the server (expects DB env vars set)\n"
	@printf "  run-sqlite   Run the server with SQLite env vars\n"
	@printf "  dev          Run hot-reloading dev server\n"
	@printf "  test         Run Go tests\n"
	@printf "  fmt          Format Go sources\n"
	@printf "  tidy         Tidy Go modules\n"
	@printf "  backup-db    Backup the configured database\n"
	@printf "  clean        Remove build artifacts\n"

build:
	@mkdir -p "$(BIN_DIR)"
	go build -o "$(BIN_PATH)" .

run:
	JANK_DB_DRIVER=sqlite JANK_DB_DSN=./sqlite.db go run .

run-sqlite:
	JANK_DB_DRIVER=sqlite JANK_DB_DSN=./sqlite.db go run .

dev:
	@test -x "$(AIR_BIN)" || { echo "air not found; installing..."; go install github.com/air-verse/air@latest; }
	"$(AIR_BIN)" -c .air.toml

test:
	go test ./...

fmt:
	gofmt -w .

tidy:
	go mod tidy

backup-db:
	bash ./scripts/backup_db.sh

clean:
	rm -rf "$(BIN_DIR)"
