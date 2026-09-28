package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

const healthcheckTimeout = 3 * time.Second

// healthcheck probes the liveness endpoint of a running server. Container
// images without a shell or HTTP client use it as their healthcheck command.
func healthcheck(ctx context.Context) error {
	address := os.Getenv("HTTP_ADDRESS")
	if address == "" {
		address = "127.0.0.1:8080"
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("HTTP_ADDRESS must be a valid host:port address")
	}
	if ip := net.ParseIP(host); host == "" || ip != nil && ip.IsUnspecified() {
		host = "127.0.0.1"
	}
	ctx, cancel := context.WithTimeout(ctx, healthcheckTimeout)
	defer cancel()
	url := "http://" + net.JoinHostPort(host, port) + "/health/live"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build liveness request: %w", err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("probe liveness: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("probe liveness: unexpected status %d", response.StatusCode)
	}
	return nil
}
