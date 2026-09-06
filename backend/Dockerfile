FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/crypto-scanner ./cmd/crypto-scanner \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

FROM alpine:3.23

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g 10001 scanner \
    && adduser -S -D -H -u 10001 -G scanner scanner

COPY --from=build /out/crypto-scanner /usr/local/bin/crypto-scanner
COPY --from=build /out/migrate /usr/local/bin/migrate

USER scanner
EXPOSE 8080

CMD ["/usr/local/bin/crypto-scanner"]
