.PHONY: help build run run-sqlite test fmt tidy clean

BINARY_NAME := jank
BIN_DIR := bin
BIN_PATH := $(BIN_DIR)/$(BINARY_NAME)

help:
	@printf "Targets:\n"
	@printf "  build        Build the binary into %s\n" "$(BIN_PATH)"
	@printf "  run          Run the server (expects DB env vars set)\n"
	@printf "  run-sqlite   Run the server with SQLite env vars\n"
	@printf "  test         Run Go tests\n"
	@printf "  fmt          Format Go sources\n"
	@printf "  tidy         Tidy Go modules\n"
	@printf "  clean        Remove build artifacts\n"

build:
	@mkdir -p "$(BIN_DIR)"
	go build -o "$(BIN_PATH)" .

run:
	JANK_DB_DRIVER=sqlite JANK_DB_DSN=./sqlite.db go run .

run-sqlite:
	JANK_DB_DRIVER=sqlite JANK_DB_DSN=./sqlite.db go run .

test:
	go test ./...

fmt:
	gofmt -w .

tidy:
	go mod tidy

clean:
	rm -rf "$(BIN_DIR)"
