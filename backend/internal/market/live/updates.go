package live

import (
	"context"

	"crypto-scanner/internal/market"
	"crypto-scanner/internal/market/kline"
)

// apply runs on Run's goroutine; ctx bounds any read it starts.
func (service *Service) apply(ctx context.Context, event kline.Event) {
	service.mu.Lock()
	state := service.states[event.Key]
	if state == nil {
		service.mu.Unlock()
		return
	}
	before := state.freshness()
	incoming, accepted := state.receive(event)
	// A grown gap, or one whose read failed, is read now.
	service.startReadLocked(ctx, event.Key, state)
	var dropped []Client
	switch freshness := state.freshness(); {
	case accepted:
		dropped = service.publishLocked(nil, state.clients, Message{Kind: KindUpdate, Key: event.Key, Candle: &incoming, Freshness: freshness})
	case freshness != before:
		// A refused packet still publishes the connectivity it changed.
		dropped = service.publishLocked(nil, state.clients, Message{Kind: KindStatus, Key: event.Key, Freshness: freshness})
	}
	service.mu.Unlock()
	service.closeClients(dropped)
}

func (service *Service) applyStatus(status kline.Status) {
	service.mu.Lock()
	var dropped []Client
	for _, key := range status.Keys {
		state := service.states[key]
		if state == nil {
			continue
		}
		state.applyStatus(key.Interval, status)
		dropped = service.publishLocked(dropped, state.clients, Message{Kind: KindStatus, Key: key, Freshness: state.freshness(), Reason: errorString(status.Err)})
	}
	service.mu.Unlock()
	service.closeClients(dropped)
}

// HistoryChanged publishes committed closed candles, closures and
// corrections, to active graph subscribers; they also fill and cover the gap
// a key is recovering. This path is demand-driven: it adds no Binance
// subscription.
func (service *Service) HistoryChanged(candles []market.Candle) {
	service.mu.Lock()
	var dropped []Client
	for key, state := range service.states {
		var committed []market.Candle
		for _, candle := range candles {
			if candle.InstrumentID == state.instrument.ID && candle.Interval == key.Interval {
				committed = append(committed, candle)
			}
		}
		if len(committed) == 0 {
			continue
		}
		state.commit(key.Interval, committed)
		dropped = service.publishLocked(dropped, state.clients, snapshotMessage(key, state))
	}
	service.mu.Unlock()
	service.closeClients(dropped)
}
