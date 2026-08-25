.PHONY: prepare check dev migrate-up migrate-down

prepare:
	go -C backend mod download
	npm -C frontend ci
	npm -C frontend exec -- lefthook install

check:
	test -z "$$(gofmt -l $$(git ls-files 'backend/*.go'))"
	go -C backend vet ./...
	go -C backend test ./...
	npm -C frontend run quality
	npm -C frontend run test

dev:
	docker compose up --watch

migrate-up:
	go -C backend run ./cmd/migrate up

migrate-down:
	go -C backend run ./cmd/migrate down
