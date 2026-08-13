BINARY := bin/crypto-scanner
MIGRATE_BINARY := bin/migrate

-include ../.env

POSTGRES_USER ?= scanner
POSTGRES_PASSWORD ?= scanner
POSTGRES_DB ?= scanner
POSTGRES_PORT ?= 5432
DATABASE_URL ?= postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@127.0.0.1:$(POSTGRES_PORT)/$(POSTGRES_DB)?sslmode=disable
SHUTDOWN_TIMEOUT ?= 15s

export DATABASE_URL TELEGRAM_BOT_TOKEN ADMIN_TELEGRAM_ID
export HTTP_ADDRESS LOG_LEVEL TELEGRAM_INIT_DATA_MAX_AGE SYNC_WORKERS SYNC_RETRY_ATTEMPTS SHUTDOWN_TIMEOUT

.PHONY: build build-all build-migrate test vet generate tidy check migrate-up migrate-down bootstrap-admin run dev clean

build:
	mkdir -p $(dir $(BINARY))
	go build -o $(BINARY) ./cmd/crypto-scanner

build-migrate:
	mkdir -p $(dir $(MIGRATE_BINARY))
	go build -o $(MIGRATE_BINARY) ./cmd/migrate

build-all: build build-migrate

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
	@PGPASSWORD="$(POSTGRES_PASSWORD)" psql \
		--no-psqlrc \
		--host=127.0.0.1 \
		--port="$(POSTGRES_PORT)" \
		--username="$(POSTGRES_USER)" \
		--dbname="$(POSTGRES_DB)" \
		--set=admin_telegram_id="$(ADMIN_TELEGRAM_ID)" \
		--file=scripts/bootstrap-admin.sql

run:
	go run ./cmd/crypto-scanner

dev:
	air --build.kill_delay "$(SHUTDOWN_TIMEOUT)"

clean:
	go clean
	rm -f $(BINARY) $(MIGRATE_BINARY)
