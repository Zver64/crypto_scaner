package postgres

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	generated "crypto-scanner/internal/storage/postgres/sqlc"
	"crypto-scanner/internal/strategy"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

func (store *Store) ListStrategies(ctx context.Context) ([]strategy.Strategy, error) {
	rows, err := store.queries.ListStrategies(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]strategy.Strategy, len(rows))
	for i, row := range rows {
		items[i] = strategy.Strategy{
			ID: row.ID, Name: row.Name, Expression: row.Expression, ExitExpression: row.ExitExpression,
			TakeProfitExpression: row.TakeProfitExpression, StopLossExpression: row.StopLossExpression,
			MarketCap: strategy.MarketCapRange{MinUSD: float8Pointer(row.MinMarketCapUsd), MaxUSD: float8Pointer(row.MaxMarketCapUsd)},
			Message:   row.Message, Enabled: row.Enabled, BaselinePending: row.BaselinePending, Revision: row.Revision,
		}
	}
	return items, nil
}

func (store *Store) CreateStrategy(ctx context.Context, item strategy.Strategy, indicatorIDs []int64, symbols []string, administratorTelegramID int64) (int64, error) {
	tx, err := store.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	queries := store.queries.WithTx(tx)
	id, err := queries.InsertStrategy(ctx, generated.InsertStrategyParams{
		Name: item.Name, Expression: item.Expression, ExitExpression: item.ExitExpression,
		TakeProfitExpression: item.TakeProfitExpression, StopLossExpression: item.StopLossExpression,
		MinMarketCapUsd: float8(item.MarketCap.MinUSD), MaxMarketCapUsd: float8(item.MarketCap.MaxUSD),
		Message: item.Message, Enabled: item.Enabled,
	})
	if err != nil {
		return 0, strategyWriteError(err)
	}
	if err := queries.InsertStrategyIndicators(ctx, generated.InsertStrategyIndicatorsParams{StrategyID: id, IndicatorIds: indicatorIDs}); err != nil {
		return 0, strategyWriteError(err)
	}
	if err := replaceStrategySymbols(ctx, queries, id, symbols, administratorTelegramID); err != nil {
		return 0, err
	}
	return id, tx.Commit(ctx)
}

// UpdateStrategy replaces the name, expressions, trading settings, message,
// indicators, and instruments and returns the revision; baseline requests a
// fresh start of the trading states.
func (store *Store) UpdateStrategy(ctx context.Context, item strategy.Strategy, indicatorIDs []int64, symbols []string, administratorTelegramID int64, baseline bool) (int64, error) {
	tx, err := store.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	queries := store.queries.WithTx(tx)
	revision, err := queries.UpdateStrategy(ctx, generated.UpdateStrategyParams{
		ID: item.ID, Name: item.Name, Expression: item.Expression, ExitExpression: item.ExitExpression,
		TakeProfitExpression: item.TakeProfitExpression, StopLossExpression: item.StopLossExpression,
		MinMarketCapUsd: float8(item.MarketCap.MinUSD), MaxMarketCapUsd: float8(item.MarketCap.MaxUSD),
		Message: item.Message, Baseline: baseline,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, strategy.ErrNotFound
	}
	if err != nil {
		return 0, strategyWriteError(err)
	}
	if err := queries.DeleteStrategyIndicators(ctx, item.ID); err != nil {
		return 0, err
	}
	if err := queries.InsertStrategyIndicators(ctx, generated.InsertStrategyIndicatorsParams{StrategyID: item.ID, IndicatorIds: indicatorIDs}); err != nil {
		return 0, strategyWriteError(err)
	}
	if err := replaceStrategySymbols(ctx, queries, item.ID, symbols, administratorTelegramID); err != nil {
		return 0, err
	}
	return revision, tx.Commit(ctx)
}

// replaceStrategySymbols records the instruments a strategy reads through
// of. Instruments it did not read before must be in the administrator's
// favorites; those it keeps reading stay valid, so a delisting never blocks
// a rename. The instrument locks wait for a concurrent favorite removal, and
// the check runs after them, so it sees that removal.
func replaceStrategySymbols(ctx context.Context, queries *generated.Queries, strategyID int64, symbols []string, administratorTelegramID int64) error {
	previous, err := queries.ListStrategySymbolIDs(ctx, strategyID)
	if err != nil {
		return err
	}
	if err := queries.DeleteStrategySymbols(ctx, strategyID); err != nil {
		return err
	}
	if len(symbols) == 0 {
		return nil
	}
	rows, err := queries.LockStrategySymbols(ctx, symbols)
	if err != nil {
		return err
	}
	var ids, added []int64
	for _, row := range rows {
		ids = append(ids, row.ID)
		if !slices.Contains(previous, row.ID) {
			added = append(added, row.ID)
		}
	}
	favorites, err := queries.ListStrategyInstrumentsAmong(ctx, generated.ListStrategyInstrumentsAmongParams{AdministratorTelegramID: administratorTelegramID, InstrumentIds: added})
	if err != nil {
		return err
	}
	var absent []string
	for _, symbol := range symbols {
		index := slices.IndexFunc(rows, func(row generated.LockStrategySymbolsRow) bool { return row.Symbol == symbol })
		if index < 0 || (slices.Contains(added, rows[index].ID) && !slices.Contains(favorites, rows[index].ID)) {
			absent = append(absent, symbol)
		}
	}
	if len(absent) > 0 {
		return fmt.Errorf("%w: not an active coin in the administrator's favorites: %s", strategy.ErrInvalidArgument, strings.Join(absent, ", "))
	}
	return queries.InsertStrategySymbols(ctx, generated.InsertStrategySymbolsParams{StrategyID: strategyID, InstrumentIds: ids})
}

// strategiesReading locks the instrument and reports the strategies that
// read it as strategy.InstrumentsInUseError.
func strategiesReading(ctx context.Context, queries *generated.Queries, instrumentID int64) error {
	if err := queries.LockInstrument(ctx, instrumentID); err != nil {
		return fmt.Errorf("lock instrument: %w", err)
	}
	rows, err := queries.ListStrategiesReading(ctx, instrumentID)
	if err != nil {
		return fmt.Errorf("list strategies reading instrument: %w", err)
	}
	if len(rows) == 0 {
		return nil
	}
	use := strategy.InstrumentUse{Symbol: rows[0].Symbol}
	for _, row := range rows {
		use.Strategies = append(use.Strategies, row.Name)
	}
	return &strategy.InstrumentsInUseError{Uses: []strategy.InstrumentUse{use}}
}

// SetStrategyEnabled turns a strategy on or off and returns the revision. An
// enabled strategy awaits its baseline; a disabled one forgets its trading
// states.
func (store *Store) SetStrategyEnabled(ctx context.Context, id int64, enabled bool) (int64, error) {
	tx, err := store.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	queries := store.queries.WithTx(tx)
	revision, err := queries.SetStrategyEnabled(ctx, generated.SetStrategyEnabledParams{ID: id, Enabled: enabled})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, strategy.ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	if !enabled {
		if err := queries.DeleteAllStrategyStates(ctx, id); err != nil {
			return 0, err
		}
	}
	return revision, tx.Commit(ctx)
}

func (store *Store) DeleteStrategy(ctx context.Context, id int64) error {
	deleted, err := store.queries.DeleteStrategy(ctx, id)
	if err != nil {
		return err
	}
	if deleted == 0 {
		return strategy.ErrNotFound
	}
	return nil
}

// ListStrategyStates maps strategy ids to the trading states of their
// instruments.
func (store *Store) ListStrategyStates(ctx context.Context) (map[int64]map[int64]strategy.TradeState, error) {
	rows, err := store.queries.ListStrategyStates(ctx)
	if err != nil {
		return nil, err
	}
	result := map[int64]map[int64]strategy.TradeState{}
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
func (store *Store) ReplaceStrategyStates(ctx context.Context, strategyID, revision int64, states map[int64]strategy.TradeState) (bool, error) {
	tx, err := store.db.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	queries := store.queries.WithTx(tx)
	current, err := queries.CompleteStrategyBaseline(ctx, generated.CompleteStrategyBaselineParams{ID: strategyID, Revision: revision})
	if err != nil || current == 0 {
		return false, err
	}
	if err := queries.DeleteAllStrategyStates(ctx, strategyID); err != nil {
		return false, err
	}
	params := generated.InsertStrategyStatesParams{StrategyID: strategyID}
	for _, id := range slices.Sorted(maps.Keys(states)) {
		params.InstrumentIds = append(params.InstrumentIds, id)
		params.OpenTimes = append(params.OpenTimes, pgtype.Timestamptz{Time: states[id].OpenTime, Valid: true})
		params.Entries = append(params.Entries, states[id].Entry)
	}
	if err := queries.InsertStrategyStates(ctx, params); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

// SaveStrategyStates stores states of later candles than the stored ones
// while revision is current and announced, and returns the instruments
// stored.
func (store *Store) SaveStrategyStates(ctx context.Context, strategyID, revision int64, states map[int64]strategy.TradeState) ([]int64, error) {
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
	return store.queries.SaveStrategyStates(ctx, params)
}

func (store *Store) DeleteStrategyStates(ctx context.Context, strategyID int64, instrumentIDs []int64) error {
	return store.queries.DeleteStrategyStates(ctx, generated.DeleteStrategyStatesParams{StrategyID: strategyID, InstrumentIds: instrumentIDs})
}

// ListStrategyInstruments returns the active favorites of the administrator,
// which strategies evaluate.
func (store *Store) ListStrategyInstruments(ctx context.Context, administratorTelegramID int64) ([]strategy.Instrument, error) {
	rows, err := store.queries.ListStrategyInstruments(ctx, administratorTelegramID)
	if err != nil {
		return nil, err
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
	return store.queries.ListStrategyRecipients(ctx, administratorTelegramID)
}

// strategyWriteError maps a taken name to strategy.ErrConflict and an
// indicator deleted meanwhile to strategy.ErrInvalidArgument.
func strategyWriteError(err error) error {
	if duplicateViolation(err) {
		return strategy.ErrConflict
	}
	if foreignKeyViolation(err) {
		return fmt.Errorf("%w: an indicator it reads no longer exists", strategy.ErrInvalidArgument)
	}
	return err
}

func foreignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}
