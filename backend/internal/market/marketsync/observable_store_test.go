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

// listingStore lists active as the active instruments; listErr fails the
// listing.
type listingStore struct {
	*fakeMarketStore
	listErr error
}

func (s *listingStore) ListActiveInstrumentsByMarketCap(ctx context.Context) ([]market.Instrument, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.fakeMarketStore.ListActiveInstrumentsByMarketCap(ctx)
}

// The first listing and every one that changes the set of active instruments
// are reported, a new order alone is not; one that fails is reported by the
// next.
func TestObservableStoreReportsChangedActiveInstruments(t *testing.T) {
	btc, eth := market.Instrument{ID: 1, Symbol: "BTCUSDT"}, market.Instrument{ID: 2, Symbol: "ETHUSDT"}
	underlying := &listingStore{fakeMarketStore: &fakeMarketStore{}}
	var notified [][]market.Instrument
	observable := &marketsync.ObservableStore{Store: underlying, InstrumentsChanged: func(instruments []market.Instrument) {
		notified = append(notified, instruments)
	}}
	list := func(active ...market.Instrument) error {
		underlying.active = active
		got, err := observable.ListActiveInstrumentsByMarketCap(context.Background())
		if err == nil && !reflect.DeepEqual(got, active) {
			t.Fatalf("listed %v, want %v", got, active)
		}
		return err
	}
	for _, active := range [][]market.Instrument{{btc}, {btc, eth}, {eth, btc}} {
		if err := list(active...); err != nil {
			t.Fatal(err)
		}
	}
	underlying.listErr = errors.New("database unavailable")
	if err := list(eth); err == nil {
		t.Fatal("expected the listing failure")
	}
	underlying.listErr = nil
	if err := list(eth); err != nil {
		t.Fatal(err)
	}
	want := [][]market.Instrument{{btc}, {btc, eth}, {eth}}
	if !reflect.DeepEqual(notified, want) {
		t.Fatalf("notified = %v, want %v", notified, want)
	}
}
