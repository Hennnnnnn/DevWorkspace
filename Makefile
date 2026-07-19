.PHONY: build build-client build-server test tidy up down logs run-server

build: build-client build-server

BINARY_SUFFIX =
ifeq ($(OS),Windows_NT)
    BINARY_SUFFIX = .exe
endif

# Override DEFAULT_SERVER_URL at build time, e.g.:
#   make build DEFAULT_SERVER_URL=https://devworkspace.onrender.com
DEFAULT_SERVER_URL ?= http://localhost:8080
LDFLAGS = -X github.com/Hennnnnnn/DevWorkspace/internal/client/config.DefaultServerURL=$(DEFAULT_SERVER_URL) -X github.com/Hennnnnnn/DevWorkspace/internal/client/commands.Version=$(shell git describe --tags --always 2>/dev/null || echo dev)

build-client:
	go build -ldflags "$(LDFLAGS)" -o bin/devsync$(BINARY_SUFFIX) ./cmd/devsync

build-server:
	go build -o bin/devsync-server$(BINARY_SUFFIX) ./cmd/devsync-server

test:
	go test ./...

tidy:
	go mod tidy

up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f server

# Run server locally against dockerized Postgres (compose db must be up).
run-server:
	DEVSYNC_DATABASE_URL=postgres://devsync:devsync@localhost:5432/devsync?sslmode=disable \
	go run ./cmd/devsync-server
