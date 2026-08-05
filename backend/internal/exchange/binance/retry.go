package binance

import (
	"context"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

type retryTransport struct {
	base       http.RoundTripper
	limiter    *rate.Limiter
	attempts   int
	baseDelay  time.Duration
	retryCount atomic.Uint64
}

func (transport *retryTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	var response *http.Response
	var err error
	for attempt := 0; attempt < transport.attempts; attempt++ {
		if err := transport.limiter.Wait(request.Context()); err != nil {
			return nil, err
		}
		response, err = transport.base.RoundTrip(request.Clone(request.Context()))
		if !retryable(response, err) || attempt == transport.attempts-1 {
			return response, err
		}
		if response != nil && response.Body != nil {
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
		}
		transport.retryCount.Add(1)
		delay := retryDelay(response, transport.baseDelay, attempt)
		if err := wait(request.Context(), delay); err != nil {
			return nil, err
		}
	}
	return response, err
}

func retryable(response *http.Response, err error) bool {
	if err != nil {
		return true
	}
	return response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= http.StatusInternalServerError
}

func retryDelay(response *http.Response, base time.Duration, attempt int) time.Duration {
	if response != nil {
		if value := response.Header.Get("Retry-After"); value != "" {
			if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
				return time.Duration(seconds) * time.Second
			}
			if deadline, err := http.ParseTime(value); err == nil {
				return max(time.Until(deadline), 0)
			}
		}
	}
	delay := base * time.Duration(1<<min(attempt, 7))
	return delay + time.Duration(rand.Float64()*0.25*float64(delay))
}

func wait(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
