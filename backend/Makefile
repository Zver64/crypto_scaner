BINARY := bin/crypto-scanner

.PHONY: build run migrate-up migrate-down dev

build:
	mkdir -p $(dir $(BINARY))
	go build -o $(BINARY) ./cmd/crypto-scanner

run:
	go run ./cmd/crypto-scanner

migrate-up:
	go run ./cmd/migrate up

migrate-down:
	go run ./cmd/migrate down

dev:
	air
