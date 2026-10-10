package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"crypto-scanner/internal/chart"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/scannerindicator"
	"crypto-scanner/internal/strategy"
)

// Strategies manages the administrator's strategies.
type Strategies interface {
	List() []strategy.Entry
	Variables() []strategy.Variable
	Symbols(context.Context) ([]string, error)
	Validate(ctx context.Context, expression string, kind strategy.RuleKind) (strategy.Validation, error)
	// Unconfigured lists the indicators entry reads that are not configured.
	Unconfigured(entry strategy.Entry) []scannerindicator.Entry
	Create(context.Context, strategy.Strategy) (strategy.Entry, error)
	Update(context.Context, strategy.Strategy) (strategy.Entry, error)
	SetEnabled(ctx context.Context, id int64, enabled bool) (strategy.Entry, error)
	Delete(context.Context, int64) error
	Backtest(ctx context.Context, id int64, symbol string, from, to time.Time, withChart bool) (strategy.Backtest, error)
	BacktestDraft(ctx context.Context, item strategy.Strategy, symbol string, from, to time.Time, withChart bool) (strategy.Backtest, error)
}

func (api *api) ListStrategyVariables(context.Context, ListStrategyVariablesRequestObject) (ListStrategyVariablesResponseObject, error) {
	variables := append(api.strategies.Variables(), strategy.PositionVariables()...)
	items := make([]StrategyVariable, len(variables))
	for i, variable := range variables {
		items[i] = StrategyVariable{Name: variable.Name, Label: variable.Label, Position: variable.Position}
		if !variable.Position {
			items[i].Interval = new(CandleInterval(variable.Target.Interval))
		}
		if variable.IndicatorID != 0 {
			items[i].IndicatorId = &variable.IndicatorID
		}
	}
	return ListStrategyVariables200JSONResponse{Items: items}, nil
}

func (api *api) ListStrategySymbols(ctx context.Context, _ ListStrategySymbolsRequestObject) (ListStrategySymbolsResponseObject, error) {
	symbols, err := api.strategies.Symbols(ctx)
	if err != nil {
		return ListStrategySymbols500JSONResponse{api.internalError(ctx, "list_strategy_symbols", err)}, nil
	}
	return ListStrategySymbols200JSONResponse{Items: symbols}, nil
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
	validation, err := api.strategies.Validate(ctx, request.Body.Expression, kind)
	if err != nil {
		return ValidateStrategy500JSONResponse{api.internalError(ctx, "validate_strategy", err)}, nil
	}
	return ValidateStrategy200JSONResponse{Errors: validation.Problems, MissingIndicators: missingIndicatorDTOs(validation.Missing)}, nil
}

func (api *api) ListStrategies(context.Context, ListStrategiesRequestObject) (ListStrategiesResponseObject, error) {
	entries := api.strategies.List()
	items := make([]Strategy, len(entries))
	for i, entry := range entries {
		items[i] = api.strategyDTO(entry)
	}
	return ListStrategies200JSONResponse{Items: items}, nil
}

func (api *api) CreateStrategy(ctx context.Context, request CreateStrategyRequestObject) (CreateStrategyResponseObject, error) {
	body := request.Body
	entry, err := api.strategies.Create(ctx, strategy.Strategy{
		Name: body.Name, Signal: bool(body.Signal), Direction: strategy.Direction(body.Direction), Expression: body.Expression, ExitExpression: body.ExitExpression,
		TakeProfitExpression: body.TakeProfitExpression, StopLossExpression: body.StopLossExpression,
		MarketCap:   strategy.MarketCapRange{MinUSD: body.MinMarketCapUsd, MaxUSD: body.MaxMarketCapUsd},
		TargetRatio: fromNullable(body.TargetRatio), Window: fromNullable(body.Window),
		Message: body.Message, Enabled: body.Enabled,
	})
	switch {
	case err == nil:
		return CreateStrategy201JSONResponse(api.strategyDTO(entry)), nil
	case errors.Is(err, strategy.ErrInvalidArgument):
		return CreateStrategy400JSONResponse{invalidArgument(ctx, err.Error()).badRequest()}, nil
	case errors.Is(err, strategy.ErrConflict):
		return CreateStrategy409JSONResponse{strategyConflict(ctx)}, nil
	default:
		return CreateStrategy500JSONResponse{api.internalError(ctx, "create_strategy", err)}, nil
	}
}

func (api *api) UpdateStrategy(ctx context.Context, request UpdateStrategyRequestObject) (UpdateStrategyResponseObject, error) {
	body := request.Body
	entry, err := api.strategies.Update(ctx, strategy.Strategy{
		ID: request.StrategyId, Name: body.Name, Signal: bool(body.Signal), Direction: strategy.Direction(body.Direction), Expression: body.Expression, ExitExpression: body.ExitExpression,
		TakeProfitExpression: body.TakeProfitExpression, StopLossExpression: body.StopLossExpression,
		MarketCap:   strategy.MarketCapRange{MinUSD: body.MinMarketCapUsd, MaxUSD: body.MaxMarketCapUsd},
		TargetRatio: fromNullable(body.TargetRatio), Window: fromNullable(body.Window),
		Message: body.Message,
	})
	switch {
	case err == nil:
		return UpdateStrategy200JSONResponse(api.strategyDTO(entry)), nil
	case errors.Is(err, strategy.ErrInvalidArgument):
		return UpdateStrategy400JSONResponse{invalidArgument(ctx, err.Error()).badRequest()}, nil
	case errors.Is(err, strategy.ErrNotFound):
		return UpdateStrategy404JSONResponse{strategyNotFound(ctx)}, nil
	case errors.Is(err, strategy.ErrConflict):
		return UpdateStrategy409JSONResponse{strategyConflict(ctx)}, nil
	default:
		return UpdateStrategy500JSONResponse{api.internalError(ctx, "update_strategy", err)}, nil
	}
}

func (api *api) SetStrategyEnabled(ctx context.Context, request SetStrategyEnabledRequestObject) (SetStrategyEnabledResponseObject, error) {
	entry, err := api.strategies.SetEnabled(ctx, request.StrategyId, request.Body.Enabled)
	switch {
	case err == nil:
		return SetStrategyEnabled200JSONResponse(api.strategyDTO(entry)), nil
	case errors.Is(err, strategy.ErrNotFound):
		return SetStrategyEnabled404JSONResponse{strategyNotFound(ctx)}, nil
	default:
		return SetStrategyEnabled500JSONResponse{api.internalError(ctx, "set_strategy_enabled", err)}, nil
	}
}

func (api *api) DeleteStrategy(ctx context.Context, request DeleteStrategyRequestObject) (DeleteStrategyResponseObject, error) {
	err := api.strategies.Delete(ctx, request.StrategyId)
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
	timed, cancel := context.WithTimeout(ctx, backtestTimeout)
	defer cancel()
	from, to := backtestPeriod(request.Params.From, request.Params.To)
	started := time.Now()
	backtest, err := api.strategies.Backtest(timed, request.StrategyId, request.Params.Symbol, from, to, request.Params.Chart != nil && *request.Params.Chart)
	switch {
	case err == nil:
		dto := backtestDTO(backtest)
		dto.DurationMs = float64(time.Since(started)) / float64(time.Millisecond)
		return BacktestStrategy200JSONResponse(dto), nil
	case backtestExpired(timed, err):
		return BacktestStrategy503JSONResponse{backtestTooHeavy(ctx)}, nil
	case errors.Is(err, strategy.ErrBacktestBusy):
		return BacktestStrategy429JSONResponse{backtestBusy(ctx)}, nil
	case errors.Is(err, strategy.ErrInvalidArgument):
		return BacktestStrategy400JSONResponse{invalidArgument(ctx, err.Error()).badRequest()}, nil
	case errors.Is(err, strategy.ErrNotFound):
		return backtestNotFound(ErrorResponse(strategyNotFound(ctx))), nil
	case errors.Is(err, market.ErrInstrumentNotFound):
		return backtestNotFound(symbolNotFound(ctx).body), nil
	default:
		return BacktestStrategy500JSONResponse{api.internalError(ctx, "backtest_strategy", err)}, nil
	}
}

func (api *api) BacktestStrategyDraft(ctx context.Context, request BacktestStrategyDraftRequestObject) (BacktestStrategyDraftResponseObject, error) {
	timed, cancel := context.WithTimeout(ctx, backtestTimeout)
	defer cancel()
	body := request.Body
	from, to := backtestPeriod(body.From, body.To)
	started := time.Now()
	backtest, err := api.strategies.BacktestDraft(timed, strategy.Strategy{
		Signal: bool(body.Signal), Direction: strategy.Direction(body.Direction), Expression: body.Expression, ExitExpression: body.ExitExpression,
		TakeProfitExpression: body.TakeProfitExpression, StopLossExpression: body.StopLossExpression,
		TargetRatio: fromNullable(body.TargetRatio), Window: fromNullable(body.Window),
	}, body.Symbol, from, to, body.Chart != nil && *body.Chart)
	switch {
	case err == nil:
		dto := backtestDTO(backtest)
		dto.DurationMs = float64(time.Since(started)) / float64(time.Millisecond)
		return BacktestStrategyDraft200JSONResponse(dto), nil
	case backtestExpired(timed, err):
		return BacktestStrategyDraft503JSONResponse{backtestTooHeavy(ctx)}, nil
	case errors.Is(err, strategy.ErrBacktestBusy):
		return BacktestStrategyDraft429JSONResponse{backtestBusy(ctx)}, nil
	case errors.Is(err, strategy.ErrInvalidArgument):
		return BacktestStrategyDraft400JSONResponse{invalidArgument(ctx, err.Error()).badRequest()}, nil
	case errors.Is(err, market.ErrInstrumentNotFound):
		return BacktestStrategyDraft404JSONResponse{symbolNotFound(ctx).symbolNotFound()}, nil
	default:
		return BacktestStrategyDraft500JSONResponse{api.internalError(ctx, "backtest_strategy_draft", err)}, nil
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

func backtestDTO(backtest strategy.Backtest) StrategyBacktest {
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
		catalog := make([]ChartIndicatorDefinition, len(snapshot.Definitions))
		for j, definition := range snapshot.Definitions {
			catalog[j] = chartIndicatorDTO(definition)
		}
		page := chartWirePage(chart.Page{Symbol: backtest.Symbol, Candles: snapshot.Candles, Indicators: snapshot.Indicators}, snapshot.Interval, time.Time{})
		charts[i] = BacktestChart{Catalog: catalog, Page: *page}
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
				occurrences[i].Counted, occurrences[i].Success = &evaluation.Counted, &evaluation.Success
				occurrences[i].Stop, occurrences[i].Target, occurrences[i].Move = evaluation.Stop, &evaluation.Target, &evaluation.Move
			}
		}
		dto.Signal = &BacktestSignal{
			Window: report.Window, TargetRatio: report.TargetRatio, Occurrences: occurrences, Evaluated: report.Evaluated,
			Signals: signalStatsDTO(report.Signals), All: signalStatsDTO(report.All),
		}
	}
	return dto
}

func signalStatsDTO(stats strategy.SignalStats) BacktestSignalStats {
	return BacktestSignalStats{Count: stats.Count, Successes: stats.Successes, MedianMove: stats.MedianMove}
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
	body := newAPIError(ctx, http.StatusServiceUnavailable, "backtest_too_heavy", "The backtest did not finish within its time limit", nil).body
	return BacktestTooHeavyJSONResponse{Body: body, Headers: BacktestTooHeavyResponseHeaders{XRequestID: body.RequestId}}
}

func backtestBusy(ctx context.Context) BacktestBusyJSONResponse {
	body := newAPIError(ctx, http.StatusTooManyRequests, "backtest_busy", "Two backtests already run; retry once one ends", nil).body
	return BacktestBusyJSONResponse{Body: body, Headers: BacktestBusyResponseHeaders{XRequestID: body.RequestId}}
}

func backtestNotFound(body ErrorResponse) BacktestStrategy404JSONResponse {
	return BacktestStrategy404JSONResponse{Body: body, Headers: BacktestStrategy404ResponseHeaders{XRequestID: body.RequestId}}
}

func strategyNotFound(ctx context.Context) StrategyNotFoundJSONResponse {
	return StrategyNotFoundJSONResponse(newAPIError(ctx, http.StatusNotFound, "strategy_not_found", "Strategy does not exist", nil).body)
}

func strategyConflict(ctx context.Context) StrategyConflictJSONResponse {
	return StrategyConflictJSONResponse(newAPIError(ctx, http.StatusConflict, "strategy_exists", "Another strategy has this name", nil).body)
}

func (api *api) strategyDTO(entry strategy.Entry) Strategy {
	dto := Strategy{
		Id: entry.ID, Name: entry.Name, Signal: StrategySignal(entry.Signal), Direction: Direction(entry.Direction), Expression: entry.Expression, ExitExpression: entry.ExitExpression,
		TakeProfitExpression: entry.TakeProfitExpression, StopLossExpression: entry.StopLossExpression,
		MinMarketCapUsd: entry.MarketCap.MinUSD, MaxMarketCapUsd: entry.MarketCap.MaxUSD,
		TargetRatio: toNullable[SignalTargetRatio](entry.TargetRatio), Window: toNullable[SignalWindow](entry.Window),
		Message: entry.Message, Enabled: entry.Enabled, Valid: entry.Compiled != nil,
		MissingIndicators: missingIndicatorDTOs(api.strategies.Unconfigured(entry)),
	}
	if entry.Problem != "" {
		dto.Problem = &entry.Problem
	}
	return dto
}

func missingIndicatorDTOs(entries []scannerindicator.Entry) []StrategyMissingIndicator {
	missing := make([]StrategyMissingIndicator, len(entries))
	for i, entry := range entries {
		missing[i] = StrategyMissingIndicator{Interval: CandleInterval(entry.Interval), Type: string(entry.Selection.Type), Parameters: entry.Selection.Parameters, Title: entry.Title}
	}
	return missing
}
