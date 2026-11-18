package httpapi

import (
	"context"
	"fmt"

	"crypto-scanner/internal/chart"
	"crypto-scanner/internal/market"
)

func (api *api) ListChartIndicators(ctx context.Context, request ListChartIndicatorsRequestObject) (ListChartIndicatorsResponseObject, error) {
	catalog, err := api.Chart.Catalog(market.CandleInterval(request.Params.Interval))
	if err != nil {
		return ListChartIndicators500JSONResponse{api.internalError(ctx, "list_chart_indicators", err)}, nil
	}
	items := make([]ChartIndicatorDefinition, len(catalog.Indicators))
	for i, item := range catalog.Indicators {
		items[i] = ChartIndicatorDefinition{Id: item.ID, Type: string(item.Selection.Type), Parameters: item.Selection.Parameters}
	}
	return ListChartIndicators200JSONResponse{Items: items, Panes: chartPanesDTO(catalog.Panes)}, nil
}

// chartWirePage serializes a chart page.
func chartWirePage(page chart.Page, interval market.CandleInterval) (*ChartPageResponse, error) {
	candles := make([]Candle, len(page.Candles))
	for i, candle := range page.Candles {
		candles[i] = candleResponse(candle)
	}
	shapes := make([]ChartShape, len(page.Shapes))
	for i, shape := range page.Shapes {
		var err error
		if shapes[i], err = chartShapeDTO(shape); err != nil {
			return nil, err
		}
	}
	return &ChartPageResponse{Symbol: page.Symbol, Interval: CandleInterval(interval), Candles: candles, Shapes: shapes, Panes: chartPanesDTO(page.Panes), HasMore: page.HasMore, NextBefore: page.NextBefore}, nil
}

func chartPanesDTO(panes []chart.Pane) []ChartPane {
	result := make([]ChartPane, len(panes))
	for i, pane := range panes {
		levels := make([]ChartIndicatorLevel, len(pane.Scale.Levels))
		for j, level := range pane.Scale.Levels {
			levels[j] = ChartIndicatorLevel{Value: level.Value, Title: level.Title}
		}
		result[i] = ChartPane{Key: pane.Key, Scale: ChartIndicatorScale{Min: pane.Scale.Min, Max: pane.Scale.Max, Levels: levels}}
	}
	return result
}

func chartShapeDTO(shape chart.Shape) (ChartShape, error) {
	var result ChartShape
	placement := ChartPlacement(shape.Place.Placement)
	var pane *string
	if shape.Place.Pane != "" {
		pane = &shape.Place.Pane
	}
	switch {
	case shape.Line != nil:
		points := make([]ChartLinePoint, len(shape.Line.Points))
		for i, point := range shape.Line.Points {
			points[i] = ChartLinePoint{Time: point.Time, Value: point.Value}
			if point.Color != "" {
				points[i].Color = &point.Color
			}
			if point.Break {
				points[i].Break = &point.Break
			}
		}
		return result, result.FromChartLineShape(ChartLineShape{Id: shape.ID, Placement: placement, Pane: pane, Title: shape.Line.Title, Color: shape.Line.Color, Points: points})
	case shape.Area != nil:
		points := make([]ChartAreaPoint, len(shape.Area.Points))
		for i, point := range shape.Area.Points {
			points[i] = ChartAreaPoint{Time: point.Time, Value: point.Value, Base: point.Base, Color: point.Color}
		}
		return result, result.FromChartAreaShape(ChartAreaShape{Id: shape.ID, Placement: placement, Pane: pane, Points: points})
	case shape.Markers != nil:
		points := make([]ChartMarker, len(shape.Markers.Points))
		for i, marker := range shape.Markers.Points {
			points[i] = ChartMarker{Time: marker.Time, CalculatedTime: marker.Calculated, Value: marker.Value, LineValue: marker.LineValue}
		}
		return result, result.FromChartMarkersShape(ChartMarkersShape{Id: shape.ID, Placement: placement, Pane: pane, Title: shape.Markers.Title, Color: shape.Markers.Color,
			Position: ChartMarkersShapePosition(shape.Markers.Position), Points: points})
	case shape.Segments != nil:
		segments := make([]ChartSegment, len(shape.Segments.Items))
		for i, segment := range shape.Segments.Items {
			segments[i] = ChartSegment{CalculatedTime: segment.Calculated,
				From: ChartSegmentPoint{Time: segment.From.Time, Value: segment.From.Value}, To: ChartSegmentPoint{Time: segment.To.Time, Value: segment.To.Value}}
		}
		return result, result.FromChartSegmentsShape(ChartSegmentsShape{Id: shape.ID, Placement: placement, Pane: pane, Color: shape.Segments.Color, Segments: segments})
	}
	return result, fmt.Errorf("chart shape %q has no figure", shape.ID)
}
