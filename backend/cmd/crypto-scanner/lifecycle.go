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

type scheduledService interface {
	Run(context.Context) error
}

func runServices(
	ctx context.Context,
	listener net.Listener,
	handler http.Handler,
	scheduler scheduledService,
	botService scheduledService,
	logger *slog.Logger,
	shutdownTimeout time.Duration,
) error {
	httpCtx, stopHTTP := context.WithCancel(context.Background())
	httpReady := make(chan struct{})
	listener = &acceptSignalingListener{Listener: listener, ready: httpReady}
	httpResult := make(chan error, 1)
	go func() { httpResult <- httpapi.Serve(httpCtx, listener, handler, logger, shutdownTimeout) }()

	select {
	case <-httpReady:
	case err := <-httpResult:
		stopHTTP()
		return err
	case <-ctx.Done():
		stopHTTP()
		return <-httpResult
	}

	schedulerCtx, stopScheduler := context.WithCancel(context.Background())
	schedulerResult := make(chan error, 1)
	go func() { schedulerResult <- scheduler.Run(schedulerCtx) }()
	botCtx, stopBot := context.WithCancel(context.Background())
	botResult := make(chan error, 1)
	go func() { botResult <- botService.Run(botCtx) }()

	type stopReason uint8
	const (
		parentCancelled stopReason = iota
		httpStopped
		schedulerStopped
		botStopped
	)

	reason := parentCancelled
	var schedulerErr, botErr, httpErr error
	awaitScheduler, awaitBot, awaitHTTP := schedulerResult, botResult, httpResult
	select {
	case <-ctx.Done():
	case httpErr = <-httpResult:
		awaitHTTP = nil
		reason = httpStopped
	case schedulerErr = <-schedulerResult:
		awaitScheduler = nil
		reason = schedulerStopped
	case botErr = <-botResult:
		awaitBot = nil
		reason = botStopped
	}

	stoppedSchedulerErr, stoppedBotErr, stoppedHTTPErr := stopAndWaitServices(stopScheduler, stopBot, stopHTTP, awaitScheduler, awaitBot, awaitHTTP)
	if awaitScheduler != nil {
		schedulerErr = stoppedSchedulerErr
	}
	if awaitBot != nil {
		botErr = stoppedBotErr
	}
	if awaitHTTP != nil {
		httpErr = stoppedHTTPErr
	}

	switch reason {
	case parentCancelled:
		return parentCancellationResult(schedulerErr, botErr, httpErr)
	case httpStopped:
		return httpErr
	case schedulerStopped:
		return schedulerResultError(schedulerErr)
	case botStopped:
		return botResultError(botErr)
	default:
		panic("unknown service stop reason")
	}
}

func stopAndWaitServices(
	stopScheduler, stopBot, stopHTTP context.CancelFunc,
	schedulerResult, botResult, httpResult <-chan error,
) (schedulerErr, botErr, httpErr error) {
	stopScheduler()
	stopBot()
	stopHTTP()
	if schedulerResult != nil {
		schedulerErr = <-schedulerResult
	}
	if botResult != nil {
		botErr = <-botResult
	}
	if httpResult != nil {
		httpErr = <-httpResult
	}
	return schedulerErr, botErr, httpErr
}

func parentCancellationResult(schedulerErr, botErr, httpErr error) error {
	if schedulerErr != nil {
		return fmt.Errorf("stop market scheduler: %w", schedulerErr)
	}
	if botErr != nil {
		return fmt.Errorf("stop Telegram bot: %w", botErr)
	}
	return httpErr
}

func schedulerResultError(err error) error {
	if err != nil {
		return fmt.Errorf("run market scheduler: %w", err)
	}
	return fmt.Errorf("market scheduler stopped unexpectedly")
}

func botResultError(err error) error {
	if err != nil {
		return fmt.Errorf("run Telegram bot: %w", err)
	}
	return fmt.Errorf("Telegram bot stopped unexpectedly")
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
