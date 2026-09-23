// Package alerts contains exact-decimal one-shot price alert use cases.
package alerts

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"
)

const MaxPerInstrument = 10

var (
	ErrNotFound      = errors.New("price alert not found")
	ErrDuplicate     = errors.New("price alert target already exists")
	ErrLimit         = errors.New("price alert limit reached")
	ErrInvalidTarget = errors.New("invalid price alert target")
)

var decimalPattern = regexp.MustCompile(`^(?:0|[1-9][0-9]{0,19})(?:\.([0-9]{1,18}))?$`)

// NormalizeTarget validates the API decimal representation and returns the
// unique plain-decimal form used by PostgreSQL and comparisons.
func NormalizeTarget(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	matches := decimalPattern.FindStringSubmatch(raw)
	if matches == nil {
		return "", ErrInvalidTarget
	}
	integer, fraction := raw, ""
	if dot := strings.IndexByte(raw, '.'); dot >= 0 {
		integer, fraction = raw[:dot], raw[dot+1:]
	}
	if len(integer) > 20 || len(integer)+len(fraction) > 38 {
		return "", ErrInvalidTarget
	}
	fraction = strings.TrimRight(fraction, "0")
	integer = strings.TrimLeft(integer, "0")
	if integer == "" {
		integer = "0"
	}
	if integer == "0" && fraction == "" {
		return "", ErrInvalidTarget
	}
	if fraction != "" {
		return integer + "." + fraction, nil
	}
	return integer, nil
}

func Compare(left, right string) (int, error) {
	l, ok := new(big.Rat).SetString(left)
	if !ok {
		return 0, fmt.Errorf("parse decimal %q", left)
	}
	r, ok := new(big.Rat).SetString(right)
	if !ok {
		return 0, fmt.Errorf("parse decimal %q", right)
	}
	return l.Cmp(r), nil
}

type Alert struct {
	ID           int64
	UserID       int64
	TelegramID   int64
	InstrumentID int64
	Symbol       string
	Target       string
	Version      int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Fired struct {
	Alert     Alert
	Price     string
	EventTime time.Time
}

type CRUDStore interface {
	ListAlerts(context.Context, int64, string) ([]Alert, error)
	CreateAlert(context.Context, int64, string, string) (Alert, error)
	UpdateAlert(context.Context, int64, int64, string) (Alert, error)
	DeleteAlert(context.Context, int64, int64) error
}

type LiveIndex interface {
	Apply(Alert)
	Remove(userID, alertID int64)
	Changed()
}

type Service struct {
	store CRUDStore
	live  LiveIndex
}

func New(store CRUDStore, live LiveIndex) *Service { return &Service{store: store, live: live} }
func (s *Service) List(ctx context.Context, userID int64, symbol string) ([]Alert, error) {
	return s.store.ListAlerts(ctx, userID, symbol)
}
func (s *Service) Create(ctx context.Context, userID int64, symbol, raw string) (Alert, error) {
	target, err := NormalizeTarget(raw)
	if err != nil {
		return Alert{}, err
	}
	item, err := s.store.CreateAlert(ctx, userID, symbol, target)
	if err == nil && s.live != nil {
		s.live.Apply(item)
	}
	return item, err
}
func (s *Service) Update(ctx context.Context, userID, id int64, raw string) (Alert, error) {
	target, err := NormalizeTarget(raw)
	if err != nil {
		return Alert{}, err
	}
	item, err := s.store.UpdateAlert(ctx, userID, id, target)
	if err == nil && s.live != nil {
		s.live.Apply(item)
	}
	return item, err
}
func (s *Service) Delete(ctx context.Context, userID, id int64) error {
	err := s.store.DeleteAlert(ctx, userID, id)
	if err == nil && s.live != nil {
		s.live.Remove(userID, id)
	}
	return err
}
