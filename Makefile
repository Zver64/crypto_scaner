.DEFAULT_GOAL := help

BACKEND_BINARY ?= backend/bin/crypto-scanner

.PHONY: help prepare dev devbackend buildbackend db-up db-down migrate-up migrate-down

help:
	@printf '%s\n' \
		'make dev          Start the complete Docker development stack in watch mode' \
		'make prepare      Prepare the backend' \
		'make devbackend   Start the backend' \
		'make db-up        Start PostgreSQL' \
		'make db-down      Stop PostgreSQL' \
		'make migrate-up   Apply database migrations' \
		'make migrate-down Roll back one database migration'

prepare:
	go -C backend mod download

dev:
	docker compose up --watch

devbackend:
	cd backend && air

# Internal target used by Air during backend development.
buildbackend:
	mkdir -p $(dir $(BACKEND_BINARY))
	go -C backend build -o ../$(BACKEND_BINARY) ./cmd/crypto-scanner

db-up:
	docker compose up -d --wait

db-down:
	docker compose down

migrate-up:
	go -C backend run ./cmd/migrate up

migrate-down:
	go -C backend run ./cmd/migrate down
