.PHONY: build build-client build-server test tidy up down logs run-server

build: build-client build-server

BINARY_SUFFIX =
ifeq ($(OS),Windows_NT)
    BINARY_SUFFIX = .exe
endif

build-client:
	go build -o bin/devsync$(BINARY_SUFFIX) ./cmd/devsync

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
