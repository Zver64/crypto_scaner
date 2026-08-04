package httpapi_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"crypto-scanner/internal/httpapi"
)

func TestServeStopsAcceptingNewRequestsAndDrainsActiveWork(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	handler := http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		response.WriteHeader(http.StatusOK)
	})
	ctx, cancel := context.WithCancel(context.Background())
	serveResult := make(chan error, 1)
	go func() {
		serveResult <- httpapi.Serve(ctx, listener, handler, slog.New(slog.NewTextHandler(io.Discard, nil)), time.Second)
	}()

	activeResult := make(chan error, 1)
	go func() {
		response, requestErr := http.Get("http://" + listener.Addr().String())
		if requestErr == nil {
			response.Body.Close()
			if response.StatusCode != http.StatusOK {
				requestErr = errors.New("active request did not complete successfully")
			}
		}
		activeResult <- requestErr
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("active request did not reach handler")
	}

	cancel()
	newClient := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
	defer newClient.CloseIdleConnections()
	deadline := time.Now().Add(time.Second)
	for {
		response, requestErr := newClient.Get("http://" + listener.Addr().String())
		if requestErr != nil {
			break
		}
		response.Body.Close()
		if time.Now().After(deadline) {
			t.Fatal("server continued accepting new requests during shutdown")
		}
		time.Sleep(5 * time.Millisecond)
	}

	close(release)
	if err := <-activeResult; err != nil {
		t.Fatalf("active request error = %v", err)
	}
	if err := <-serveResult; err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
}

func TestServeForcesShutdownAtTheConfiguredDeadline(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	started := make(chan struct{})
	handler := http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		close(started)
		<-request.Context().Done()
	})
	ctx, cancel := context.WithCancel(context.Background())
	serveResult := make(chan error, 1)
	go func() {
		serveResult <- httpapi.Serve(ctx, listener, handler, slog.New(slog.NewTextHandler(io.Discard, nil)), 30*time.Millisecond)
	}()
	go func() {
		response, requestErr := http.Get("http://" + listener.Addr().String())
		if requestErr == nil {
			response.Body.Close()
		}
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("request did not reach handler")
	}

	cancelledAt := time.Now()
	cancel()
	select {
	case err := <-serveResult:
		if err == nil || !strings.Contains(err.Error(), "shutdown deadline") {
			t.Fatalf("Serve() error = %v, want shutdown deadline error", err)
		}
		if elapsed := time.Since(cancelledAt); elapsed > 150*time.Millisecond {
			t.Fatalf("forced shutdown took %v, want no more than 150ms", elapsed)
		}
	case <-time.After(time.Second):
		t.Fatal("Serve() did not return after shutdown deadline")
	}
}
