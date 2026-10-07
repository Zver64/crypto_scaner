FROM golang:1.26-alpine AS build

# TA-Lib is compiled from C sources embedded in github.com/TA-Lib/ta-lib-cgo.
RUN apk add --no-cache build-base

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Compiling the embedded TA-Lib C sources is the slowest part of the build.
# Keep its Go build cache entries in an image layer, which depends only on
# go.mod and go.sum and is therefore reused by registry and CI layer caches;
# BuildKit cache mounts are not exported to them. The flags must match the
# final build, or the cache entries do not apply.
RUN --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=1 GOCACHE=/talib-cache go build -trimpath github.com/TA-Lib/ta-lib-cgo

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    cp -a /talib-cache/. /root/.cache/go-build/ && \
    CGO_ENABLED=1 go build -trimpath \
        -ldflags="-s -w -linkmode external -extldflags -static" -o /out/ ./cmd/crypto-scanner ./cmd/migrate

# The binaries are linked statically against musl, so they need no libc.
# Distroless static ships CA certificates and time zone data, runs as a
# non-root user, and has no shell; healthchecks use `crypto-scanner healthcheck`.
FROM gcr.io/distroless/static-debian13:nonroot

COPY --from=build /out/crypto-scanner /usr/local/bin/crypto-scanner
COPY --from=build /out/migrate /usr/local/bin/migrate

EXPOSE 8080

CMD ["/usr/local/bin/crypto-scanner"]
