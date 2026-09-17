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
	marketCapService scheduledService,
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
	marketCapCtx, stopMarketCap := context.WithCancel(context.Background())
	marketCapResult := make(chan error, 1)
	go func() { marketCapResult <- marketCapService.Run(marketCapCtx) }()

	type stopReason uint8
	const (
		parentCancelled stopReason = iota
		httpStopped
		schedulerStopped
		botStopped
		marketCapStopped
	)

	reason := parentCancelled
	var schedulerErr, botErr, marketCapErr, httpErr error
	awaitScheduler, awaitBot, awaitMarketCap, awaitHTTP := schedulerResult, botResult, marketCapResult, httpResult
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
	case marketCapErr = <-marketCapResult:
		awaitMarketCap = nil
		reason = marketCapStopped
	}

	stoppedSchedulerErr, stoppedBotErr, stoppedMarketCapErr, stoppedHTTPErr := stopAndWaitServices(stopScheduler, stopBot, stopMarketCap, stopHTTP, awaitScheduler, awaitBot, awaitMarketCap, awaitHTTP)
	if awaitScheduler != nil {
		schedulerErr = stoppedSchedulerErr
	}
	if awaitBot != nil {
		botErr = stoppedBotErr
	}
	if awaitMarketCap != nil {
		marketCapErr = stoppedMarketCapErr
	}
	if awaitHTTP != nil {
		httpErr = stoppedHTTPErr
	}

	switch reason {
	case parentCancelled:
		return parentCancellationResult(schedulerErr, botErr, marketCapErr, httpErr)
	case httpStopped:
		return httpErr
	case schedulerStopped:
		return schedulerResultError(schedulerErr)
	case botStopped:
		return botResultError(botErr)
	case marketCapStopped:
		return marketCapResultError(marketCapErr)
	default:
		panic("unknown service stop reason")
	}
}

func stopAndWaitServices(
	stopScheduler, stopBot, stopMarketCap, stopHTTP context.CancelFunc,
	schedulerResult, botResult, marketCapResult, httpResult <-chan error,
) (schedulerErr, botErr, marketCapErr, httpErr error) {
	stopScheduler()
	stopBot()
	stopMarketCap()
	stopHTTP()
	if schedulerResult != nil {
		schedulerErr = <-schedulerResult
	}
	if botResult != nil {
		botErr = <-botResult
	}
	if marketCapResult != nil {
		marketCapErr = <-marketCapResult
	}
	if httpResult != nil {
		httpErr = <-httpResult
	}
	return schedulerErr, botErr, marketCapErr, httpErr
}

func parentCancellationResult(schedulerErr, botErr, marketCapErr, httpErr error) error {
	if schedulerErr != nil {
		return fmt.Errorf("stop market scheduler: %w", schedulerErr)
	}
	if botErr != nil {
		return fmt.Errorf("stop Telegram bot: %w", botErr)
	}
	if marketCapErr != nil {
		return fmt.Errorf("stop market cap synchronizer: %w", marketCapErr)
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

func marketCapResultError(err error) error {
	if err != nil {
		return fmt.Errorf("run market cap synchronizer: %w", err)
	}
	return fmt.Errorf("market cap synchronizer stopped unexpectedly")
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
