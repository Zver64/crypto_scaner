package postgres

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	generated "crypto-scanner/internal/storage/postgres/sqlc"
	"crypto-scanner/internal/strategy"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

func (store *Store) ListStrategies(ctx context.Context) ([]strategy.Strategy, error) {
	rows, err := store.queries.ListStrategies(ctx)
	if err != nil {
		return nil, fmt.Errorf("list strategies: %w", err)
	}
	items := make([]strategy.Strategy, len(rows))
	for i, row := range rows {
		items[i] = strategy.Strategy{
			ID: row.ID, Name: row.Name, Signal: row.Signal, Direction: strategy.Direction(row.Direction), Expression: row.Expression, ExitExpression: row.ExitExpression,
			TakeProfitExpression: row.TakeProfitExpression, StopLossExpression: row.StopLossExpression,
			Coins: row.Coins, CoinSource: strategy.CoinSource(row.CoinSource),
			MarketCap:   strategy.MarketCapRange{MinUSD: float8Pointer(row.MinMarketCapUsd), MaxUSD: float8Pointer(row.MaxMarketCapUsd)},
			TargetRatio: int(row.TargetRatio.Int16), Window: int(row.WindowCandles.Int16),
			Message: row.Message, Enabled: row.Enabled, BaselinePending: row.BaselinePending, Revision: row.Revision,
		}
	}
	return items, nil
}

func (store *Store) CreateStrategy(ctx context.Context, item strategy.Strategy, indicatorIDs []int64) (uuid.UUID, error) {
	tx, err := store.db.Begin(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("begin strategy creation: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	queries := store.queries.WithTx(tx)
	id, err := queries.InsertStrategy(ctx, generated.InsertStrategyParams{
		Name: item.Name, Signal: item.Signal, Direction: string(item.Direction), Expression: item.Expression, ExitExpression: item.ExitExpression,
		TakeProfitExpression: item.TakeProfitExpression, StopLossExpression: item.StopLossExpression,
		CoinSource: string(item.CoinSource), MinMarketCapUsd: float8(item.MarketCap.MinUSD), MaxMarketCapUsd: float8(item.MarketCap.MaxUSD),
		TargetRatio: optionalInt2(item.TargetRatio), WindowCandles: optionalInt2(item.Window),
		Message: item.Message, Enabled: item.Enabled,
	})
	if err != nil {
		return uuid.Nil, strategyWriteError("insert strategy", err)
	}
	if err := queries.InsertStrategyIndicators(ctx, generated.InsertStrategyIndicatorsParams{StrategyID: id, IndicatorIds: indicatorIDs}); err != nil {
		return uuid.Nil, strategyWriteError("insert strategy indicators", err)
	}
	if err := replaceStrategyCoins(ctx, queries, id, item.Coins); err != nil {
		return uuid.Nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, fmt.Errorf("commit strategy creation: %w", err)
	}
	return id, nil
}

// UpdateStrategy replaces the name, expressions, coins, trading settings,
// message, and indicators and returns the revision; retrade starts a new
// revision, and baseline requests a fresh start of the trading states.
func (store *Store) UpdateStrategy(ctx context.Context, item strategy.Strategy, indicatorIDs []int64, retrade, baseline bool) (int64, error) {
	tx, err := store.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin strategy update: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	queries := store.queries.WithTx(tx)
	revision, err := queries.UpdateStrategy(ctx, generated.UpdateStrategyParams{
		ID: item.ID, Name: item.Name, Expression: item.Expression, ExitExpression: item.ExitExpression,
		TakeProfitExpression: item.TakeProfitExpression, StopLossExpression: item.StopLossExpression,
		CoinSource: string(item.CoinSource), MinMarketCapUsd: float8(item.MarketCap.MinUSD), MaxMarketCapUsd: float8(item.MarketCap.MaxUSD),
		TargetRatio: optionalInt2(item.TargetRatio), WindowCandles: optionalInt2(item.Window),
		Message: item.Message, Retrade: retrade, Baseline: baseline,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, strategy.ErrNotFound
	}
	if err != nil {
		return 0, strategyWriteError("update strategy", err)
	}
	if err := queries.DeleteStrategyIndicators(ctx, item.ID); err != nil {
		return 0, fmt.Errorf("delete strategy indicators: %w", err)
	}
	if err := queries.InsertStrategyIndicators(ctx, generated.InsertStrategyIndicatorsParams{StrategyID: item.ID, IndicatorIds: indicatorIDs}); err != nil {
		return 0, strategyWriteError("insert strategy indicators", err)
	}
	if err := replaceStrategyCoins(ctx, queries, item.ID, item.Coins); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit strategy update: %w", err)
	}
	return revision, nil
}

// replaceStrategyCoins replaces the coins of a strategy; a new strategy has
// none to delete. It first locks the coins' instruments in id
// order, the order of the other instrument row locks, so the rewrite cannot
// deadlock with the deletion of delisted instruments, whose cascade deletes
// coin rows.
func replaceStrategyCoins(ctx context.Context, queries *generated.Queries, id uuid.UUID, symbols []string) error {
	if err := queries.LockStrategyCoinInstruments(ctx, symbols); err != nil {
		return fmt.Errorf("lock strategy coin instruments: %w", err)
	}
	if err := queries.DeleteStrategyCoins(ctx, id); err != nil {
		return fmt.Errorf("delete strategy coins: %w", err)
	}
	if err := queries.InsertStrategyCoins(ctx, generated.InsertStrategyCoinsParams{StrategyID: id, Symbols: symbols}); err != nil {
		return fmt.Errorf("insert strategy coins: %w", err)
	}
	return nil
}

// optionalInt2 stores 0, which a trading strategy has for the settings of
// signals, as NULL.
func optionalInt2(value int) pgtype.Int2 {
	return pgtype.Int2{Int16: int16(value), Valid: value != 0}
}

// SetStrategyEnabled turns a strategy on or off and returns the revision. An
// enabled strategy awaits its baseline; a disabled one forgets its trading
// states.
func (store *Store) SetStrategyEnabled(ctx context.Context, id uuid.UUID, enabled bool) (int64, error) {
	revision, err := store.setStrategyEnabled(ctx, id, enabled, func(queries *generated.Queries) (int64, error) {
		return queries.SetStrategyEnabled(ctx, generated.SetStrategyEnabledParams{ID: id, Enabled: enabled})
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, strategy.ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("set strategy enabled: %w", err)
	}
	return revision, nil
}

// DisableStrategyAtRevision disables the strategy like SetStrategyEnabled,
// but only while it is enabled at revision, and returns the new revision;
// otherwise disabled is false and nothing changes.
func (store *Store) DisableStrategyAtRevision(ctx context.Context, id uuid.UUID, revision int64) (next int64, disabled bool, err error) {
	next, err = store.setStrategyEnabled(ctx, id, false, func(queries *generated.Queries) (int64, error) {
		return queries.DisableStrategyAtRevision(ctx, generated.DisableStrategyAtRevisionParams{ID: id, Revision: revision})
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("disable strategy at revision: %w", err)
	}
	return next, true, nil
}

// setStrategyEnabled runs update, which returns the new revision or
// pgx.ErrNoRows, in a transaction that forgets the trading states of a
// disabled strategy.
func (store *Store) setStrategyEnabled(ctx context.Context, id uuid.UUID, enabled bool, update func(*generated.Queries) (int64, error)) (int64, error) {
	tx, err := store.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	queries := store.queries.WithTx(tx)
	revision, err := update(queries)
	if err != nil {
		return 0, fmt.Errorf("update: %w", err)
	}
	if !enabled {
		if err := queries.DeleteAllStrategyStates(ctx, id); err != nil {
			return 0, fmt.Errorf("delete strategy states: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return revision, nil
}

func (store *Store) DeleteStrategy(ctx context.Context, id uuid.UUID) error {
	deleted, err := store.queries.DeleteStrategy(ctx, id)
	if err != nil {
		return fmt.Errorf("delete strategy: %w", err)
	}
	if deleted == 0 {
		return strategy.ErrNotFound
	}
	return nil
}

// ListStrategyStates maps strategy ids to the trading states of their
// instruments.
func (store *Store) ListStrategyStates(ctx context.Context) (map[uuid.UUID]map[int64]strategy.TradeState, error) {
	rows, err := store.queries.ListStrategyStates(ctx)
	if err != nil {
		return nil, fmt.Errorf("list strategy states: %w", err)
	}
	result := map[uuid.UUID]map[int64]strategy.TradeState{}
	for _, row := range rows {
		if result[row.StrategyID] == nil {
			result[row.StrategyID] = map[int64]strategy.TradeState{}
		}
		state := strategy.TradeState{OpenTime: row.OpenTime.Time.UTC(), Entry: row.Entry, Buys: int(row.Buys), Filled: int(row.Filled), Quantity: row.Quantity,
			TakeProfit: row.TakeProfit, StopLoss: row.StopLoss,
		}
		if row.OpenedAt.Valid {
			state.OpenedAt = row.OpenedAt.Time.UTC()
		}
		result[row.StrategyID][row.InstrumentID] = state
	}
	return result, nil
}

// ReplaceStrategyStates stores the baseline states of revision, which hold
// no trades, and completes its pending baseline. It reports false, changing
// nothing, when that revision no longer awaits a baseline.
func (store *Store) ReplaceStrategyStates(ctx context.Context, strategyID uuid.UUID, revision int64, states map[int64]strategy.TradeState) (bool, error) {
	tx, err := store.db.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin strategy baseline: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	queries := store.queries.WithTx(tx)
	current, err := queries.CompleteStrategyBaseline(ctx, generated.CompleteStrategyBaselineParams{ID: strategyID, Revision: revision})
	if err != nil {
		return false, fmt.Errorf("complete strategy baseline: %w", err)
	}
	if current == 0 {
		return false, nil
	}
	if err := queries.DeleteAllStrategyStates(ctx, strategyID); err != nil {
		return false, fmt.Errorf("delete strategy states: %w", err)
	}
	params := generated.InsertStrategyStatesParams{StrategyID: strategyID}
	for _, id := range slices.Sorted(maps.Keys(states)) {
		params.InstrumentIds = append(params.InstrumentIds, id)
		params.OpenTimes = append(params.OpenTimes, pgtype.Timestamptz{Time: states[id].OpenTime, Valid: true})
		params.Entries = append(params.Entries, states[id].Entry)
	}
	if err := queries.InsertStrategyStates(ctx, params); err != nil {
		return false, fmt.Errorf("insert strategy states: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit strategy baseline: %w", err)
	}
	return true, nil
}

// SaveStrategyStates stores states of later candles than the stored ones
// while revision is current and announced, and returns the instruments
// stored.
func (store *Store) SaveStrategyStates(ctx context.Context, strategyID uuid.UUID, revision int64, states map[int64]strategy.TradeState) ([]int64, error) {
	params := generated.SaveStrategyStatesParams{StrategyID: strategyID, Revision: revision}
	for _, id := range slices.Sorted(maps.Keys(states)) {
		state := states[id]
		params.InstrumentIds = append(params.InstrumentIds, id)
		params.OpenTimes = append(params.OpenTimes, pgtype.Timestamptz{Time: state.OpenTime, Valid: true})
		params.Entries = append(params.Entries, state.Entry)
		params.Buys = append(params.Buys, int32(state.Buys))
		params.Filled = append(params.Filled, int32(state.Filled))
		params.Quantities = append(params.Quantities, state.Quantity)
		params.OpenedAt = append(params.OpenedAt, pgtype.Timestamptz{Time: state.OpenedAt, Valid: !state.OpenedAt.IsZero()})
		params.TakeProfits = append(params.TakeProfits, state.TakeProfit)
		params.StopLosses = append(params.StopLosses, state.StopLoss)
	}
	saved, err := store.queries.SaveStrategyStates(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("save strategy states: %w", err)
	}
	return saved, nil
}

func (store *Store) DeleteStrategyStates(ctx context.Context, strategyID uuid.UUID, instrumentIDs []int64) error {
	if err := store.queries.DeleteStrategyStates(ctx, generated.DeleteStrategyStatesParams{StrategyID: strategyID, InstrumentIds: instrumentIDs}); err != nil {
		return fmt.Errorf("delete strategy states: %w", err)
	}
	return nil
}

// ListStrategyFavorites returns the symbols of the user's favorites, active
// or not, ascending.
func (store *Store) ListStrategyFavorites(ctx context.Context, userID int64) ([]string, error) {
	symbols, err := store.queries.ListStrategyFavorites(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list strategy favorites: %w", err)
	}
	return symbols, nil
}

// ListStrategyInstruments returns the active instruments, which strategies
// evaluate.
func (store *Store) ListStrategyInstruments(ctx context.Context) ([]strategy.Instrument, error) {
	rows, err := store.queries.ListStrategyInstruments(ctx)
	if err != nil {
		return nil, fmt.Errorf("list strategy instruments: %w", err)
	}
	items := make([]strategy.Instrument, len(rows))
	for i, row := range rows {
		items[i] = strategy.Instrument{ID: row.ID, Symbol: row.Symbol}
		if row.MarketCapKnown {
			items[i].MarketCapUSD = &row.MarketCapUsd
		}
	}
	return items, nil
}

func (store *Store) ListStrategyRecipients(ctx context.Context, administratorTelegramID int64) ([]int64, error) {
	recipients, err := store.queries.ListStrategyRecipients(ctx, administratorTelegramID)
	if err != nil {
		return nil, fmt.Errorf("list strategy recipients: %w", err)
	}
	return recipients, nil
}

// strategyWriteError maps a taken name to strategy.ErrConflict and an
// indicator deleted meanwhile to strategy.ErrInvalidArgument, and wraps any
// other error with operation.
func strategyWriteError(operation string, err error) error {
	if duplicateViolation(err) {
		return strategy.ErrConflict
	}
	if foreignKeyViolation(err) {
		return fmt.Errorf("%w: an indicator it reads no longer exists", strategy.ErrInvalidArgument)
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func foreignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}
