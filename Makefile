.PHONY: prepare dev migrate-up migrate-down

prepare:
	go -C backend mod download
	npm -C frontend ci

dev:
	docker compose up --watch

migrate-up:
	go -C backend run ./cmd/migrate up

migrate-down:
	go -C backend run ./cmd/migrate down
