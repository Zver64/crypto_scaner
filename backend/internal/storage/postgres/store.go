package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"crypto-scanner/internal/analysis"
	"crypto-scanner/internal/auth"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/marketcap"
	generated "crypto-scanner/internal/storage/postgres/sqlc"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Store adapts generated PostgreSQL access to application domain values.
type Store struct {
	db      *DB
	queries *generated.Queries
}

// NewStore creates the PostgreSQL application store.
func NewStore(db *DB) *Store { return &Store{db: db, queries: generated.New(db)} }

var (
	_ auth.UserStore     = (*Store)(nil)
	_ auth.AccessStore   = (*Store)(nil)
	_ market.MarketStore = (*Store)(nil)
	_ marketcap.Store    = (*Store)(nil)
)

func (store *Store) BootstrapCompleted(ctx context.Context) (bool, error) {
	value, err := store.queries.MappingBootstrapCompleted(ctx)
	if err != nil {
		return false, err
	}
	completed, ok := value.(bool)
	return completed && ok, nil
}
func (store *Store) ReplaceSnapshot(ctx context.Context, mappings []marketcap.Mapping) error {
	tx, err := store.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	q := store.queries.WithTx(tx)
	if err = q.ClearMappings(ctx); err != nil {
		return err
	}
	for _, m := range mappings {
		if err = upsertMapping(ctx, q, m); err != nil {
			return err
		}
	}
	if err = q.ReplaceMappingsAndCompleteBootstrap(ctx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (store *Store) ReplaceStablecoinClassifications(ctx context.Context, stablecoinIDs []string) error {
	if len(stablecoinIDs) == 0 {
		return fmt.Errorf("stablecoin classification snapshot is empty")
	}
	return store.queries.ReplaceStablecoinClassifications(ctx, stablecoinIDs)
}

func (store *Store) GetMapping(ctx context.Context, base string) (marketcap.Mapping, error) {
	row, err := store.queries.GetCoinGeckoMapping(ctx, base)
	if err != nil {
		return marketcap.Mapping{}, err
	}
	return marketcap.Mapping{BaseAsset: row.BaseAsset, CoinID: row.CoinID.String, QuoteAsset: row.QuoteAsset, SourceSymbol: row.SourceSymbol, Status: row.Status, Reason: row.Reason.String, ExpiresAt: timePointer(row.ExpiresAt)}, nil
}
func (store *Store) SaveMapping(ctx context.Context, m marketcap.Mapping) error {
	return upsertMapping(ctx, store.queries, m)
}
func upsertMapping(ctx context.Context, q *generated.Queries, m marketcap.Mapping) error {
	return q.UpsertCoinGeckoMapping(ctx, generated.UpsertCoinGeckoMappingParams{BaseAsset: m.BaseAsset, CoinID: pgtype.Text{String: m.CoinID, Valid: m.CoinID != ""}, QuoteAsset: m.QuoteAsset, SourceSymbol: m.SourceSymbol, Status: m.Status, Reason: pgtype.Text{String: m.Reason, Valid: m.Reason != ""}, ObservedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}, ExpiresAt: timestamptz(m.ExpiresAt)})
}
func (store *Store) GetCap(ctx context.Context, id string) (marketcap.Cap, error) {
	row, err := store.queries.GetCoinGeckoMarketCap(ctx, id)
	if err != nil {
		return marketcap.Cap{}, err
	}
	usd, err := strconv.ParseFloat(row.MarketCapUsd, 64)
	if err != nil {
		return marketcap.Cap{}, err
	}
	return marketcap.Cap{CoinID: row.CoinID, USD: usd, Available: true, FetchedAt: row.FetchedAt.Time, ObservedAt: row.ObservedAt.Time}, nil
}
func (store *Store) SaveCap(ctx context.Context, c marketcap.Cap) error {
	return store.queries.UpsertCoinGeckoMarketCap(ctx, generated.UpsertCoinGeckoMarketCapParams{CoinID: c.CoinID, MarketCapUsd: decimal(c.USD), FetchedAt: pgtype.Timestamptz{Time: c.FetchedAt, Valid: true}, ObservedAt: pgtype.Timestamptz{Time: c.ObservedAt, Valid: true}})
}

func (store *Store) FindEnabledByTelegramID(ctx context.Context, telegramID int64) (auth.User, error) {
	row, err := store.queries.FindEnabledUserByTelegramID(ctx, telegramID)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, auth.ErrUserNotFound
	}
	if err != nil {
		return auth.User{}, fmt.Errorf("find enabled user by Telegram ID: %w", err)
	}
	return auth.User{ID: row.ID, TelegramID: row.TelegramID, Username: row.Username.String, DisplayName: row.DisplayName.String, Enabled: row.IsEnabled}, nil
}

// GrantAccess creates or re-enables an application user. The boolean reports
// whether Scanner Access changed, so repeated additions remain harmless.
func (store *Store) GrantAccess(ctx context.Context, telegramID int64, username, displayName string) (auth.User, bool, error) {
	existing, err := store.FindEnabledByTelegramID(ctx, telegramID)
	if err == nil {
		return existing, false, nil
	}
	if !errors.Is(err, auth.ErrUserNotFound) {
		return auth.User{}, false, fmt.Errorf("look up user before grant: %w", err)
	}
	row, err := store.queries.GrantUserAccess(ctx, generated.GrantUserAccessParams{
		TelegramID:  telegramID,
		Username:    pgtype.Text{String: username, Valid: username != ""},
		DisplayName: pgtype.Text{String: displayName, Valid: displayName != ""},
	})
	if err != nil {
		return auth.User{}, false, fmt.Errorf("grant user access: %w", err)
	}
	return auth.User{ID: row.ID, TelegramID: row.TelegramID, Username: row.Username.String, DisplayName: row.DisplayName.String, Enabled: row.IsEnabled}, true, nil
}

// ListNonAdministratorUsers returns a deterministic page of accounts that the
// configured Administrator may remove.
func (store *Store) ListNonAdministratorUsers(ctx context.Context, administratorID int64, offset, limit int) ([]auth.User, error) {
	if offset < 0 || limit <= 0 || limit > math.MaxInt32 {
		return nil, fmt.Errorf("invalid user page")
	}
	rows, err := store.queries.ListNonAdministratorUsers(ctx, generated.ListNonAdministratorUsersParams{TelegramID: administratorID, Limit: int32(limit), Offset: int32(offset)})
	if err != nil {
		return nil, fmt.Errorf("list enabled users: %w", err)
	}
	users := make([]auth.User, 0, len(rows))
	for _, row := range rows {
		users = append(users, auth.User{ID: row.ID, TelegramID: row.TelegramID, Username: row.Username.String, DisplayName: row.DisplayName.String, Enabled: row.IsEnabled})
	}
	return users, nil
}

// DeleteUser hard-deletes precisely the selected database record. Matching the
// stable record ID prevents a stale deletion from removing a later re-add.
func (store *Store) DeleteUser(ctx context.Context, id, telegramID int64) (bool, error) {
	_, err := store.queries.DeleteUserByID(ctx, generated.DeleteUserByIDParams{ID: id, TelegramID: telegramID})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("delete user: %w", err)
	}
	return true, nil
}

// BootstrapAdministrator inserts the configured administrator only if absent.
// Existing rows, including disabled users and their timestamps, stay untouched.
// Administrative authority is always checked against configuration by telegrambot.
func (store *Store) BootstrapAdministrator(ctx context.Context, telegramID int64) error {
	if err := store.queries.BootstrapAdministrator(ctx, telegramID); err != nil {
		return fmt.Errorf("bootstrap administrator: %w", err)
	}
	return nil
}

func (store *Store) ApplyInstrumentSnapshot(ctx context.Context, items []market.Instrument) error {
	tx, err := store.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin instrument snapshot: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	queries := store.queries.WithTx(tx)
	if err := queries.DeactivateAllInstruments(ctx); err != nil {
		return fmt.Errorf("deactivate previous instrument snapshot: %w", err)
	}
	for _, item := range items {
		if _, err := queries.UpsertInstrument(ctx, generated.UpsertInstrumentParams{
			Symbol: item.Symbol, BaseAsset: item.BaseAsset, QuoteAsset: item.QuoteAsset,
			ExchangeStatus: item.Status, IsActive: item.Active,
		}); err != nil {
			return fmt.Errorf("apply instrument %q: %w", item.Symbol, err)
		}
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
	return market.Instrument{ID: row.ID, Symbol: row.Symbol, BaseAsset: row.BaseAsset, QuoteAsset: row.QuoteAsset, Status: row.ExchangeStatus, Active: row.IsActive}, nil
}

func (store *Store) ListActiveInstruments(ctx context.Context) ([]market.Instrument, error) {
	rows, err := store.queries.ListActiveInstruments(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active instruments: %w", err)
	}
	return marketInstruments(rows), nil
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
		item := market.Instrument{ID: row.ID, Symbol: row.Symbol, BaseAsset: row.BaseAsset, QuoteAsset: row.QuoteAsset, Status: row.ExchangeStatus, Active: row.IsActive}
		available, ok := row.MarketCapAvailable.(bool)
		if !ok {
			return nil, fmt.Errorf("invalid persisted market cap availability for %s", item.Symbol)
		}
		if available {
			capText, ok := row.MarketCapUsd.(string)
			if !ok {
				return nil, fmt.Errorf("invalid persisted market cap for %s", item.Symbol)
			}
			value, parseErr := strconv.ParseFloat(capText, 64)
			if parseErr != nil || math.IsNaN(value) || math.IsInf(value, 0) {
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
	}
	for _, constraint := range selection.Constraints {
		switch {
		case constraint.Fact == analysis.SelectionFactStablecoin && constraint.Operator == analysis.SelectionEqual && !constraint.Boolean:
			params.ExcludeStablecoins = true
		case constraint.Fact == analysis.SelectionFactMarketCapUSD && constraint.Operator == analysis.SelectionAtLeast && constraint.Number >= 0 && !math.IsNaN(constraint.Number) && !math.IsInf(constraint.Number, 0):
			if err := params.MinimumMarketCapUsd.ScanScientific(decimal(constraint.Number)); err != nil {
				return generated.SelectActiveInstrumentsParams{}, fmt.Errorf("invalid minimum market cap: %w", err)
			}
		default:
			return generated.SelectActiveInstrumentsParams{}, fmt.Errorf("unsupported instrument selection constraint")
		}
	}
	return params, nil
}

func marketInstruments(rows []generated.BinanceSpotInstrument) []market.Instrument {
	items := make([]market.Instrument, 0, len(rows))
	for _, row := range rows {
		items = append(items, market.Instrument{ID: row.ID, Symbol: row.Symbol, BaseAsset: row.BaseAsset, QuoteAsset: row.QuoteAsset, Status: row.ExchangeStatus, Active: row.IsActive})
	}
	return items
}

func (store *Store) UpsertCandles(ctx context.Context, items []market.Candle) error {
	params := make([]generated.UpsertCandleParams, 0, len(items))
	for index, item := range items {
		values, err := candleParams(item)
		if err != nil {
			return fmt.Errorf("validate candle %d: %w", index, err)
		}
		params = append(params, values)
	}
	if len(params) == 0 {
		return nil
	}
	tx, err := store.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin candle upsert: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	queries := store.queries.WithTx(tx)
	for _, values := range params {
		if err := queries.UpsertCandle(ctx, values); err != nil {
			return fmt.Errorf("upsert candle: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit candle upsert: %w", err)
	}
	return nil
}

func (store *Store) ListLatestCandlesByInterval(ctx context.Context, instrumentID int64, interval string, limit int) ([]market.Candle, error) {
	if limit <= 0 || int64(limit) > math.MaxInt32 {
		return nil, fmt.Errorf("candle limit must be between 1 and %d", math.MaxInt32)
	}
	rows, err := store.queries.ListLatestCandles(ctx, generated.ListLatestCandlesParams{InstrumentID: instrumentID, Interval: interval, Limit: int32(limit)})
	if err != nil {
		return nil, fmt.Errorf("list latest candles: %w", err)
	}
	items := make([]market.Candle, 0, len(rows))
	for _, row := range rows {
		item, err := candleFromRow(row)
		if err != nil {
			return nil, fmt.Errorf("convert candle opened at %s: %w", row.OpenTime.Time, err)
		}
		items = append(items, item)
	}
	return items, nil
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
		price, err := strconv.ParseFloat(row.Close, 64)
		if err != nil || math.IsNaN(price) || math.IsInf(price, 0) {
			return nil, fmt.Errorf("invalid hourly close for instrument %d at %s", row.InstrumentID, row.OpenTime.Time)
		}
		prices = append(prices, market.HourlyPrice{InstrumentID: row.InstrumentID, OpenTime: row.OpenTime.Time.UTC(), Close: price})
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

func (store *Store) DatabaseReady(ctx context.Context) bool { return store.db.Ping(ctx) == nil }

func (store *Store) MigrationsReady(ctx context.Context) bool {
	return VerifySchema(ctx, store.db, "") == nil
}

func (store *Store) SuccessfulMarketSyncExists(ctx context.Context) bool {
	exists, err := store.queries.SuccessfulMarketSyncExists(ctx)
	return err == nil && exists
}

func candleParams(item market.Candle) (generated.UpsertCandleParams, error) {
	values := []float64{item.Open, item.High, item.Low, item.Close, item.Volume, item.QuoteAssetVolume}
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return generated.UpsertCandleParams{}, fmt.Errorf("numeric value must be finite")
		}
	}
	return generated.UpsertCandleParams{
		InstrumentID: item.InstrumentID, Interval: string(item.Interval),
		OpenTime: pgtype.Timestamptz{Time: item.OpenTime, Valid: true}, CloseTime: pgtype.Timestamptz{Time: item.CloseTime, Valid: true},
		Open: decimal(item.Open), High: decimal(item.High), Low: decimal(item.Low), Close: decimal(item.Close),
		Volume: decimal(item.Volume), QuoteAssetVolume: decimal(item.QuoteAssetVolume), TradeCount: item.TradeCount,
	}, nil
}

func candleFromRow(row generated.BinanceSpotCandle) (market.Candle, error) {
	fields := []struct{ name, value string }{
		{"open", row.Open}, {"high", row.High}, {"low", row.Low}, {"close", row.Close},
		{"volume", row.Volume}, {"quote asset volume", row.QuoteAssetVolume},
	}
	converted := make([]float64, len(fields))
	for index, field := range fields {
		value, err := strconv.ParseFloat(field.value, 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			return market.Candle{}, fmt.Errorf("%s NUMERIC %q is outside finite float64 range", field.name, field.value)
		}
		converted[index] = value
	}
	return market.Candle{
		InstrumentID: row.InstrumentID, Interval: market.CandleInterval(row.Interval), OpenTime: row.OpenTime.Time, CloseTime: row.CloseTime.Time,
		Open: converted[0], High: converted[1], Low: converted[2], Close: converted[3], Volume: converted[4], QuoteAssetVolume: converted[5], TradeCount: row.TradeCount,
	}, nil
}

func decimal(value float64) string { return strconv.FormatFloat(value, 'g', -1, 64) }

func timestamptz(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *value, Valid: true}
}

func timePointer(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}
