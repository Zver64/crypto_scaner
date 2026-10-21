package logging

import (
	"context"
	"log/slog"
	"maps"
	"sync"
)

// Failures logs recurring failures, keyed by what fails, so a lasting one
// reaches the administrator once: the first failure of a key is logged at
// WARN (or ERROR), each repeat at INFO, until Recover or Forget re-arms the
// key, so a later failure is reported again. It is safe for concurrent use.
type Failures[K comparable] struct {
	logger  *slog.Logger
	mu      sync.Mutex
	failing map[K]struct{}
}

// NewFailures returns Failures that log to logger.
func NewFailures[K comparable](logger *slog.Logger) *Failures[K] {
	return &Failures[K]{logger: logger, failing: map[K]struct{}{}}
}

// Warn logs a failure of key at WARN when key starts failing, otherwise at
// INFO.
func (f *Failures[K]) Warn(ctx context.Context, key K, msg string, args ...any) {
	f.fail(ctx, slog.LevelWarn, key, msg, args)
}

// Error logs a failure of key at ERROR when key starts failing, otherwise at
// INFO.
func (f *Failures[K]) Error(ctx context.Context, key K, msg string, args ...any) {
	f.fail(ctx, slog.LevelError, key, msg, args)
}

func (f *Failures[K]) fail(ctx context.Context, level slog.Level, key K, msg string, args []any) {
	if !f.Mark(key) {
		level = slog.LevelInfo
	}
	f.logger.Log(ctx, level, msg, args...)
}

// Mark records a failure of key without logging and reports whether key
// starts failing, for callers that log several keys in one record.
func (f *Failures[K]) Mark(key K) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, known := f.failing[key]
	f.failing[key] = struct{}{}
	return !known
}

// Recover re-arms key and, when it was failing, logs msg at INFO.
func (f *Failures[K]) Recover(ctx context.Context, key K, msg string, args ...any) {
	if f.forget(key) {
		f.logger.InfoContext(ctx, msg, args...)
	}
}

// Forget re-arms key without logging, such as when a success is not worth a
// record.
func (f *Failures[K]) Forget(key K) { f.forget(key) }

func (f *Failures[K]) forget(key K) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, known := f.failing[key]
	delete(f.failing, key)
	return known
}

// Retain forgets the failing keys keep rejects, such as those no longer in
// use.
func (f *Failures[K]) Retain(keep func(K) bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	maps.DeleteFunc(f.failing, func(key K, _ struct{}) bool { return !keep(key) })
}

// Failure is Failures of a single operation.
type Failure struct {
	failures *Failures[struct{}]
}

// NewFailure returns a Failure that logs to logger.
func NewFailure(logger *slog.Logger) *Failure {
	return &Failure{failures: NewFailures[struct{}](logger)}
}

// Warn logs a failure at WARN when the operation starts failing, otherwise
// at INFO.
func (f *Failure) Warn(ctx context.Context, msg string, args ...any) {
	f.failures.Warn(ctx, struct{}{}, msg, args...)
}

// Recover re-arms the operation and, when it was failing, logs msg at INFO.
func (f *Failure) Recover(ctx context.Context, msg string, args ...any) {
	f.failures.Recover(ctx, struct{}{}, msg, args...)
}

// Forget re-arms the operation without logging.
func (f *Failure) Forget() { f.failures.Forget(struct{}{}) }
