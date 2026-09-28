package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"crypto-scanner/internal/analysis"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/market/marketsync"
	"crypto-scanner/internal/platform/numeric"
	generated "crypto-scanner/internal/storage/postgres/sqlc"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (store *Store) ApplyInstrumentSnapshot(ctx context.Context, items []market.Instrument) error {
	tx, err := store.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin instrument snapshot: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	queries := store.queries.WithTx(tx)
	params := generated.UpsertInstrumentsParams{
		Symbols: make([]string, len(items)), BaseAssets: make([]string, len(items)), QuoteAssets: make([]string, len(items)),
		Statuses: make([]string, len(items)), Actives: make([]bool, len(items)),
	}
	for index, item := range items {
		params.Symbols[index], params.BaseAssets[index], params.QuoteAssets[index] = item.Symbol, item.BaseAsset, item.QuoteAsset
		params.Statuses[index], params.Actives[index] = item.Status, item.Active
	}
	if err := queries.DeactivateInstrumentsExcept(ctx, params.Symbols); err != nil {
		return fmt.Errorf("deactivate instruments absent from snapshot: %w", err)
	}
	if err := queries.UpsertInstruments(ctx, params); err != nil {
		return fmt.Errorf("apply instrument snapshot: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit instrument snapshot: %w", err)
	}
	return nil
}

func (store *Store) GetActiveInstrumentBySymbol(ctx context.Context, symbol string) (market.Instrument, error) {
	row, err := store.queries.GetActiveInstrumentBySymbol(ctx, symbol)
	if errors.Is(err, pgx.ErrNoRows) {
		return market.Instrument{}, market.ErrInstrumentNotFound
	}
	if err != nil {
		return market.Instrument{}, fmt.Errorf("get active instrument by symbol: %w", err)
	}
	return instrumentFromRow(row.BinanceSpotInstrument), nil
}

func (store *Store) ListActiveInstruments(ctx context.Context) ([]market.Instrument, error) {
	rows, err := store.queries.ListActiveInstruments(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active instruments: %w", err)
	}
	items := make([]market.Instrument, 0, len(rows))
	for _, row := range rows {
		items = append(items, instrumentFromRow(row.BinanceSpotInstrument))
	}
	return items, nil
}

func (store *Store) SelectActiveInstruments(ctx context.Context, selection analysis.Selection) ([]market.Instrument, error) {
	params, err := selectionParams(selection)
	if err != nil {
		return nil, err
	}
	rows, err := store.queries.SelectActiveInstruments(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("select active instruments: %w", err)
	}
	items := make([]market.Instrument, 0, len(rows))
	for _, row := range rows {
		item := instrumentFromRow(row.BinanceSpotInstrument)
		if row.MarketCapAvailable {
			value, parseErr := numeric.ParseFinite(row.MarketCapUsd)
			if parseErr != nil {
				return nil, fmt.Errorf("invalid persisted market cap for %s", item.Symbol)
			}
			item.MarketCapUSD = &value
		}
		items = append(items, item)
	}
	return items, nil
}

func selectionParams(selection analysis.Selection) (generated.SelectActiveInstrumentsParams, error) {
	if selection.Limit < 0 || int64(selection.Limit) > math.MaxInt32 ||
		(selection.SortDirection != "" && selection.SortDirection != "asc" && selection.SortDirection != "desc") ||
		(selection.SortFact != 0 && selection.SortFact != analysis.SelectionFactMarketCapUSD) {
		return generated.SelectActiveInstrumentsParams{}, fmt.Errorf("invalid instrument selection")
	}
	params := generated.SelectActiveInstrumentsParams{
		MinimumMarketCapUsd: pgtype.Numeric{},
		MarketCapSort:       selection.SortDirection,
		ResultLimit:         int32(selection.Limit),
		Symbol:              selection.Symbol,
		Symbols:             selection.Symbols,
	}
	for _, constraint := range selection.Constraints {
		switch {
		case constraint.Fact == analysis.SelectionFactStablecoin && constraint.Operator == analysis.SelectionEqual && !constraint.Boolean:
			params.ExcludeStablecoins = true
		case constraint.Fact == analysis.SelectionFactMarketCapUSD && constraint.Operator == analysis.SelectionAtLeast && constraint.Number >= 0 && numeric.Finite(constraint.Number):
			if err := params.MinimumMarketCapUsd.ScanScientific(decimal(constraint.Number)); err != nil {
				return generated.SelectActiveInstrumentsParams{}, fmt.Errorf("invalid minimum market cap: %w", err)
			}
		default:
			return generated.SelectActiveInstrumentsParams{}, fmt.Errorf("unsupported instrument selection constraint")
		}
	}
	return params, nil
}

func instrumentFromRow(row generated.BinanceSpotInstrument) market.Instrument {
	return market.Instrument{ID: row.ID, Symbol: row.Symbol, BaseAsset: row.BaseAsset, QuoteAsset: row.QuoteAsset, Status: row.ExchangeStatus, Active: row.IsActive}
}

func (store *Store) UpsertCandles(ctx context.Context, items []market.Candle) error {
	_, err := store.UpsertCandlesWithChanges(ctx, items)
	return err
}

// UpsertCandlesWithChanges reports only rows inserted or materially corrected
// after the entire batch has committed.
func (store *Store) UpsertCandlesWithChanges(ctx context.Context, items []market.Candle) ([]market.Candle, error) {
	params := make([]generated.UpsertCandleParams, 0, len(items))
	for index, item := range items {
		values, err := candleParams(item)
		if err != nil {
			return nil, fmt.Errorf("validate candle %d: %w", index, err)
		}
		params = append(params, values)
	}
	if len(params) == 0 {
		return nil, nil
	}
	tx, err := store.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin candle upsert: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	queries := store.queries.WithTx(tx)
	// One round trip for the whole page; an unchanged row returns no row.
	changed := make([]market.Candle, 0, len(items))
	var upsertErr error
	queries.UpsertCandle(ctx, params).QueryRow(func(index int, _ pgtype.Timestamptz, err error) {
		switch {
		case upsertErr != nil || errors.Is(err, pgx.ErrNoRows):
		case err != nil:
			upsertErr = fmt.Errorf("upsert candle: %w", err)
		default:
			changed = append(changed, items[index])
		}
	})
	if upsertErr != nil {
		return nil, upsertErr
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit candle upsert: %w", err)
	}
	return changed, nil
}

// ListLatestCandles returns up to limit latest candles per instrument in
// chronological order.
func (store *Store) ListLatestCandles(ctx context.Context, instrumentIDs []int64, interval market.CandleInterval, limit int) (map[int64][]market.Candle, error) {
	if len(instrumentIDs) == 0 {
		return map[int64][]market.Candle{}, nil
	}
	if limit <= 0 || int64(limit) > math.MaxInt32 {
		return nil, fmt.Errorf("candle limit must be between 1 and %d", math.MaxInt32)
	}
	rows, err := store.queries.ListLatestCandles(ctx, generated.ListLatestCandlesParams{
		InstrumentIds: instrumentIDs,
		Interval:      string(interval),
		RowLimit:      int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list latest candles: %w", err)
	}
	items := make(map[int64][]market.Candle, len(instrumentIDs))
	for _, row := range rows {
		item, err := candleFromRow(row)
		if err != nil {
			return nil, fmt.Errorf("convert candle opened at %s: %w", row.OpenTime.Time, err)
		}
		items[row.InstrumentID] = append(items[row.InstrumentID], item)
	}
	return items, nil
}

// PruneCandles keeps the newest keep candles of interval per instrument and
// returns how many were deleted.
func (store *Store) PruneCandles(ctx context.Context, interval market.CandleInterval, keep int) (int64, error) {
	if !interval.Valid() || keep < 1 || int64(keep) > math.MaxInt32 {
		return 0, fmt.Errorf("invalid candle retention")
	}
	deleted, err := store.queries.PruneCandles(ctx, generated.PruneCandlesParams{Interval: string(interval), KeepCount: int32(keep)})
	if err != nil {
		return 0, fmt.Errorf("prune %s candles: %w", interval, err)
	}
	return deleted, nil
}

// DeleteDelistedInstruments removes, with all their candles, the instruments
// inactive since before inactiveBefore that nobody has favorited, and returns
// their symbols.
func (store *Store) DeleteDelistedInstruments(ctx context.Context, inactiveBefore time.Time) ([]string, error) {
	tx, err := store.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin delisted instrument deletion: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	queries := store.queries.WithTx(tx)
	ids, err := queries.ListDelistedInstrumentIDs(ctx, timestamptz(&inactiveBefore))
	if err != nil {
		return nil, fmt.Errorf("list delisted instruments: %w", err)
	}
	if len(ids) == 0 {
		return nil, nil
	}
	if _, err := queries.DeleteDelistedInstrumentCandles(ctx, ids); err != nil {
		return nil, fmt.Errorf("delete delisted instrument candles: %w", err)
	}
	symbols, err := queries.DeleteDelistedInstruments(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("delete delisted instruments: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit delisted instrument deletion: %w", err)
	}
	return symbols, nil
}

// SummarizeCandleHistory summarizes up to limit latest candles per
// instrument without reading candle rows.
func (store *Store) SummarizeCandleHistory(ctx context.Context, instrumentIDs []int64, interval market.CandleInterval, limit int) (map[int64]marketsync.HistorySummary, error) {
	step, ok := intervalStep(interval)
	if !ok || limit <= 0 || int64(limit) > math.MaxInt32 {
		return nil, fmt.Errorf("invalid candle history summary")
	}
	histories := make(map[int64]marketsync.HistorySummary, len(instrumentIDs))
	if len(instrumentIDs) == 0 {
		return histories, nil
	}
	rows, err := store.queries.SummarizeCandleHistory(ctx, generated.SummarizeCandleHistoryParams{
		InstrumentIds: instrumentIDs, Interval: string(interval), RowLimit: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("summarize candle history: %w", err)
	}
	for _, row := range rows {
		histories[row.InstrumentID] = marketsync.HistorySummary{
			Count: int(row.CandleCount), Oldest: row.OldestOpenTime.Time.UTC(), Latest: row.LatestOpenTime.Time.UTC(),
		}
	}
	gaps, err := store.queries.ListCandleGaps(ctx, generated.ListCandleGapsParams{
		InstrumentIds: instrumentIDs, Interval: string(interval), RowLimit: int32(limit), Step: step,
	})
	if err != nil {
		return nil, fmt.Errorf("list candle gaps: %w", err)
	}
	for _, gap := range gaps {
		history := histories[gap.InstrumentID]
		history.Gaps = append(history.Gaps, marketsync.HistoryGap{
			From: interval.NextOpenTime(gap.PreviousOpenTime.Time.UTC()), To: gap.NextOpenTime.Time.UTC(),
		})
		histories[gap.InstrumentID] = history
	}
	return histories, nil
}

// intervalStep is one interval as a PostgreSQL calendar interval.
func intervalStep(interval market.CandleInterval) (pgtype.Interval, bool) {
	switch interval {
	case market.IntervalHour:
		return pgtype.Interval{Microseconds: time.Hour.Microseconds(), Valid: true}, true
	case market.IntervalDay:
		return pgtype.Interval{Days: 1, Valid: true}, true
	case market.IntervalWeek:
		return pgtype.Interval{Days: 7, Valid: true}, true
	case market.IntervalMonth:
		return pgtype.Interval{Months: 1, Valid: true}, true
	default:
		return pgtype.Interval{}, false
	}
}

func (store *Store) GetCandleHistoryCoverage(ctx context.Context, instrumentID int64, interval market.CandleInterval) (market.HistoryCoverage, bool, error) {
	row, err := store.queries.GetCandleHistoryCoverage(ctx, generated.GetCandleHistoryCoverageParams{InstrumentID: instrumentID, Interval: string(interval)})
	if errors.Is(err, pgx.ErrNoRows) {
		return market.HistoryCoverage{}, false, nil
	}
	if err != nil {
		return market.HistoryCoverage{}, false, fmt.Errorf("get candle history coverage: %w", err)
	}
	return market.HistoryCoverage{
		InstrumentID: row.InstrumentID, Interval: market.CandleInterval(row.Interval),
		VerifiedOldestOpenTime: row.VerifiedOldestOpenTime.Time.UTC(), TargetDepth: int(row.TargetDepth),
		PolicyVersion: int(row.PolicyVersion), RetryAfter: row.RetryAfter.Time.UTC(),
	}, true, nil
}

func (store *Store) SaveCandleHistoryCoverage(ctx context.Context, coverage market.HistoryCoverage) error {
	if coverage.InstrumentID <= 0 || !coverage.Interval.Valid() || coverage.TargetDepth <= 0 || coverage.PolicyVersion <= 0 || coverage.VerifiedOldestOpenTime.IsZero() || coverage.RetryAfter.IsZero() {
		return fmt.Errorf("invalid candle history coverage")
	}
	if err := store.queries.SaveCandleHistoryCoverage(ctx, generated.SaveCandleHistoryCoverageParams{
		InstrumentID: coverage.InstrumentID, Interval: string(coverage.Interval),
		VerifiedOldestOpenTime: timestamptz(&coverage.VerifiedOldestOpenTime), TargetDepth: int32(coverage.TargetDepth),
		PolicyVersion: int32(coverage.PolicyVersion), RetryAfter: timestamptz(&coverage.RetryAfter),
	}); err != nil {
		return fmt.Errorf("save candle history coverage: %w", err)
	}
	return nil
}

func (store *Store) ListCandlePage(ctx context.Context, instrumentID int64, interval market.CandleInterval, before *time.Time, limit int) (market.CandlePage, error) {
	if instrumentID <= 0 || !interval.Valid() || limit <= 0 || int64(limit) >= math.MaxInt32 {
		return market.CandlePage{}, fmt.Errorf("invalid candle page")
	}
	rows, err := store.queries.ListCandlePage(ctx, generated.ListCandlePageParams{
		InstrumentID: instrumentID, Interval: string(interval), BeforeTime: timestamptz(before), RowLimit: int32(limit + 1),
	})
	if err != nil {
		return market.CandlePage{}, fmt.Errorf("list candle page: %w", err)
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	candles := make([]market.Candle, len(rows))
	for index, row := range rows {
		candle, convertErr := candleFromRow(row)
		if convertErr != nil {
			return market.CandlePage{}, fmt.Errorf("convert candle opened at %s: %w", row.OpenTime.Time, convertErr)
		}
		candles[len(rows)-1-index] = candle
	}
	return market.CandlePage{Candles: candles, HasMore: hasMore}, nil
}

func (store *Store) GetSyncState(ctx context.Context, profile market.SyncProfile) (market.SyncState, error) {
	row, err := store.queries.GetSyncState(ctx, profile.Key())
	if errors.Is(err, pgx.ErrNoRows) {
		return market.SyncState{Profile: profile, Status: market.SyncStatusNeverRun}, nil
	}
	if err != nil {
		return market.SyncState{}, fmt.Errorf("get synchronization state: %w", err)
	}
	return market.SyncState{
		Profile: profile, LastStartedAt: timePointer(row.LastStartedAt), LastSucceededAt: timePointer(row.LastSucceededAt),
		LastClosedOpenTime: timePointer(row.LastClosedOpenTime), Status: market.SyncStatus(row.Status), ErrorMessage: row.ErrorMessage.String,
	}, nil
}

func (store *Store) ListHourlyPrices(ctx context.Context, instrumentIDs []int64, from, to time.Time) ([]market.HourlyPrice, error) {
	rows, err := store.queries.ListHourlyPrices(ctx, generated.ListHourlyPricesParams{InstrumentIds: instrumentIDs, FromTime: timestamptz(&from), ToTime: timestamptz(&to)})
	if err != nil {
		return nil, fmt.Errorf("list hourly prices: %w", err)
	}
	prices := make([]market.HourlyPrice, 0, len(rows))
	for _, row := range rows {
		if !numeric.Finite(row.Close) {
			return nil, fmt.Errorf("invalid hourly close for instrument %d at %s", row.InstrumentID, row.OpenTime.Time)
		}
		prices = append(prices, market.HourlyPrice{InstrumentID: row.InstrumentID, OpenTime: row.OpenTime.Time.UTC(), Close: row.Close})
	}
	return prices, nil
}

func (store *Store) SaveSyncState(ctx context.Context, state market.SyncState) error {
	if err := store.queries.SaveSyncState(ctx, generated.SaveSyncStateParams{
		ProfileKey: state.Profile.Key(), LastStartedAt: timestamptz(state.LastStartedAt), LastSucceededAt: timestamptz(state.LastSucceededAt),
		LastClosedOpenTime: timestamptz(state.LastClosedOpenTime), Status: string(state.Status),
		ErrorMessage: pgtype.Text{String: state.ErrorMessage, Valid: state.ErrorMessage != ""},
	}); err != nil {
		return fmt.Errorf("save synchronization state: %w", err)
	}
	return nil
}

func candleParams(item market.Candle) (generated.UpsertCandleParams, error) {
	values := []float64{item.Open, item.High, item.Low, item.Close, item.Volume, item.QuoteAssetVolume}
	for _, value := range values {
		if !numeric.Finite(value) {
			return generated.UpsertCandleParams{}, fmt.Errorf("numeric value must be finite")
		}
	}
	return generated.UpsertCandleParams{
		InstrumentID: item.InstrumentID, Interval: string(item.Interval),
		OpenTime: pgtype.Timestamptz{Time: item.OpenTime, Valid: true}, CloseTime: pgtype.Timestamptz{Time: item.CloseTime, Valid: true},
		Open: item.Open, High: item.High, Low: item.Low, Close: item.Close,
		Volume: item.Volume, QuoteAssetVolume: item.QuoteAssetVolume, TradeCount: item.TradeCount,
	}, nil
}

// candleFromRow rejects NaN and infinities, which DOUBLE PRECISION columns
// accept and CHECK constraints do not exclude.
func candleFromRow(row generated.BinanceSpotCandle) (market.Candle, error) {
	for _, value := range []float64{row.Open, row.High, row.Low, row.Close, row.Volume, row.QuoteAssetVolume} {
		if !numeric.Finite(value) {
			return market.Candle{}, fmt.Errorf("candle value %v is not finite", value)
		}
	}
	return market.Candle{
		InstrumentID: row.InstrumentID, Interval: market.CandleInterval(row.Interval), OpenTime: row.OpenTime.Time, CloseTime: row.CloseTime.Time,
		Open: row.Open, High: row.High, Low: row.Low, Close: row.Close, Volume: row.Volume, QuoteAssetVolume: row.QuoteAssetVolume, TradeCount: row.TradeCount,
	}, nil
}
