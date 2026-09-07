.PHONY: prepare check migrate-up migrate-down

prepare:
	go -C backend mod download
	npm -C frontend ci
	go -C .tools tool lefthook install

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
