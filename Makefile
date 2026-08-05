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

.PHONY: build test vet generate tidy check migrate-up migrate-down bootstrap-admin set-webhook run clean

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

tidy:
	go mod tidy

check: test vet build

migrate-up:
	go run ./cmd/migrate up

migrate-down:
	go run ./cmd/migrate down

bootstrap-admin:
	go run ./cmd/bootstrap-admin

set-webhook:
	go run ./cmd/crypto-scanner telegram set-webhook

run: migrate-up bootstrap-admin
	go run ./cmd/crypto-scanner

clean:
	go clean
	rm -f $(BINARY) $(BOOTSTRAP_BINARY)
