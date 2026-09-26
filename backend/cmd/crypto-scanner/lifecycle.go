package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"crypto-scanner/internal/httpapi"
)

type runner interface {
	Run(context.Context) error
}

// service is a named background runner owned by the process lifecycle.
type service struct {
	name string
	runner
}

// runServices serves HTTP and, once it accepts connections, starts services.
// The first of them to stop, or the parent context, stops all the others and
// every one is awaited before returning.
func runServices(ctx context.Context, listener net.Listener, handler http.Handler, logger *slog.Logger, shutdownTimeout time.Duration, services ...service) error {
	httpCtx, stopHTTP := context.WithCancel(context.Background())
	defer stopHTTP()
	httpReady := make(chan struct{})
	listener = &acceptSignalingListener{Listener: listener, ready: httpReady}
	httpResult := make(chan error, 1)
	go func() { httpResult <- httpapi.Serve(httpCtx, listener, handler, logger, shutdownTimeout) }()

	select {
	case <-httpReady:
	case err := <-httpResult:
		return err
	case <-ctx.Done():
		stopHTTP()
		return <-httpResult
	}

	type stopped struct {
		index int
		err   error
	}
	servicesCtx, stopServices := context.WithCancel(context.Background())
	defer stopServices()
	results := make(chan stopped, len(services))
	for index, service := range services {
		go func() { results <- stopped{index: index, err: service.Run(servicesCtx)} }()
	}

	errs := make([]error, len(services))
	pending := len(services)
	first := -1 // index of the service that stopped first
	var httpErr error
	httpDone := false
	select {
	case <-ctx.Done():
	case httpErr = <-httpResult:
		httpDone = true
	case result := <-results:
		pending--
		errs[result.index] = result.err
		first = result.index
	}

	stopServices()
	stopHTTP()
	for ; pending > 0; pending-- {
		result := <-results
		errs[result.index] = result.err
	}
	if !httpDone {
		httpErr = <-httpResult
	}

	switch {
	case httpDone:
		return httpErr
	case first >= 0 && errs[first] != nil:
		return fmt.Errorf("run %s: %w", services[first].name, errs[first])
	case first >= 0:
		return fmt.Errorf("%s stopped unexpectedly", services[first].name)
	}
	for index, err := range errs {
		if err != nil {
			return fmt.Errorf("stop %s: %w", services[index].name, err)
		}
	}
	return httpErr
}

type acceptSignalingListener struct {
	net.Listener
	once  sync.Once
	ready chan<- struct{}
}

func (listener *acceptSignalingListener) Accept() (net.Conn, error) {
	listener.once.Do(func() { close(listener.ready) })
	return listener.Listener.Accept()
}
