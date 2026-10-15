package marketsync_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"crypto-scanner/internal/market"
	"crypto-scanner/internal/market/marketsync"
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
	observable := &marketsync.ObservableStore{Store: underlying, Changed: func(candles []market.Candle) { notifications += len(candles) }}
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

// snapshotStore makes the applied snapshot the active instruments; listErr
// fails listing them.
type snapshotStore struct {
	*fakeMarketStore
	listErr error
}

func (s *snapshotStore) ApplyInstrumentSnapshot(_ context.Context, items []market.Instrument) error {
	s.active = items
	return nil
}

func (s *snapshotStore) ListActiveInstruments(ctx context.Context) ([]market.Instrument, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.fakeMarketStore.ListActiveInstruments(ctx)
}

// The first snapshot and every one that changes the active instruments are
// reported; one whose result cannot be read is reported by the next.
func TestObservableStoreReportsChangedActiveInstruments(t *testing.T) {
	btc, eth := market.Instrument{ID: 1, Symbol: "BTCUSDT"}, market.Instrument{ID: 2, Symbol: "ETHUSDT"}
	underlying := &snapshotStore{fakeMarketStore: &fakeMarketStore{}}
	var notified [][]market.Instrument
	observable := &marketsync.ObservableStore{Store: underlying, InstrumentsChanged: func(instruments []market.Instrument) {
		notified = append(notified, instruments)
	}}
	apply := func(snapshot ...market.Instrument) error {
		return observable.ApplyInstrumentSnapshot(context.Background(), snapshot)
	}
	for _, snapshot := range [][]market.Instrument{{btc}, {btc, eth}, {btc, eth}} {
		if err := apply(snapshot...); err != nil {
			t.Fatal(err)
		}
	}
	underlying.listErr = errors.New("database unavailable")
	if err := apply(eth); err == nil {
		t.Fatal("expected the listing failure")
	}
	underlying.listErr = nil
	if err := apply(eth); err != nil {
		t.Fatal(err)
	}
	want := [][]market.Instrument{{btc}, {btc, eth}, {eth}}
	if !reflect.DeepEqual(notified, want) {
		t.Fatalf("notified = %v, want %v", notified, want)
	}
}
