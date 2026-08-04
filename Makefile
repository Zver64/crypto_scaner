BINARY := bin/crypto-scanner
BOOTSTRAP_BINARY := bin/bootstrap-admin

.PHONY: build test vet generate sqlc-version tidy dependencies check migrate-up migrate-down bootstrap-admin clean

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

clean:
	go clean
	rm -f $(BINARY) $(BOOTSTRAP_BINARY)
