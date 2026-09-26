package httpapi

import (
	"context"
	"math"
	"net/http"
	"strings"
	"time"

	"crypto-scanner/internal/analysis"
	"crypto-scanner/internal/closedindicator"
)

func marketSearchRequest(request MarketAnalysisRequest) analysis.SearchRequest {
	result := analysis.SearchRequest{Criteria: criterionConfigs(request.Criteria)}
	if request.Limit != nil {
		result.Limit = *request.Limit
	}
	if request.Sort != nil {
		result.Sort = &analysis.SearchSort{Field: string(request.Sort.Field), Direction: string(request.Sort.Direction)}
	}
	return result
}

func criterionConfigs(criteria []CriterionRequest) []analysis.CriterionConfig {
	configs := make([]analysis.CriterionConfig, len(criteria))
	for i, criterion := range criteria {
		configs[i] = analysis.CriterionConfig{Key: criterion.Key, Name: criterion.Name, Label: criterion.Label, Parameters: criterion.Parameters}
	}
	return configs
}

func (api *api) AnalyzeInstrument(ctx context.Context, request AnalyzeInstrumentRequestObject) (AnalyzeInstrumentResponseObject, error) {
	body := InstrumentAnalysisRequest{}
	if request.Body != nil {
		body = *request.Body
	}
	result, err := api.analysis.AnalyzeSymbol(ctx, analysis.SymbolRequest{Symbol: request.Symbol, Criteria: criterionConfigs(body.Criteria)})
	if err != nil {
		return api.analyzeInstrumentError(ctx, err, request.Symbol), nil
	}
	return AnalyzeInstrument200JSONResponse{
		Body: InstrumentAnalysisResponse{
			Symbol: result.Symbol, Matched: result.Matched,
			Evaluations: responseEvaluations(result.Evaluations), Warnings: responseWarnings(result.Warnings),
		},
		Headers: AnalyzeInstrument200ResponseHeaders{XRequestID: RequestIdentifier(ctx)},
	}, nil
}

func (api *api) AnalyzeMarket(ctx context.Context, request AnalyzeMarketRequestObject) (AnalyzeMarketResponseObject, error) {
	body := MarketAnalysisRequest{}
	if request.Body != nil {
		body = *request.Body
	}
	result, err := api.analysis.Search(ctx, marketSearchRequest(body))
	if err != nil {
		return api.analyzeMarketError(ctx, err), nil
	}
	return AnalyzeMarket200JSONResponse{
		Body:    marketAnalysisResponse(result),
		Headers: AnalyzeMarket200ResponseHeaders{XRequestID: RequestIdentifier(ctx)},
	}, nil
}

func marketAnalysisResponse(result analysis.SearchResult) MarketAnalysisResponse {
	items := make([]MarketAnalysisItem, len(result.Items))
	for i, item := range result.Items {
		items[i] = MarketAnalysisItem{Symbol: item.Symbol, Matched: item.Matched, Evaluations: responseMarketScanEvaluations(item.Evaluations), PriceHistory: item.PriceHistory, ClosedIndicators: closedIndicatorsResponse(item.ClosedIndicators)}
	}
	unresolved := make([]UnresolvedInstrument, len(result.Unresolved))
	for i, item := range result.Unresolved {
		unresolved[i] = UnresolvedInstrument{Symbol: item.Symbol, Code: UnresolvedInstrumentCode(item.Code), Message: item.Message}
	}
	return MarketAnalysisResponse{
		PriceHistoryWindow: PriceHistoryWindow{From: result.PriceHistoryWindow.From, To: result.PriceHistoryWindow.To},
		MatchedCount:       result.MatchedCount, AnalyzedCount: result.AnalyzedCount, InsufficientDataCount: result.InsufficientDataCount,
		Items: items, Unresolved: unresolved, Warnings: responseWarnings(result.Warnings),
	}
}

func closedIndicatorsResponse(values []closedindicator.Value) []ClosedIndicator {
	result := make([]ClosedIndicator, len(values))
	for i, value := range values {
		outputs := make([]ClosedIndicatorOutput, len(value.Outputs))
		for j, output := range value.Outputs {
			outputs[j] = ClosedIndicatorOutput{Name: output.Name, Value: output.Value}
		}
		var openTime *time.Time
		if !value.OpenTime.IsZero() {
			opened := value.OpenTime.UTC()
			openTime = &opened
		}
		parameters := map[string]interface{}{}
		for key, parameter := range value.Target.Selection.Parameters {
			parameters[key] = parameter
		}
		result[i] = ClosedIndicator{
			Type: string(value.Target.Selection.Type), Interval: CandleInterval(value.Target.Interval),
			Parameters: parameters, OpenTime: openTime, Outputs: outputs,
		}
	}
	return result
}

func (api *api) analyzeInstrumentError(ctx context.Context, err error, symbol string) AnalyzeInstrumentResponseObject {
	mapped, ok := analysisError(ctx, err, symbol)
	if !ok {
		return AnalyzeInstrument500JSONResponse{api.internalError(ctx, "analyze_instrument", err)}
	}
	switch mapped.status {
	case http.StatusBadRequest:
		return AnalyzeInstrument400JSONResponse{mapped.badRequest()}
	case http.StatusNotFound:
		return AnalyzeInstrument404JSONResponse{mapped.symbolNotFound()}
	case http.StatusConflict:
		return AnalyzeInstrument409JSONResponse{mapped.insufficientData()}
	case http.StatusUnprocessableEntity:
		return AnalyzeInstrument422JSONResponse{mapped.unprocessable()}
	default:
		return AnalyzeInstrument503JSONResponse{mapped.unavailable()}
	}
}

func (api *api) analyzeMarketError(ctx context.Context, err error) AnalyzeMarketResponseObject {
	mapped, ok := analysisError(ctx, err, "")
	switch {
	case !ok:
		return AnalyzeMarket500JSONResponse{api.internalError(ctx, "analyze_market", err)}
	case mapped.status == http.StatusBadRequest:
		return AnalyzeMarket400JSONResponse{mapped.badRequest()}
	case mapped.status == http.StatusUnprocessableEntity:
		return AnalyzeMarket422JSONResponse{mapped.unprocessable()}
	case mapped.status == http.StatusServiceUnavailable:
		return AnalyzeMarket503JSONResponse{mapped.unavailable()}
	default:
		return AnalyzeMarket500JSONResponse{api.internalError(ctx, "analyze_market", err)}
	}
}

func roundPercentage(value float64) float64 { return math.Round(value*10_000) / 10_000 }

func responseEvaluations(evaluations []analysis.Evaluation) []Evaluation {
	items := make([]Evaluation, len(evaluations))
	for i, evaluation := range evaluations {
		metrics := make(map[string]float64, len(evaluation.Metrics))
		for name, value := range evaluation.Metrics {
			metrics[name] = value
		}
		items[i] = Evaluation{Key: evaluation.Key, Name: evaluation.Name, Label: evaluation.Label, Matched: evaluation.Matched, Metrics: metrics, CandleCount: evaluation.CandleCount, From: evaluation.From.UTC(), To: evaluation.To.UTC()}
	}
	return items
}

// Market Scan retains its presentation rounding. Instrument Analysis carries
// original metrics so derived recommendations can be calculated before rounding.
func responseMarketScanEvaluations(evaluations []analysis.Evaluation) []Evaluation {
	items := responseEvaluations(evaluations)
	for index := range items {
		for name, value := range items[index].Metrics {
			if strings.HasSuffix(name, "_percent") {
				items[index].Metrics[name] = roundPercentage(value)
			}
		}
	}
	return items
}

func responseWarnings(warnings []analysis.Warning) []Warning {
	result := make([]Warning, len(warnings))
	for i, warning := range warnings {
		result[i] = Warning{Code: warning.Code, Message: warning.Message}
	}
	return result
}
