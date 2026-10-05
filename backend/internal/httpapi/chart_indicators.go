package httpapi

import (
	"context"

	"crypto-scanner/internal/chart"
	"crypto-scanner/internal/market"
)

func (api *api) ListChartIndicators(_ context.Context, request ListChartIndicatorsRequestObject) (ListChartIndicatorsResponseObject, error) {
	catalog := api.chart.Catalog(market.CandleInterval(request.Params.Interval))
	items := make([]ChartIndicatorDefinition, len(catalog))
	for i, item := range catalog {
		items[i] = chartIndicatorDTO(item)
	}
	return ListChartIndicators200JSONResponse{Items: items}, nil
}

func chartIndicatorDTO(item chart.CatalogIndicator) ChartIndicatorDefinition {
	lines := make([]ChartIndicatorLine, len(item.Lines))
	for i, line := range item.Lines {
		lines[i] = ChartIndicatorLine{Output: line.Output, Title: line.Title, Color: line.Color}
	}
	definition := ChartIndicatorDefinition{
		Id:         item.ID,
		Type:       string(item.Selection.Type),
		Parameters: item.Selection.Parameters,
		Placement:  ChartIndicatorDefinitionPlacement(item.Placement),
		Lines:      lines,
	}
	if item.Scale != nil {
		levels := make([]ChartIndicatorLevel, len(item.Scale.Levels))
		for i, level := range item.Scale.Levels {
			levels[i] = ChartIndicatorLevel{Value: level.Value, Title: level.Title}
		}
		definition.Scale = &ChartIndicatorScale{Min: item.Scale.Min, Max: item.Scale.Max, Levels: levels}
	}
	if item.Pane != "" {
		definition.Pane = &item.Pane
	}
	return definition
}
