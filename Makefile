BINARY := bin/crypto-scanner
BOOTSTRAP_BINARY := bin/bootstrap-admin

-include .env

POSTGRES_USER ?= scanner
POSTGRES_PASSWORD ?= scanner
POSTGRES_DB ?= scanner
POSTGRES_PORT ?= 5432
DATABASE_URL ?= postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@127.0.0.1:$(POSTGRES_PORT)/$(POSTGRES_DB)?sslmode=disable

export DATABASE_URL TELEGRAM_BOT_TOKEN TELEGRAM_WEBHOOK_SECRET ADMIN_TELEGRAM_ID MINI_APP_URL PUBLIC_BASE_URL
export HTTP_ADDRESS LOG_LEVEL TELEGRAM_INIT_DATA_MAX_AGE SYNC_WORKERS SYNC_RETRY_ATTEMPTS SHUTDOWN_TIMEOUT

.PHONY: build test vet generate sqlc-version tidy dependencies check migrate-up migrate-down bootstrap-admin clean \
	dev-up dev-up-detached dev-db dev-run dev-stop dev-down dev-logs dev-reset

build:
	mkdir -p $(dir $(BINARY))
	go build -o $(BINARY) ./cmd/crypto-scanner
	go build -o $(BOOTSTRAP_BINARY) ./cmd/bootstrap-admin

test:
	go test ./...

vet:
	go vet ./...

generate:
	go generate ./...

sqlc-version:
	go tool sqlc version

tidy:
	go mod tidy

dependencies:
	go mod download

check: test vet build

migrate-up:
	go run ./cmd/migrate up

migrate-down:
	go run ./cmd/migrate down

bootstrap-admin:
	go run ./cmd/bootstrap-admin

dev-up:
	docker compose up --build

dev-up-detached:
	docker compose up --build --detach

dev-db:
	docker compose up --detach postgres

dev-run: migrate-up bootstrap-admin
	go run ./cmd/crypto-scanner

dev-stop:
	docker compose stop

dev-down:
	docker compose down --remove-orphans

dev-logs:
	docker compose logs --follow

dev-reset:
	docker compose down --volumes --remove-orphans

clean:
	go clean
	rm -f $(BINARY) $(BOOTSTRAP_BINARY)
