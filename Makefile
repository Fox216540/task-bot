.PHONY: help run test check-go deps

BOT_TOKEN ?=
DB_PATH ?= task_bot.sqlite
ALLOWED_USER_ID ?=
GO_BIN := $(shell if command -v go >/dev/null 2>&1; then command -v go; elif [ -x /usr/local/go/bin/go ]; then echo /usr/local/go/bin/go; fi)
GOCACHE ?= $(CURDIR)/.cache/go-build
GOMODCACHE ?= $(CURDIR)/.cache/go-mod
TEST_PATH ?= ./tests/...

help:
	@echo "Available targets:"
	@echo "  make run   - run bot (requires BOT_TOKEN and ALLOWED_USER_ID)"
	@echo "  make test  - run tests from TEST_PATH (default: ./tests/...)"
	@echo "  make deps  - download and tidy dependencies"

check-go:
	@if [ -z "$(GO_BIN)" ]; then \
		echo "Go not found."; \
		echo "Install Go or add it to PATH (example: export PATH=/usr/local/go/bin:\$$PATH)"; \
		exit 1; \
	fi

deps: check-go
	mkdir -p "$(GOCACHE)" "$(GOMODCACHE)"
	GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" "$(GO_BIN)" mod tidy

run: check-go
	@if [ -z "$(BOT_TOKEN)" ]; then \
		echo "BOT_TOKEN is required. Example:"; \
		echo "  BOT_TOKEN=123:ABC ALLOWED_USER_ID=123456789 make run"; \
		exit 1; \
	fi
	@if [ -z "$(ALLOWED_USER_ID)" ]; then \
		echo "ALLOWED_USER_ID is required. Example:"; \
		echo "  BOT_TOKEN=123:ABC ALLOWED_USER_ID=123456789 make run"; \
		exit 1; \
	fi
	mkdir -p "$(GOCACHE)" "$(GOMODCACHE)"
	GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" BOT_TOKEN="$(BOT_TOKEN)" DB_PATH="$(DB_PATH)" ALLOWED_USER_ID="$(ALLOWED_USER_ID)" "$(GO_BIN)" run ./cmd/task-bot

test: check-go
	mkdir -p "$(GOCACHE)" "$(GOMODCACHE)"
	GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" "$(GO_BIN)" test $(TEST_PATH)
