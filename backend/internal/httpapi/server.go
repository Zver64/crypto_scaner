package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

const (
	readHeaderTimeout = 5 * time.Second
	idleTimeout       = 60 * time.Second
)

// Serve runs an HTTP server until the process context is cancelled. It closes
// the listener immediately, drains active requests up to shutdownTimeout, and
// force-closes remaining connections when the deadline expires.
func Serve(
	ctx context.Context,
	listener net.Listener,
	handler http.Handler,
	logger *slog.Logger,
	shutdownTimeout time.Duration,
) error {
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}
	serveResult := make(chan error, 1)
	go func() {
		serveResult <- server.Serve(listener)
	}()

	select {
	case err := <-serveResult:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	case <-ctx.Done():
	}

	deadline := time.Now().Add(shutdownTimeout)
	shutdownCtx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	forceResult := make(chan error, 1)
	forceTimer := time.AfterFunc(time.Until(deadline), func() {
		forceResult <- server.Close()
	})
	shutdownErr := server.Shutdown(shutdownCtx)
	if !forceTimer.Stop() {
		closeErr := <-forceResult
		if shutdownErr == nil {
			shutdownErr = context.DeadlineExceeded
		}
		if closeErr != nil {
			return fmt.Errorf("HTTP shutdown deadline exceeded: %w (force close: %v)", shutdownErr, closeErr)
		}
		return fmt.Errorf("HTTP shutdown deadline exceeded: %w", shutdownErr)
	}
	if shutdownErr != nil {
		return fmt.Errorf("shutdown HTTP: %w", shutdownErr)
	}

	if err := <-serveResult; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP: %w", err)
	}
	return nil
}
