BINARY := bin/crypto-scanner

.PHONY: build test vet generate sqlc-version tidy dependencies check clean

build:
	mkdir -p $(dir $(BINARY))
	go build -o $(BINARY) ./cmd/crypto-scanner

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

clean:
	go clean
	rm -f $(BINARY)
