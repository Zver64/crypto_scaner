.PHONY: prepare check test-db generate generate-backend generate-frontend generate-sqlc migrate-up migrate-down install-cli uninstall-cli

prepare:
	go -C backend mod download
	npm -C frontend ci
	go -C .tools tool lefthook install

generate: generate-backend generate-frontend generate-sqlc

generate-backend:
	go -C backend generate ./internal/httpapi/openapi ./internal/indicator/talib

generate-sqlc:
	go -C backend generate ./internal/storage/postgres

generate-frontend:
	npm -C frontend run generate-api
	npm -C frontend run generate-routes

check:
	test -z "$$(gofmt -l $$(find backend -type f -name '*.go'))"
	go -C backend vet ./...
	go -C backend test ./...
	npm -C frontend run quality
	npm -C frontend run test

# Runs the backend tests, including the database integration tests, against the
# scanner_test database of the running Compose PostgreSQL. The packages whose
# tests read CRYPTO_SCANNER_TEST_DATABASE_URL run one at a time, because the
# integration tests reset the schema; all other packages run in parallel first.
test-db:
	@docker compose ps --status running --services 2>/dev/null | grep -qx postgres || \
		{ echo "test-db: the Compose PostgreSQL is not running" >&2; exit 1; }
	@set -e; \
	db=scanner_test; \
	compose_env() { docker compose exec -T postgres printenv "$$1"; }; \
	if [ "$$(compose_env POSTGRES_DB)" = "$$db" ]; then echo "test-db: $$db is the application database" >&2; exit 1; fi; \
	PGUSER=$$(compose_env POSTGRES_USER); PGPASSWORD=$$(compose_env POSTGRES_PASSWORD); export PGUSER PGPASSWORD; \
	if [ -z "$$(docker compose exec -T postgres psql -U "$$PGUSER" -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname = '$$db'")" ]; then \
		docker compose exec -T postgres createdb -U "$$PGUSER" "$$db"; \
	fi; \
	db_packages=$$(grep -rl --include='*_test.go' CRYPTO_SCANNER_TEST_DATABASE_URL backend | xargs -n1 dirname | sort -u | sed 's|^backend|crypto-scanner|'); \
	go -C backend test -count=1 $$(go -C backend list ./... | grep -vxF "$$db_packages"); \
	CRYPTO_SCANNER_TEST_DATABASE_URL="postgres://$$(docker compose port postgres 5432)/$$db?sslmode=disable" \
	CRYPTO_SCANNER_TEST_DATABASE_RESET_OK=1 \
	go -C backend test -p 1 -count=1 $$db_packages

migrate-up:
	go -C backend run ./cmd/migrate up

migrate-down:
	go -C backend run ./cmd/migrate down

install-cli:
	go -C backend install ./cmd/scanner
	mkdir -p ~/.local/share/zsh/site-functions
	# Intentionally requires Go's install directory on PATH; keep this direct invocation.
	scanner completion zsh > ~/.local/share/zsh/site-functions/_scanner

uninstall-cli:
	go -C backend clean -i ./cmd/scanner
	rm -f ~/.local/share/zsh/site-functions/_scanner
	rm -rf ~/.config/scanner
