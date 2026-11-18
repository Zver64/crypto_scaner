package binance

import (
	"cmp"
	"slices"
	"sync"
)

// statusQueue holds the connection statuses of a stream pool until its
// consumer drains them, so a busy consumer never loses one. Statuses
// coalesce per key: a disconnect replaces whatever is pending, and a
// connection replaces a pending connection but keeps the disconnect before
// it, because the consumer must see that the key went down to recover what
// it missed meanwhile.
type statusQueue[K comparable] struct {
	// ready signals, without ever blocking the publisher, that drain has
	// statuses; capacity 1, and a pending signal covers every later status.
	ready chan struct{}

	mu      sync.Mutex
	pending map[K]keyStatuses
}

// keyStatuses are the pending statuses of one key: at most a disconnect
// followed by a newer connection.
type keyStatuses struct {
	down *connectionStatus
	up   *connectionStatus
}

// connectionStatus reports the keys of connection epoch going up or down.
type connectionStatus struct {
	connected bool
	epoch     int64
	err       error
}

// statusUpdate is one drained status and the keys it applies to.
type statusUpdate[K comparable] struct {
	keys []K
	connectionStatus
}

func newStatusQueue[K comparable]() *statusQueue[K] {
	return &statusQueue[K]{ready: make(chan struct{}, 1), pending: make(map[K]keyStatuses)}
}

func (queue *statusQueue[K]) publish(keys []K, connected bool, epoch int64, err error) {
	if len(keys) == 0 {
		return
	}
	status := &connectionStatus{connected: connected, epoch: epoch, err: err}
	queue.mu.Lock()
	for _, key := range keys {
		if connected {
			pending := queue.pending[key]
			pending.up = status
			queue.pending[key] = pending
		} else {
			queue.pending[key] = keyStatuses{down: status}
		}
	}
	queue.mu.Unlock()
	select {
	case queue.ready <- struct{}{}:
	default:
	}
}

// drain returns and clears the pending statuses in the order they happened.
func (queue *statusQueue[K]) drain() []statusUpdate[K] {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	// A status is published once for all keys of its connection, so the
	// keys sharing one status pointer form one update again.
	updates := make(map[*connectionStatus]*statusUpdate[K])
	for key, pending := range queue.pending {
		for _, status := range []*connectionStatus{pending.down, pending.up} {
			if status == nil {
				continue
			}
			update := updates[status]
			if update == nil {
				update = &statusUpdate[K]{connectionStatus: *status}
				updates[status] = update
			}
			update.keys = append(update.keys, key)
		}
	}
	clear(queue.pending)
	result := make([]statusUpdate[K], 0, len(updates))
	for _, update := range updates {
		result = append(result, *update)
	}
	// Epochs grow with every connection attempt; one connection goes up
	// before it goes down.
	slices.SortFunc(result, func(left, right statusUpdate[K]) int {
		if order := cmp.Compare(left.epoch, right.epoch); order != 0 {
			return order
		}
		switch {
		case left.connected == right.connected:
			return 0
		case left.connected:
			return -1
		default:
			return 1
		}
	})
	return result
}
