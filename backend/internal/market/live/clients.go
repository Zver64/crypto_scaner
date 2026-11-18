package live

import (
	"context"
	"errors"
	"fmt"
	"time"

	"crypto-scanner/internal/market"
	"crypto-scanner/internal/market/kline"
)

// clientState is one registered client connection and the keys it watches.
type clientState struct {
	client Client
	keys   map[kline.Key]struct{}
}

func (service *Service) RegisterClient(client Client) bool {
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.stopped {
		return false
	}
	state := service.clients[client.ID()]
	if state == nil {
		state = &clientState{keys: make(map[kline.Key]struct{})}
		service.clients[client.ID()] = state
	}
	state.client = client
	return true
}

func (service *Service) Subscribe(ctx context.Context, client Client, symbol string, interval market.CandleInterval) error {
	symbol = market.NormalizeSymbol(symbol)
	if !interval.Valid() {
		return fmt.Errorf("live candles: unsupported candle interval %q", interval)
	}
	key := kline.Key{Symbol: symbol, Interval: interval}
	service.mu.Lock()
	if done, err := service.checkSubscriptionLocked(client.ID(), key); done {
		service.mu.Unlock()
		return err
	}
	state := service.states[key]
	if state == nil {
		service.mu.Unlock()
		instrument, err := service.store.GetActiveInstrumentBySymbol(ctx, symbol)
		if err != nil {
			if errors.Is(err, market.ErrInstrumentNotFound) {
				return ErrInactiveSymbol
			}
			return fmt.Errorf("look up active instrument: %w", err)
		}
		service.mu.Lock()
		// The lookup ran unlocked: recheck everything it may have changed,
		// including a state created or released meanwhile.
		if done, err := service.checkSubscriptionLocked(client.ID(), key); done {
			service.mu.Unlock()
			return err
		}
		state = service.states[key]
		if state == nil {
			state = newKeyState(instrument)
			service.states[key] = state
			service.upstream.Subscribe(key)
			service.wakeRequestsLocked()
		}
	}
	state.releaseAt = time.Time{}
	state.clients[client.ID()] = client
	registration := service.clients[client.ID()]
	if registration == nil {
		registration = &clientState{client: client, keys: make(map[kline.Key]struct{})}
		service.clients[client.ID()] = registration
	}
	registration.keys[key] = struct{}{}
	var dropped []Client
	if !client.Enqueue(Message{Kind: KindSubscribed, Key: key}) || !client.Enqueue(snapshotMessage(key, state)) {
		service.removeClientLocked(client.ID())
		dropped = append(dropped, client)
	}
	service.mu.Unlock()
	service.closeClients(dropped)
	service.logCounts("subscribe")
	return nil
}

// checkSubscriptionLocked reports whether Subscribe is done before attaching
// the client, with its result; service.mu must be held.
func (service *Service) checkSubscriptionLocked(clientID string, key kline.Key) (bool, error) {
	if service.stopped {
		return true, ErrServiceStopped
	}
	if clientState := service.clients[clientID]; clientState != nil {
		if _, exists := clientState.keys[key]; exists {
			return true, nil
		}
		if len(clientState.keys) >= maxClientSubscriptions {
			return true, ErrTooManyClientSubscriptions
		}
	}
	return false, nil
}

func (service *Service) Unsubscribe(clientID string, key kline.Key) {
	key.Symbol = market.NormalizeSymbol(key.Symbol)
	service.mu.Lock()
	service.unsubscribeLocked(clientID, key)
	service.mu.Unlock()
	service.logCounts("unsubscribe")
}

func (service *Service) RemoveClient(clientID string) {
	service.mu.Lock()
	service.removeClientLocked(clientID)
	service.mu.Unlock()
	service.logCounts("disconnect")
}

func (service *Service) removeClientLocked(clientID string) {
	clientState := service.clients[clientID]
	var keys []kline.Key
	if clientState != nil {
		keys = make([]kline.Key, 0, len(clientState.keys))
		for key := range clientState.keys {
			keys = append(keys, key)
		}
	}
	for _, key := range keys {
		service.unsubscribeLocked(clientID, key)
	}
	delete(service.clients, clientID)
}

func (service *Service) unsubscribeLocked(clientID string, key kline.Key) {
	state := service.states[key]
	if state == nil {
		return
	}
	if _, exists := state.clients[clientID]; !exists {
		return
	}
	delete(state.clients, clientID)
	if clientState := service.clients[clientID]; clientState != nil {
		delete(clientState.keys, key)
	}
	if len(state.clients) != 0 {
		return
	}
	state.releaseAt = time.Now().Add(service.delay)
	select {
	case service.releases <- struct{}{}:
	default:
	}
}

// releaseDue releases keys whose release delay passed without clients and
// sets timer to the next pending release.
func (service *Service) releaseDue(timer *time.Timer) {
	service.mu.Lock()
	now := time.Now()
	var next time.Time
	released := false
	for key, state := range service.states {
		switch {
		case state.releaseAt.IsZero() || len(state.clients) != 0:
		case !state.releaseAt.After(now):
			delete(service.states, key)
			service.upstream.Unsubscribe(key)
			released = true
		case next.IsZero() || state.releaseAt.Before(next):
			next = state.releaseAt
		}
	}
	service.mu.Unlock()
	timer.Stop()
	if !next.IsZero() {
		timer.Reset(time.Until(next))
	}
	if released {
		service.logCounts("release")
	}
}
