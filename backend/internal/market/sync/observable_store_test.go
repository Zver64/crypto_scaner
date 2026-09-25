package sync_test

import (
	"context"
	"errors"
	"testing"

	"crypto-scanner/internal/market"
	marketsync "crypto-scanner/internal/market/sync"
)

type changeReportingStore struct {
	*fakeMarketStore
	changed []market.Candle
	err     error
}

func (s *changeReportingStore) UpsertCandlesWithChanges(context.Context, []market.Candle) ([]market.Candle, error) {
	return s.changed, s.err
}

func TestObservableStoreOnlyNotifiesCommittedChanges(t *testing.T) {
	input := []market.Candle{{InstrumentID: 1, Close: 10}}
	underlying := &changeReportingStore{fakeMarketStore: &fakeMarketStore{}}
	notifications := 0
	observable := marketsync.ObservableStore{Store: underlying, Changed: func(candles []market.Candle) { notifications += len(candles) }}
	if _, err := observable.UpsertCandlesWithChanges(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	if notifications != 0 {
		t.Fatal("notified for unchanged history")
	}
	underlying.changed = input
	if _, err := observable.UpsertCandlesWithChanges(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	if notifications != 1 {
		t.Fatalf("notifications=%d, want one changed candle", notifications)
	}
	underlying.err = errors.New("commit failed")
	if _, err := observable.UpsertCandlesWithChanges(context.Background(), input); err == nil {
		t.Fatal("expected failure")
	}
	if notifications != 1 {
		t.Fatal("notified before failed commit")
	}
}
