package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestRunServicesParentCancellationPrefersSchedulerErrorAndWaitsForSiblings(t *testing.T) {
	schedulerErr := errors.New("scheduler stop failed")
	botErr := errors.New("bot stop failed")
	listener := newGatedListener()
	scheduler := newControlledService(schedulerErr)
	bot := newControlledService(botErr)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := startServices(ctx, listener, scheduler, bot)
	waitForStart(t, listener.accepted, "HTTP accept")
	waitForStart(t, scheduler.started, "scheduler")
	waitForStart(t, bot.started, "bot")

	cancel()
	waitForStart(t, scheduler.cancelled, "scheduler cancellation")
	waitForStart(t, bot.cancelled, "bot cancellation")
	assertNotReturned(t, result)
	close(scheduler.release)
	close(bot.release)

	err := waitForResult(t, result)
	if !errors.Is(err, schedulerErr) || err.Error() != "stop market scheduler: scheduler stop failed" {
		t.Fatalf("runServices() error = %v, want scheduler stop error", err)
	}
}

func TestRunServicesParentCancellationPrefersBotErrorOverHTTP(t *testing.T) {
	botErr := errors.New("bot stop failed")
	listener := newGatedListener()
	scheduler := newControlledService(nil)
	bot := newControlledService(botErr)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := startServices(ctx, listener, scheduler, bot)
	waitForServices(t, listener, scheduler, bot)

	cancel()
	waitForStart(t, scheduler.cancelled, "scheduler cancellation")
	waitForStart(t, bot.cancelled, "bot cancellation")
	close(scheduler.release)
	close(bot.release)

	err := waitForResult(t, result)
	if !errors.Is(err, botErr) || err.Error() != "stop Telegram bot: bot stop failed" {
		t.Fatalf("runServices() error = %v, want bot stop error", err)
	}
}

func TestRunServicesSchedulerFailureCancelsAndWaitsForSiblings(t *testing.T) {
	schedulerErr := errors.New("scheduler failed")
	listener := newGatedListener()
	scheduler := newControlledService(schedulerErr)
	bot := newControlledService(errors.New("bot stop failed"))
	result := startServices(context.Background(), listener, scheduler, bot)
	waitForServices(t, listener, scheduler, bot)

	close(scheduler.release)
	waitForStart(t, bot.cancelled, "bot cancellation")
	assertNotReturned(t, result)
	close(bot.release)

	err := waitForResult(t, result)
	if !errors.Is(err, schedulerErr) || err.Error() != "run market scheduler: scheduler failed" {
		t.Fatalf("runServices() error = %v, want scheduler failure", err)
	}
}

func TestRunServicesUnexpectedSchedulerStopCancelsSiblings(t *testing.T) {
	listener := newGatedListener()
	scheduler := newControlledService(nil)
	bot := newControlledService(nil)
	result := startServices(context.Background(), listener, scheduler, bot)
	waitForServices(t, listener, scheduler, bot)

	close(scheduler.release)
	waitForStart(t, bot.cancelled, "bot cancellation")
	close(bot.release)

	err := waitForResult(t, result)
	if err == nil || err.Error() != "market scheduler stopped unexpectedly" {
		t.Fatalf("runServices() error = %v, want unexpected scheduler stop", err)
	}
}

func TestRunServicesBotFailureCancelsSiblings(t *testing.T) {
	botErr := errors.New("bot failed")
	listener := newGatedListener()
	scheduler := newControlledService(errors.New("scheduler stop failed"))
	bot := newControlledService(botErr)
	result := startServices(context.Background(), listener, scheduler, bot)
	waitForServices(t, listener, scheduler, bot)

	close(bot.release)
	waitForStart(t, scheduler.cancelled, "scheduler cancellation")
	close(scheduler.release)

	err := waitForResult(t, result)
	if !errors.Is(err, botErr) || err.Error() != "run Telegram bot: bot failed" {
		t.Fatalf("runServices() error = %v, want bot failure", err)
	}
}

func TestRunServicesUnexpectedBotStopCancelsSiblings(t *testing.T) {
	listener := newGatedListener()
	scheduler := newControlledService(nil)
	bot := newControlledService(nil)
	result := startServices(context.Background(), listener, scheduler, bot)
	waitForServices(t, listener, scheduler, bot)

	close(bot.release)
	waitForStart(t, scheduler.cancelled, "scheduler cancellation")
	close(scheduler.release)

	err := waitForResult(t, result)
	if err == nil || err.Error() != "Telegram bot stopped unexpectedly" {
		t.Fatalf("runServices() error = %v, want unexpected bot stop", err)
	}
}

func TestRunServicesHTTPFailureCancelsSiblingsAndPreservesHTTPResult(t *testing.T) {
	httpErr := errors.New("HTTP failed")
	listener := newGatedListener()
	scheduler := newControlledService(errors.New("scheduler stop failed"))
	bot := newControlledService(errors.New("bot stop failed"))
	result := startServices(context.Background(), listener, scheduler, bot)
	waitForServices(t, listener, scheduler, bot)

	listener.result <- httpErr
	waitForStart(t, scheduler.cancelled, "scheduler cancellation")
	waitForStart(t, bot.cancelled, "bot cancellation")
	assertNotReturned(t, result)
	close(scheduler.release)
	close(bot.release)

	err := waitForResult(t, result)
	if !errors.Is(err, httpErr) || err.Error() != "serve HTTP: HTTP failed" {
		t.Fatalf("runServices() error = %v, want HTTP failure", err)
	}
}

func TestRunServicesWaitsForInFlightHTTPRequest(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	entered := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	scheduler := newControlledService(nil)
	bot := newControlledService(nil)
	// These services finish as soon as cancellation reaches them, leaving only
	// the active HTTP request to hold up the shutdown drain.
	result := make(chan error, 1)
	go func() {
		result <- runServices(ctx, listener, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			close(entered)
			<-release
			w.WriteHeader(http.StatusNoContent)
		}), scheduler, bot, slog.New(slog.NewTextHandler(io.Discard, nil)), 5*time.Second)
	}()
	waitForStart(t, scheduler.started, "scheduler")
	waitForStart(t, bot.started, "bot")
	client := &http.Client{Timeout: 5 * time.Second}
	requestResult := make(chan error, 1)
	go func() {
		response, err := client.Get("http://" + listener.Addr().String())
		if err == nil {
			_ = response.Body.Close()
		}
		requestResult <- err
	}()
	waitForStart(t, entered, "HTTP handler")
	cancel()
	waitForStart(t, scheduler.cancelled, "scheduler cancellation")
	waitForStart(t, bot.cancelled, "bot cancellation")
	close(scheduler.release)
	close(bot.release)
	assertNotReturned(t, result)
	unblock()
	if err := waitForResult(t, result); err != nil {
		t.Fatalf("runServices() error = %v", err)
	}
	if err := waitForResult(t, requestResult); err != nil {
		t.Fatalf("in-flight request failed: %v", err)
	}
}

func startServices(ctx context.Context, listener net.Listener, scheduler, bot scheduledService) <-chan error {
	result := make(chan error, 1)
	go func() {
		result <- runServices(ctx, listener, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), scheduler, bot, slog.New(slog.NewTextHandler(io.Discard, nil)), time.Second)
	}()
	return result
}

func waitForServices(t *testing.T, listener *gatedListener, scheduler, bot *controlledService) {
	t.Helper()
	waitForStart(t, listener.accepted, "HTTP accept")
	waitForStart(t, scheduler.started, "scheduler")
	waitForStart(t, bot.started, "bot")
}

func waitForStart(t *testing.T, signal <-chan struct{}, name string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", name)
	}
}

func assertNotReturned(t *testing.T, result <-chan error) {
	t.Helper()
	select {
	case err := <-result:
		t.Fatalf("runServices() returned before siblings stopped: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
}

func waitForResult(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for runServices")
		return nil
	}
}

type controlledService struct {
	result    error
	started   chan struct{}
	cancelled chan struct{}
	release   chan struct{}
}

func newControlledService(result error) *controlledService {
	return &controlledService{result: result, started: make(chan struct{}), cancelled: make(chan struct{}), release: make(chan struct{})}
}

func (service *controlledService) Run(ctx context.Context) error {
	close(service.started)
	select {
	case <-ctx.Done():
		close(service.cancelled)
	case <-service.release:
		return service.result
	}
	<-service.release
	return service.result
}

type gatedListener struct {
	accepted chan struct{}
	result   chan error
	closed   chan struct{}
	once     sync.Once
}

func newGatedListener() *gatedListener {
	return &gatedListener{accepted: make(chan struct{}), result: make(chan error, 1), closed: make(chan struct{})}
}

func (listener *gatedListener) Accept() (net.Conn, error) {
	listener.once.Do(func() { close(listener.accepted) })
	select {
	case err := <-listener.result:
		return nil, err
	case <-listener.closed:
		return nil, net.ErrClosed
	}
}

func (listener *gatedListener) Close() error {
	listener.once.Do(func() { close(listener.accepted) })
	select {
	case <-listener.closed:
	default:
		close(listener.closed)
	}
	return nil
}

func (*gatedListener) Addr() net.Addr { return dummyAddress("gated") }
