package binance

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"crypto-scanner/internal/platform/backoff"

	"golang.org/x/time/rate"
)

type historyRepairContextKey struct{}

const (
	// usedWeightHeader reports the IP's request weight in the current minute.
	usedWeightHeader = "X-MBX-USED-WEIGHT-1M"
	// weightPauseShare of the per-minute limit pauses requests until the next
	// minute, leaving headroom for requests already in flight.
	weightPauseShare = 0.9
)

type retryTransport struct {
	base          http.RoundTripper
	limiter       *rate.Limiter // request weight per second
	repairLimiter *rate.Limiter // history repair requests per second
	attempts      int
	baseDelay     time.Duration
	retryCount    atomic.Uint64
	// weightLimit is Binance's REQUEST_WEIGHT per minute, 0 until discovered.
	weightLimit atomic.Int64
	// pausedUntil (Unix nanoseconds) stops every request after a rate-limit
	// response or a nearly exhausted weight budget, not only the one that saw it.
	pausedUntil atomic.Int64
}

func (transport *retryTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	ctx := request.Context()
	weight := min(requestWeight(request.URL.Path), transport.limiter.Burst())
	var response *http.Response
	var err error
	for attempt := 0; attempt < transport.attempts; attempt++ {
		if paused := time.Until(time.Unix(0, transport.pausedUntil.Load())); paused > 0 {
			if err := backoff.Sleep(ctx, paused); err != nil {
				return nil, err
			}
		}
		if err := transport.limiter.WaitN(ctx, weight); err != nil {
			return nil, err
		}
		if ctx.Value(historyRepairContextKey{}) == true {
			if err := transport.repairLimiter.Wait(ctx); err != nil {
				return nil, err
			}
		}
		response, err = transport.base.RoundTrip(request.Clone(ctx))
		transport.observe(response)
		if !retryable(response, err) || attempt == transport.attempts-1 {
			return response, err
		}
		if response != nil && response.Body != nil {
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
		}
		transport.retryCount.Add(1)
		delay := retryDelay(response, transport.baseDelay, attempt)
		if err := backoff.Sleep(ctx, delay); err != nil {
			return nil, err
		}
	}
	return response, err
}

// observe pauses all requests when Binance reports a rate limit (429), an IP
// ban (418), or a weight budget close to exhaustion.
func (transport *retryTransport) observe(response *http.Response) {
	if response == nil {
		return
	}
	if response.StatusCode == http.StatusTooManyRequests || response.StatusCode == http.StatusTeapot {
		if delay, ok := retryAfter(response); ok {
			transport.pause(time.Now().Add(delay))
		}
		return
	}
	limit := transport.weightLimit.Load()
	used, err := strconv.ParseInt(response.Header.Get(usedWeightHeader), 10, 64)
	if limit > 0 && err == nil && float64(used) >= float64(limit)*weightPauseShare {
		transport.pause(time.Now().Truncate(time.Minute).Add(time.Minute + time.Second))
	}
}

func (transport *retryTransport) pause(until time.Time) {
	next := until.UnixNano()
	for {
		current := transport.pausedUntil.Load()
		if current >= next || transport.pausedUntil.CompareAndSwap(current, next) {
			return
		}
	}
}

// requestWeight returns the documented REQUEST_WEIGHT of a public endpoint.
func requestWeight(path string) int {
	switch {
	case strings.HasSuffix(path, "/exchangeInfo"):
		return 20
	case strings.HasSuffix(path, "/klines"):
		return 2
	default:
		return 1
	}
}

func retryable(response *http.Response, err error) bool {
	if err != nil {
		return true
	}
	return response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= http.StatusInternalServerError
}

func retryDelay(response *http.Response, base time.Duration, attempt int) time.Duration {
	if delay, ok := retryAfter(response); ok {
		return delay
	}
	return backoff.Jitter(backoff.Exponential(base, base<<7, attempt), 0.25)
}

func retryAfter(response *http.Response) (time.Duration, bool) {
	if response == nil {
		return 0, false
	}
	value := response.Header.Get("Retry-After")
	if value == "" {
		return 0, false
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second, true
	}
	if deadline, err := http.ParseTime(value); err == nil {
		return max(time.Until(deadline), 0), true
	}
	return 0, false
}
