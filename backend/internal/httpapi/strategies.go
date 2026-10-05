package httpapi

import (
	"context"
	"errors"
	"net/http"

	"crypto-scanner/internal/market"
	"crypto-scanner/internal/strategy"
)

// Strategies manages the administrator's strategies.
type Strategies interface {
	List() []strategy.Entry
	Variables() []strategy.Variable
	Symbols(context.Context) ([]string, error)
	Validate(ctx context.Context, expression string) (strategy.Validation, error)
	Create(context.Context, strategy.Strategy) (strategy.Entry, error)
	Update(ctx context.Context, id int64, name, expression, message string) (strategy.Entry, error)
	SetEnabled(ctx context.Context, id int64, enabled bool) (strategy.Entry, error)
	Delete(context.Context, int64) error
	Backtest(ctx context.Context, id int64, symbol string) (strategy.Backtest, error)
}

func (api *api) ListStrategyVariables(context.Context, ListStrategyVariablesRequestObject) (ListStrategyVariablesResponseObject, error) {
	variables := api.strategies.Variables()
	items := make([]StrategyVariable, len(variables))
	for i, variable := range variables {
		items[i] = StrategyVariable{Name: variable.Name, Label: variable.Label, Interval: CandleInterval(variable.Target.Interval)}
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
	validation, err := api.strategies.Validate(ctx, request.Body.Expression)
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
	entry, err := api.strategies.Create(ctx, strategy.Strategy{Name: request.Body.Name, Expression: request.Body.Expression, Message: request.Body.Message, Enabled: request.Body.Enabled})
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
	entry, err := api.strategies.Update(ctx, request.StrategyId, request.Body.Name, request.Body.Expression, request.Body.Message)
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
	symbol := market.NormalizeSymbol(request.Params.Symbol)
	if symbol == "" {
		return BacktestStrategy400JSONResponse{invalidArgument(ctx, "Symbol is required").badRequest()}, nil
	}
	backtest, err := api.strategies.Backtest(ctx, request.StrategyId, symbol)
	switch {
	case err == nil:
	case errors.Is(err, strategy.ErrInvalidArgument):
		return BacktestStrategy400JSONResponse{invalidArgument(ctx, err.Error()).badRequest()}, nil
	case errors.Is(err, strategy.ErrNotFound):
		return backtestNotFound(ErrorResponse(strategyNotFound(ctx))), nil
	case errors.Is(err, market.ErrInstrumentNotFound):
		return backtestNotFound(symbolNotFound(ctx).body), nil
	default:
		return BacktestStrategy500JSONResponse{api.internalError(ctx, "backtest_strategy", err)}, nil
	}
	alerts := make([]StrategyBacktestAlert, len(backtest.Alerts))
	for i, openTime := range backtest.Alerts {
		alerts[i] = StrategyBacktestAlert{OpenTime: openTime}
	}
	dto := StrategyBacktest{Interval: CandleInterval(backtest.Interval), Alerts: alerts}
	if !backtest.From.IsZero() {
		dto.From, dto.To = &backtest.From, &backtest.To
	}
	return BacktestStrategy200JSONResponse(dto), nil
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
	dto := Strategy{Id: entry.ID, Name: entry.Name, Expression: entry.Expression, Message: entry.Message, Enabled: entry.Enabled, Valid: entry.Compiled != nil}
	if entry.Problem != "" {
		dto.Problem = &entry.Problem
	}
	return dto
}
