package httpapi

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"time"

	"crypto-scanner/internal/chart"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/strategy"

	"github.com/google/uuid"
)

// Strategies manages the administrator's strategies.
type Strategies interface {
	List() []strategy.Entry
	Get(uuid.UUID) (strategy.Entry, error)
	// Catalog lists what expressions read.
	Catalog() (strategy.Catalog, error)
	Validate(ctx context.Context, expression string, kind strategy.RuleKind) (strategy.Validation, error)
	Create(context.Context, strategy.Strategy) (strategy.Entry, error)
	Update(context.Context, strategy.Strategy) (strategy.Entry, error)
	SetEnabled(ctx context.Context, id uuid.UUID, enabled bool) (strategy.Entry, error)
	Delete(context.Context, uuid.UUID) error
	Backtest(ctx context.Context, id uuid.UUID, symbol string, from, to time.Time, withChart bool) (strategy.Backtest, error)
	BacktestDraft(ctx context.Context, item strategy.Strategy, symbol string, from, to time.Time, withChart bool) (strategy.Backtest, error)
}

func (api *api) GetStrategyCatalog(ctx context.Context, _ GetStrategyCatalogRequestObject) (GetStrategyCatalogResponseObject, error) {
	catalog, err := api.Strategies.Catalog()
	if err != nil {
		return GetStrategyCatalog500JSONResponse{api.internalError(ctx, "get_strategy_catalog", err)}, nil
	}
	response := StrategyCatalog{
		Indicators: indicatorCallDTOs(catalog.Indicators), IndicatorTypes: make([]IndicatorType, len(catalog.IndicatorTypes)),
		CandleFields: catalog.CandleFields,
		Intervals:    make([]CandleInterval, len(catalog.Intervals)), PositionVariables: make([]StrategyPositionVariable, len(catalog.Positions)),
	}
	for i, descriptor := range catalog.IndicatorTypes {
		response.IndicatorTypes[i] = indicatorTypeDTO(descriptor)
	}
	for i, interval := range catalog.Intervals {
		response.Intervals[i] = CandleInterval(interval)
	}
	for i, variable := range catalog.Positions {
		response.PositionVariables[i] = StrategyPositionVariable{Name: variable.Name, Count: variable.Count}
	}
	return GetStrategyCatalog200JSONResponse(response), nil
}

func indicatorCallDTOs(calls []strategy.IndicatorCall) []StrategyIndicatorCall {
	items := make([]StrategyIndicatorCall, len(calls))
	for i, call := range calls {
		items[i] = StrategyIndicatorCall{
			Type: string(call.Selection.Type), Parameters: append([]string{}, call.Arguments...), Title: call.Title,
			Outputs: append([]string{}, call.Outputs...),
		}
	}
	return items
}

func (api *api) ValidateStrategy(ctx context.Context, request ValidateStrategyRequestObject) (ValidateStrategyResponseObject, error) {
	kind := strategy.EntryRule
	if request.Body.Kind != nil {
		switch *request.Body.Kind {
		case StrategyValidationInputKindExit:
			kind = strategy.ExitRule
		case StrategyValidationInputKindPrice:
			kind = strategy.PriceRule
		}
	}
	validation, err := api.Strategies.Validate(ctx, request.Body.Expression, kind)
	if err != nil {
		return ValidateStrategy500JSONResponse{api.internalError(ctx, "validate_strategy", err)}, nil
	}
	return ValidateStrategy200JSONResponse{Errors: validation.Problems, Expression: validation.Expression, Indicators: indicatorCallDTOs(validation.Indicators)}, nil
}

func (api *api) ListStrategies(context.Context, ListStrategiesRequestObject) (ListStrategiesResponseObject, error) {
	entries := api.Strategies.List()
	items := make([]Strategy, len(entries))
	for i, entry := range entries {
		items[i] = strategyDTO(entry)
	}
	return ListStrategies200JSONResponse{Items: items}, nil
}

func (api *api) GetStrategy(ctx context.Context, request GetStrategyRequestObject) (GetStrategyResponseObject, error) {
	entry, err := api.Strategies.Get(request.StrategyId)
	switch {
	case err == nil:
		return GetStrategy200JSONResponse(strategyDTO(entry)), nil
	case errors.Is(err, strategy.ErrNotFound):
		return GetStrategy404JSONResponse{strategyNotFound(ctx)}, nil
	default:
		return GetStrategy500JSONResponse{api.internalError(ctx, "get_strategy", err)}, nil
	}
}

func (api *api) CreateStrategy(ctx context.Context, request CreateStrategyRequestObject) (CreateStrategyResponseObject, error) {
	item := strategyFromDTO(request.Body.Rules, request.Body.Coins)
	item.Name, item.Message, item.Enabled = request.Body.Name, request.Body.Message, request.Body.Enabled
	entry, err := api.Strategies.Create(ctx, item)
	switch {
	case err == nil:
		return CreateStrategy201JSONResponse(strategyDTO(entry)), nil
	case errors.Is(err, strategy.ErrInvalidArgument):
		return CreateStrategy400JSONResponse{strategyInvalid(ctx, err)}, nil
	case errors.Is(err, strategy.ErrConflict):
		return CreateStrategy409JSONResponse{strategyConflict(ctx)}, nil
	default:
		return CreateStrategy500JSONResponse{api.internalError(ctx, "create_strategy", err)}, nil
	}
}

func (api *api) UpdateStrategy(ctx context.Context, request UpdateStrategyRequestObject) (UpdateStrategyResponseObject, error) {
	item := strategyFromDTO(request.Body.Rules, request.Body.Coins)
	item.ID, item.Name, item.Message = request.StrategyId, request.Body.Name, request.Body.Message
	entry, err := api.Strategies.Update(ctx, item)
	switch {
	case err == nil:
		return UpdateStrategy200JSONResponse(strategyDTO(entry)), nil
	case errors.Is(err, strategy.ErrInvalidArgument):
		return UpdateStrategy400JSONResponse{strategyInvalid(ctx, err)}, nil
	case errors.Is(err, strategy.ErrNotFound):
		return UpdateStrategy404JSONResponse{strategyNotFound(ctx)}, nil
	case errors.Is(err, strategy.ErrConflict):
		return UpdateStrategy409JSONResponse{strategyConflict(ctx)}, nil
	default:
		return UpdateStrategy500JSONResponse{api.internalError(ctx, "update_strategy", err)}, nil
	}
}

func (api *api) SetStrategyEnabled(ctx context.Context, request SetStrategyEnabledRequestObject) (SetStrategyEnabledResponseObject, error) {
	entry, err := api.Strategies.SetEnabled(ctx, request.StrategyId, request.Body.Enabled)
	switch {
	case err == nil:
		return SetStrategyEnabled200JSONResponse(strategyDTO(entry)), nil
	case errors.Is(err, strategy.ErrInvalidArgument):
		return SetStrategyEnabled400JSONResponse{strategyInvalid(ctx, err)}, nil
	case errors.Is(err, strategy.ErrNotFound):
		return SetStrategyEnabled404JSONResponse{strategyNotFound(ctx)}, nil
	default:
		return SetStrategyEnabled500JSONResponse{api.internalError(ctx, "set_strategy_enabled", err)}, nil
	}
}

func (api *api) DeleteStrategy(ctx context.Context, request DeleteStrategyRequestObject) (DeleteStrategyResponseObject, error) {
	err := api.Strategies.Delete(ctx, request.StrategyId)
	switch {
	case err == nil:
		return DeleteStrategy204Response{}, nil
	case errors.Is(err, strategy.ErrNotFound):
		return DeleteStrategy404JSONResponse{strategyNotFound(ctx)}, nil
	default:
		return DeleteStrategy500JSONResponse{api.internalError(ctx, "delete_strategy", err)}, nil
	}
}

func (api *api) BacktestStrategy(ctx context.Context, request BacktestStrategyRequestObject) (BacktestStrategyResponseObject, error) {
	body := request.Body
	backtested, err := body.Strategy.ValueByDiscriminator()
	if err != nil {
		return BacktestStrategy500JSONResponse{api.internalError(ctx, "backtest_strategy", err)}, nil
	}
	timed, cancel := context.WithTimeout(ctx, backtestTimeout)
	defer cancel()
	from, to := backtestPeriod(body.From, body.To)
	withChart := body.Chart != nil && *body.Chart
	started := time.Now()
	var backtest strategy.Backtest
	switch backtested := backtested.(type) {
	case SavedBacktestedStrategy:
		backtest, err = api.Strategies.Backtest(timed, backtested.Id, body.Symbol, from, to, withChart)
	case DraftBacktestedStrategy:
		backtest, err = api.Strategies.BacktestDraft(timed, strategyFromDTO(backtested.Rules, StrategyCoins{}), body.Symbol, from, to, withChart)
	}
	switch {
	case err == nil:
		dto, err := backtestDTO(backtest)
		if err != nil {
			return BacktestStrategy500JSONResponse{api.internalError(ctx, "backtest_strategy", err)}, nil
		}
		dto.DurationMs = float64(time.Since(started)) / float64(time.Millisecond)
		return BacktestStrategy200JSONResponse(dto), nil
	case backtestExpired(timed, err):
		return BacktestStrategy503JSONResponse{backtestTooHeavy(ctx)}, nil
	case errors.Is(err, strategy.ErrBacktestBusy):
		return BacktestStrategy429JSONResponse{backtestBusy(ctx)}, nil
	case errors.Is(err, strategy.ErrInvalidArgument):
		return BacktestStrategy400JSONResponse{strategyInvalid(ctx, err)}, nil
	case errors.Is(err, strategy.ErrNotFound):
		return BacktestStrategy404JSONResponse(strategyNotFound(ctx)), nil
	case errors.Is(err, market.ErrInstrumentNotFound):
		return BacktestStrategy404JSONResponse(symbolNotFound(ctx).body), nil
	default:
		return BacktestStrategy500JSONResponse{api.internalError(ctx, "backtest_strategy", err)}, nil
	}
}

// backtestPeriod reads the optional bounds of a backtest in UTC, zero when
// absent.
func backtestPeriod(fromBound, toBound *time.Time) (from, to time.Time) {
	if fromBound != nil {
		from = fromBound.UTC()
	}
	if toBound != nil {
		to = toBound.UTC()
	}
	return from, to
}

func backtestDTO(backtest strategy.Backtest) (StrategyBacktest, error) {
	trades := make([]BacktestTrade, len(backtest.Trades))
	for i, trade := range backtest.Trades {
		trades[i] = backtestTradeDTO(trade)
	}
	equity := make([]BacktestEquityPoint, len(backtest.Equity))
	for i, point := range backtest.Equity {
		equity[i] = BacktestEquityPoint{Time: point.Time, Equity: point.Equity}
	}
	columns := make([]BacktestIndicatorColumn, len(backtest.IndicatorColumns))
	for i, column := range backtest.IndicatorColumns {
		columns[i] = BacktestIndicatorColumn{Key: column.Key, Title: column.Title}
	}
	charts := make([]BacktestChart, len(backtest.Charts))
	for i, snapshot := range backtest.Charts {
		page, err := chartWirePage(chart.Page{Symbol: backtest.Symbol, Candles: snapshot.Candles, Shapes: snapshot.Shapes, Panes: snapshot.Panes}, snapshot.Interval)
		if err != nil {
			return StrategyBacktest{}, err
		}
		charts[i] = BacktestChart{Page: *page}
	}
	dto := StrategyBacktest{
		IndicatorColumns: columns, Charts: charts,
		Interval: CandleInterval(backtest.Interval), Symbol: backtest.Symbol, Direction: Direction(backtest.Direction), Fee: strategy.BacktestFee,
		Trades: trades, SkippedAlerts: backtest.Skipped, Equity: equity,
		Summary:   BacktestSummary{NetProfit: backtest.NetProfit, MaxDrawdown: backtest.MaxDrawdown, Stats: tradeStatsDTO(backtest.Stats)},
		Baselines: BacktestBaselines{BuyAndHold: backtest.BuyAndHold, Dca: backtest.DCA},
	}
	if !backtest.From.IsZero() {
		dto.From, dto.To = &backtest.From, &backtest.To
	}
	if report := backtest.Signal; report != nil {
		occurrences := make([]BacktestSignalOccurrence, len(report.Occurrences))
		for i, occurrence := range report.Occurrences {
			occurrences[i] = BacktestSignalOccurrence{Time: occurrence.Time, Close: occurrence.Close, Values: backtestValues(occurrence.Values), IndicatorValues: backtestValues(occurrence.IndicatorValues)}
			if evaluation := occurrence.Evaluation; evaluation != nil {
				occurrences[i].ExitTime = &evaluation.ExitTime
				occurrences[i].Counted, occurrences[i].Outcome = &evaluation.Counted, new(BacktestSignalOutcome(evaluation.Outcome))
				occurrences[i].Stop, occurrences[i].Target, occurrences[i].Move = evaluation.Stop, &evaluation.Target, &evaluation.Move
			}
		}
		dto.Signal = &BacktestSignal{Occurrences: occurrences, Candles: report.Candles}
		if judgement := report.Judgement; judgement != nil {
			dto.Signal.Evaluation = &BacktestSignalEvaluation{
				Window: judgement.Window, TargetRatio: judgement.TargetRatio, Evaluated: judgement.Evaluated,
				Signals: signalStatsDTO(judgement.Signals), All: signalStatsDTO(judgement.All),
			}
		}
	}
	return dto, nil
}

func signalStatsDTO(stats strategy.SignalStats) BacktestSignalStats {
	return BacktestSignalStats{Count: stats.Count, Successes: stats.Successes, Neutrals: stats.Neutrals, SuccessRate: stats.SuccessRate(), MedianMove: stats.MedianMove}
}

// fromNullable reads a nullable enum of the contract as an int, 0 for null.
func fromNullable[T ~int](value *T) int {
	if value == nil {
		return 0
	}
	return int(*value)
}

// toNullable writes an int as a nullable enum of the contract, null for 0.
func toNullable[T ~int](value int) *T {
	if value == 0 {
		return nil
	}
	return new(T(value))
}

func backtestTradeDTO(trade strategy.Trade) BacktestTrade {
	fills := make([]BacktestFill, len(trade.Fills))
	for i, fill := range trade.Fills {
		fills[i] = BacktestFill{SignalTime: fill.Signal, Time: fill.Time, Price: fill.Price, Values: backtestValues(fill.Values)}
	}
	dto := BacktestTrade{
		EntryTime: trade.EntryTime, EntryPrice: trade.EntryPrice, ExitTime: trade.ExitTime, ExitPrice: trade.ExitPrice,
		Buys: trade.Buys, Open: trade.Open, NetReturn: trade.Return, Fills: fills, ExitValues: backtestValues(trade.ExitValues),
	}
	if trade.TakeProfit > 0 {
		dto.TakeProfit = &trade.TakeProfit
	}
	if trade.StopLoss > 0 {
		dto.StopLoss = &trade.StopLoss
	}
	if !trade.Open {
		dto.ExitReason = new(BacktestTradeExitReason(trade.Reason))
		dto.ExitSignalTime = &trade.ExitSignal
	}
	return dto
}

// backtestValues serializes missing values as an empty object.
func backtestValues(values map[string]float64) BacktestValues {
	if values == nil {
		return BacktestValues{}
	}
	return values
}

func tradeStatsDTO(stats strategy.TradeStats) BacktestTradeStats {
	return BacktestTradeStats{
		TradeCount: stats.Count, WinRate: stats.WinRate, ProfitFactor: stats.ProfitFactor,
		AverageTrade: stats.AverageTrade, AverageWin: stats.AverageWin, AverageLoss: stats.AverageLoss,
		AverageBars: stats.AverageBars, TakeProfitExits: stats.TakeProfits, StopLossExits: stats.StopLosses, ExitRuleExits: stats.ExitRules,
	}
}

// backtestTimeout bounds a backtest well below the server's read and write
// timeouts: once the read timeout passes, the server's background read
// cancels the request context, and the response must still be written.
const backtestTimeout = min(readTimeout, writeTimeout) - 10*time.Second

// backtestExpired reports that a backtest failed because its time limit
// passed, which is the request's weight, not a server fault.
func backtestExpired(timed context.Context, err error) bool {
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(timed.Err(), context.DeadlineExceeded)
}

func backtestTooHeavy(ctx context.Context) BacktestTooHeavyJSONResponse {
	return BacktestTooHeavyJSONResponse(newAPIError(ctx, http.StatusServiceUnavailable, "backtest_too_heavy", "The backtest did not finish within its time limit", nil).body)
}

func backtestBusy(ctx context.Context) BacktestBusyJSONResponse {
	return BacktestBusyJSONResponse(newAPIError(ctx, http.StatusTooManyRequests, "backtest_busy", "Two backtests already run; retry once one ends", nil).body)
}

func strategyNotFound(ctx context.Context) StrategyNotFoundJSONResponse {
	return StrategyNotFoundJSONResponse(newAPIError(ctx, http.StatusNotFound, "strategy_not_found", "Strategy does not exist", nil).body)
}

func strategyConflict(ctx context.Context) StrategyConflictJSONResponse {
	return StrategyConflictJSONResponse(newAPIError(ctx, http.StatusConflict, "strategy_exists", "Another strategy has this name", nil).body)
}

// strategyInvalid reports err, an invalid strategy, with its problems.
func strategyInvalid(ctx context.Context, err error) StrategyInvalidJSONResponse {
	var invalid *strategy.InvalidError
	if !errors.As(err, &invalid) {
		invalid = &strategy.InvalidError{Problems: []string{err.Error()}}
	}
	return StrategyInvalidJSONResponse{Error: StrategyInvalidError{
		Code: StrategyInvalidErrorCodeInvalidArgument, Message: invalid.Error(),
		Details: &StrategyProblems{Problems: slices.Clone(invalid.Problems)},
	}, RequestId: RequestIdentifier(ctx)}
}

// strategyFromDTO reads the rules and the coins of a strategy.
func strategyFromDTO(rules StrategyRules, coins StrategyCoins) strategy.Strategy {
	return strategy.Strategy{
		Signal: rules.Signal, Direction: strategy.Direction(rules.Direction), Expression: rules.Expression,
		ExitExpression: rules.ExitExpression, TakeProfitExpression: rules.TakeProfitExpression, StopLossExpression: rules.StopLossExpression,
		TargetRatio: fromNullable(rules.TargetRatio), Window: fromNullable(rules.Window),
		Coins: coins.Symbols, CoinSource: strategy.CoinSource(coins.Source),
		MarketCap: strategy.MarketCapRange{MinUSD: coins.MinMarketCapUsd, MaxUSD: coins.MaxMarketCapUsd},
	}
}

// strategyDTO writes a strategy with its rules and coins.
func strategyDTO(entry strategy.Entry) Strategy {
	dto := Strategy{
		Id: entry.ID, Name: entry.Name,
		Rules: StrategyRules{
			Signal: entry.Signal, Direction: Direction(entry.Direction), Expression: entry.Expression,
			ExitExpression: entry.ExitExpression, TakeProfitExpression: entry.TakeProfitExpression, StopLossExpression: entry.StopLossExpression,
			TargetRatio: toNullable[SignalTargetRatio](entry.TargetRatio), Window: toNullable[SignalWindow](entry.Window),
		},
		Coins: StrategyCoins{
			Source: StrategyCoinSource(entry.CoinSource), Symbols: append([]string{}, entry.Coins...),
			MinMarketCapUsd: entry.MarketCap.MinUSD, MaxMarketCapUsd: entry.MarketCap.MaxUSD,
		},
		Message: entry.Message, Enabled: entry.Enabled, Valid: entry.Compiled != nil,
	}
	if entry.Problem != "" {
		dto.Problem = &entry.Problem
	}
	return dto
}
