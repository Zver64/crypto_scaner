package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"crypto-scanner/internal/market"
	"crypto-scanner/internal/strategy"
)

// Strategies manages the administrator's strategies.
type Strategies interface {
	List() []strategy.Entry
	Variables() []strategy.Variable
	Symbols(context.Context) ([]string, error)
	Validate(ctx context.Context, expression string, exit bool) (strategy.Validation, error)
	Create(context.Context, strategy.Strategy) (strategy.Entry, error)
	Update(context.Context, strategy.Strategy) (strategy.Entry, error)
	SetEnabled(ctx context.Context, id int64, enabled bool) (strategy.Entry, error)
	Delete(context.Context, int64) error
	Backtest(ctx context.Context, id int64, symbol string) (strategy.Backtest, error)
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
	validation, err := api.strategies.Validate(ctx, request.Body.Expression, request.Body.Exit != nil && *request.Body.Exit)
	if err != nil {
		return ValidateStrategy500JSONResponse{api.internalError(ctx, "validate_strategy", err)}, nil
	}
	missing := make([]StrategyMissingIndicator, len(validation.Missing))
	for i, entry := range validation.Missing {
		missing[i] = StrategyMissingIndicator{Interval: CandleInterval(entry.Interval), Type: string(entry.Selection.Type), Parameters: entry.Selection.Parameters, Title: entry.Title}
	}
	return ValidateStrategy200JSONResponse{Errors: validation.Problems, MissingIndicators: missing}, nil
}

func (api *api) ListStrategies(context.Context, ListStrategiesRequestObject) (ListStrategiesResponseObject, error) {
	entries := api.strategies.List()
	items := make([]Strategy, len(entries))
	for i, entry := range entries {
		items[i] = strategyDTO(entry)
	}
	return ListStrategies200JSONResponse{Items: items}, nil
}

func (api *api) CreateStrategy(ctx context.Context, request CreateStrategyRequestObject) (CreateStrategyResponseObject, error) {
	body := request.Body
	entry, err := api.strategies.Create(ctx, strategy.Strategy{
		Name: body.Name, Expression: body.Expression, ExitExpression: body.ExitExpression, Accumulate: body.Accumulate, MaxBuys: body.MaxBuys,
		Message: body.Message, Enabled: body.Enabled,
	})
	switch {
	case err == nil:
		return CreateStrategy201JSONResponse(strategyDTO(entry)), nil
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
		ID: request.StrategyId, Name: body.Name, Expression: body.Expression, ExitExpression: body.ExitExpression,
		Accumulate: body.Accumulate, MaxBuys: body.MaxBuys, Message: body.Message,
	})
	switch {
	case err == nil:
		return UpdateStrategy200JSONResponse(strategyDTO(entry)), nil
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
		return SetStrategyEnabled200JSONResponse(strategyDTO(entry)), nil
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
	backtest, err := api.strategies.Backtest(timed, request.StrategyId, request.Params.Symbol)
	switch {
	case err == nil:
	case backtestExpired(timed, err):
		return BacktestStrategy503JSONResponse{backtestTooHeavy(ctx)}, nil
	case errors.Is(err, strategy.ErrInvalidArgument):
		return BacktestStrategy400JSONResponse{invalidArgument(ctx, err.Error()).badRequest()}, nil
	case errors.Is(err, strategy.ErrNotFound):
		return backtestNotFound(ErrorResponse(strategyNotFound(ctx))), nil
	case errors.Is(err, market.ErrInstrumentNotFound):
		return backtestNotFound(symbolNotFound(ctx).body), nil
	default:
		return BacktestStrategy500JSONResponse{api.internalError(ctx, "backtest_strategy", err)}, nil
	}
	trades := make([]BacktestTrade, len(backtest.Trades))
	for i, trade := range backtest.Trades {
		trades[i] = BacktestTrade{
			EntryTime: trade.EntryTime, EntryPrice: trade.EntryPrice, ExitTime: trade.ExitTime, ExitPrice: trade.ExitPrice,
			Buys: trade.Buys, Open: trade.Open, NetReturn: trade.Return,
		}
	}
	equity := make([]BacktestEquityPoint, len(backtest.Equity))
	for i, point := range backtest.Equity {
		equity[i] = BacktestEquityPoint{Time: point.Time, Equity: point.Equity}
	}
	dto := StrategyBacktest{
		Interval: CandleInterval(backtest.Interval), Symbol: backtest.Symbol, Fee: strategy.BacktestFee,
		Trades: trades, SkippedAlerts: backtest.Skipped, Equity: equity,
		Summary:   BacktestSummary{NetProfit: backtest.NetProfit, MaxDrawdown: backtest.MaxDrawdown, Stats: tradeStatsDTO(backtest.Stats)},
		Baselines: BacktestBaselines{BuyAndHold: backtest.BuyAndHold, Dca: backtest.DCA},
	}
	if !backtest.From.IsZero() {
		dto.From, dto.To = &backtest.From, &backtest.To
	}
	return BacktestStrategy200JSONResponse(dto), nil
}

func tradeStatsDTO(stats strategy.TradeStats) BacktestTradeStats {
	return BacktestTradeStats{
		TradeCount: stats.Count, WinRate: stats.WinRate, ProfitFactor: stats.ProfitFactor,
		AverageTrade: stats.AverageTrade, AverageWin: stats.AverageWin, AverageLoss: stats.AverageLoss,
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

func backtestNotFound(body ErrorResponse) BacktestStrategy404JSONResponse {
	return BacktestStrategy404JSONResponse{Body: body, Headers: BacktestStrategy404ResponseHeaders{XRequestID: body.RequestId}}
}

func strategyNotFound(ctx context.Context) StrategyNotFoundJSONResponse {
	return StrategyNotFoundJSONResponse(newAPIError(ctx, http.StatusNotFound, "strategy_not_found", "Strategy does not exist", nil).body)
}

func strategyConflict(ctx context.Context) StrategyConflictJSONResponse {
	return StrategyConflictJSONResponse(newAPIError(ctx, http.StatusConflict, "strategy_exists", "Another strategy has this name", nil).body)
}

func strategyDTO(entry strategy.Entry) Strategy {
	dto := Strategy{
		Id: entry.ID, Name: entry.Name, Expression: entry.Expression, ExitExpression: entry.ExitExpression, Accumulate: entry.Accumulate,
		MaxBuys: entry.MaxBuys, Message: entry.Message, Enabled: entry.Enabled, Valid: entry.Compiled != nil,
	}
	if entry.Problem != "" {
		dto.Problem = &entry.Problem
	}
	return dto
}
