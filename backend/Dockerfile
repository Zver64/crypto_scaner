FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ ./cmd/...

# Distroless static ships CA certificates and time zone data, runs as a
# non-root user, and has no shell; healthchecks use `crypto-scanner healthcheck`.
FROM gcr.io/distroless/static-debian13:nonroot

COPY --from=build /out/crypto-scanner /usr/local/bin/crypto-scanner
COPY --from=build /out/migrate /usr/local/bin/migrate

EXPOSE 8080

CMD ["/usr/local/bin/crypto-scanner"]
