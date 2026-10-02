package postgres

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	generated "crypto-scanner/internal/storage/postgres/sqlc"
	"crypto-scanner/internal/strategy"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (store *Store) ListStrategies(ctx context.Context) ([]strategy.Strategy, error) {
	rows, err := store.queries.ListStrategies(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]strategy.Strategy, len(rows))
	for i, row := range rows {
		items[i] = strategy.Strategy{ID: row.ID, Name: row.Name, Expression: row.Expression, Enabled: row.Enabled, BaselinePending: row.BaselinePending, Revision: row.Revision}
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
	id, err := queries.InsertStrategy(ctx, generated.InsertStrategyParams{Name: item.Name, Expression: item.Expression, Enabled: item.Enabled})
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

// UpdateStrategy replaces the name, expression, indicators, and instruments
// and returns the revision; baseline requests an announcement of the matches
// of the new expression.
func (store *Store) UpdateStrategy(ctx context.Context, item strategy.Strategy, indicatorIDs []int64, symbols []string, administratorTelegramID int64, baseline bool) (int64, error) {
	tx, err := store.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	queries := store.queries.WithTx(tx)
	revision, err := queries.UpdateStrategy(ctx, generated.UpdateStrategyParams{ID: item.ID, Name: item.Name, Expression: item.Expression, Baseline: baseline})
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
		return err
	}
	rows, err := queries.ListStrategiesReading(ctx, instrumentID)
	if err != nil || len(rows) == 0 {
		return err
	}
	use := strategy.InstrumentUse{Symbol: rows[0].Symbol}
	for _, row := range rows {
		use.Strategies = append(use.Strategies, row.Name)
	}
	return &strategy.InstrumentsInUseError{Uses: []strategy.InstrumentUse{use}}
}

// SetStrategyEnabled turns a strategy on or off and returns the revision. An
// enabled strategy awaits the announcement of its matches; a disabled one
// forgets them.
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
		if err := queries.DeleteAllStrategyMatches(ctx, id); err != nil {
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

func (store *Store) ListStrategyMatches(ctx context.Context) (map[int64][]int64, error) {
	rows, err := store.queries.ListStrategyMatches(ctx)
	if err != nil {
		return nil, err
	}
	result := map[int64][]int64{}
	for _, row := range rows {
		result[row.StrategyID] = append(result[row.StrategyID], row.InstrumentID)
	}
	return result, nil
}

// ReplaceStrategyMatches stores the matches announced for revision and
// completes its pending baseline. It reports false, changing nothing, when
// that revision no longer awaits a baseline.
func (store *Store) ReplaceStrategyMatches(ctx context.Context, strategyID, revision int64, instrumentIDs []int64) (bool, error) {
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
	if err := queries.DeleteAllStrategyMatches(ctx, strategyID); err != nil {
		return false, err
	}
	if err := queries.InsertStrategyMatches(ctx, generated.InsertStrategyMatchesParams{StrategyID: strategyID, InstrumentIds: instrumentIDs}); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

// AddStrategyMatches adds matches while revision is current and announced,
// and returns the instruments that were not matching before.
func (store *Store) AddStrategyMatches(ctx context.Context, strategyID, revision int64, instrumentIDs []int64) ([]int64, error) {
	return store.queries.AddStrategyMatches(ctx, generated.AddStrategyMatchesParams{StrategyID: strategyID, Revision: revision, InstrumentIds: instrumentIDs})
}

func (store *Store) DeleteStrategyMatches(ctx context.Context, strategyID int64, instrumentIDs []int64) error {
	return store.queries.DeleteStrategyMatches(ctx, generated.DeleteStrategyMatchesParams{StrategyID: strategyID, InstrumentIds: instrumentIDs})
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
