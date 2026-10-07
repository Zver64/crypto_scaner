.PHONY: prepare check generate generate-backend generate-frontend generate-sqlc migrate-up migrate-down install-cli uninstall-cli

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
