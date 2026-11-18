// Package chart coordinates candle pages with the shared indicator registry.
package chart

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"time"

	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
)

var ErrInvalidRequest = errors.New("invalid chart request")

const (
	// DefaultRange is the initial chart range; larger ranges come from scroll-back.
	DefaultRange = 200
	// MaxRange bounds the number of closed candles in one chart to the
	// history synchronization keeps complete.
	MaxRange = market.SyncDepth
	// MaxIndicators bounds the indicator selection of one chart.
	MaxIndicators        = 8
	maxIndicatorLookback = 5000
)

type Store interface {
	GetActiveInstrumentBySymbol(context.Context, string) (market.Instrument, error)
	ListCandlePage(context.Context, int64, market.CandleInterval, *time.Time, int) (market.CandlePage, error)
}
type Request struct {
	Symbol     string
	Interval   market.CandleInterval
	Limit      int
	Indicators []indicator.Selection
}
type Page struct {
	Symbol  string
	Candles []market.Candle
	// Shapes draw the indicators over the candles, and Panes are the panes
	// below the candles they lie in.
	Shapes     []Shape
	Panes      []Pane
	HasMore    bool
	NextBefore *time.Time
	// warmup holds the closed candles before Candles that indicators need to
	// settle. They are calculated over but never returned to clients.
	warmup []market.Candle
	// instrument lets live sessions rebuild without resolving the symbol again.
	instrument market.Instrument
}
type Service struct {
	store      Store
	indicators *indicator.Registry
	drawers    *Drawers
	catalog    CatalogSource
	logger     *slog.Logger
}

func NewService(store Store, indicators *indicator.Registry, drawers *Drawers, catalog CatalogSource, logger *slog.Logger) (*Service, error) {
	if store == nil || indicators == nil || drawers == nil || catalog == nil || logger == nil {
		return nil, errors.New("chart store, indicator registry, drawers, catalog, and logger are required")
	}
	return &Service{store: store, indicators: indicators, drawers: drawers, catalog: catalog, logger: logger.With("module", "chart")}, nil
}

// Catalog is what the charts of an interval draw: the indicators clients
// should request, and the panes a chart that requests exactly them has.
type Catalog struct {
	Indicators []CatalogIndicator
	Panes      []Pane
}

// Catalog returns the catalog of interval charts. Drawers draw the same
// shapes whatever the values, so drawing the indicators over no candles lays
// out the panes of every frame of such a chart.
func (service *Service) Catalog(interval market.CandleInterval) (Catalog, error) {
	items := service.catalog.ChartCatalog(interval)
	indicators := make([]Calculated, len(items))
	for i, item := range items {
		indicators[i] = Calculated{ID: indicatorID(i), Calculation: indicator.Calculation{Type: item.Selection.Type, Parameters: item.Selection.Parameters}, Scale: item.Scale}
	}
	drawing, err := service.drawers.Draw(nil, indicators)
	if err != nil {
		return Catalog{}, fmt.Errorf("lay out the %s chart catalog: %w", interval, err)
	}
	return Catalog{Indicators: items, Panes: drawing.Panes}, nil
}

// indicatorID names an indicator of a chart by its position in the request,
// the same as in the catalog.
func indicatorID(position int) string { return strconv.Itoa(position) }

// Build resolves the symbol and loads the chart range with indicator warm-up.
func (service *Service) Build(ctx context.Context, request Request) (Page, error) {
	return service.build(ctx, nil, request, true)
}

// build loads the chart range. A resolved instrument skips the symbol lookup;
// live sessions skip indicators here because extend calculates them per frame.
func (service *Service) build(ctx context.Context, instrument *market.Instrument, request Request, indicators bool) (Page, error) {
	symbol := market.NormalizeSymbol(request.Symbol)
	if service == nil || symbol == "" || !request.Interval.Valid() || request.Limit <= 0 {
		return Page{}, fmt.Errorf("%w: symbol, interval, and limit are required", ErrInvalidRequest)
	}
	if request.Limit > MaxRange {
		return Page{}, fmt.Errorf("%w: chart range exceeds limit", ErrInvalidRequest)
	}
	warmup, err := service.warmup(request.Indicators)
	if err != nil {
		return Page{}, err
	}
	if instrument == nil {
		resolved, err := service.store.GetActiveInstrumentBySymbol(ctx, symbol)
		if err != nil {
			return Page{}, fmt.Errorf("resolve chart instrument: %w", err)
		}
		instrument = &resolved
	}
	started := time.Now()
	stored, err := service.store.ListCandlePage(ctx, instrument.ID, request.Interval, nil, request.Limit+warmup)
	if err != nil {
		return Page{}, fmt.Errorf("list chart candles: %w", err)
	}
	loadDuration := time.Since(started)
	split := max(0, len(stored.Candles)-request.Limit)
	page := Page{
		Symbol:     instrument.Symbol,
		Candles:    stored.Candles[split:],
		HasMore:    stored.HasMore || split > 0,
		warmup:     stored.Candles[:split],
		instrument: *instrument,
	}
	page.NextBefore = market.CandlePage{Candles: page.Candles, HasMore: page.HasMore}.NextBefore()
	if indicators {
		if page, err = service.draw(page, request.Interval, request.Indicators); err != nil {
			return Page{}, fmt.Errorf("%w: calculate: %w", ErrInvalidRequest, err)
		}
	}
	service.logger.DebugContext(ctx, "chart range loaded", "symbol", page.Symbol, "interval", request.Interval, "limit", request.Limit, "warmup", warmup,
		"candles", len(stored.Candles), "load_duration", loadDuration)
	return page, nil
}

// Validate checks an indicator selection before any history is loaded.
func (service *Service) Validate(configs []indicator.Selection) error {
	if len(configs) > MaxIndicators {
		return fmt.Errorf("%w: indicator count must not exceed %d", ErrInvalidRequest, MaxIndicators)
	}
	for _, config := range configs {
		value, err := service.indicators.Lookback(config.Type, config.Parameters)
		if err != nil {
			return fmt.Errorf("%w: lookback for %q: %w", ErrInvalidRequest, config.Type, err)
		}
		if value > maxIndicatorLookback {
			return fmt.Errorf("%w: lookback for %q exceeds %d", ErrInvalidRequest, config.Type, maxIndicatorLookback)
		}
	}
	return nil
}

// warmup validates the selection and returns the number of closed candles
// loaded before the visible range: the largest Window but one.
func (service *Service) warmup(configs []indicator.Selection) (int, error) {
	if err := service.Validate(configs); err != nil {
		return 0, err
	}
	largest := 0
	for _, config := range configs {
		window, err := service.indicators.Window(config)
		if err != nil {
			return 0, err
		}
		largest = max(largest, window-1)
	}
	return largest, nil
}

// calculate runs each indicator over its own Window of warm-up before the
// visible candles, so its points do not depend on the other indicators, and
// keeps only the points of visible candles.
func (service *Service) calculate(interval market.CandleInterval, warmup, candles []market.Candle, configs []indicator.Selection) ([]indicator.Calculation, error) {
	results := make([]indicator.Calculation, len(configs))
	for index, config := range configs {
		window, err := service.indicators.Window(config)
		if err != nil {
			return nil, err
		}
		own := warmup[max(0, len(warmup)-(window-1)):]
		all := append(append([]market.Candle(nil), own...), candles...)
		calculated, err := service.indicators.CalculateCandles(interval, all, []indicator.Selection{config})
		if err != nil {
			return nil, err
		}
		results[index] = calculated[0]
		if len(own) == 0 {
			continue
		}
		for j := range results[index].Series {
			points := results[index].Series[j].Points
			first := len(points)
			if len(candles) > 0 {
				first, _ = slices.BinarySearchFunc(points, candles[0].OpenTime, func(point indicator.Point, target time.Time) int {
					return point.Time.Compare(target)
				})
			}
			results[index].Series[j].Points = points[first:]
		}
	}
	return results, nil
}

// extend calculates a private live context. It never mutates closed history.
func (service *Service) extend(page Page, interval market.CandleInterval, live *market.Candle, configs []indicator.Selection) (Page, error) {
	candles := append([]market.Candle(nil), page.Candles...)
	if live != nil {
		if len(candles) > 0 && candles[len(candles)-1].OpenTime.Equal(live.OpenTime) {
			candles[len(candles)-1] = *live
		} else if len(candles) == 0 || interval.NextOpenTime(candles[len(candles)-1].OpenTime).Equal(live.OpenTime) {
			candles = append(candles, *live)
		}
	}
	page.Candles = candles
	return service.draw(page, interval, configs)
}

// draw calculates the indicators over the candles of page and draws them. A
// pane takes the scale of the catalog indicator with its selection; the
// indicators are named by their position.
func (service *Service) draw(page Page, interval market.CandleInterval, configs []indicator.Selection) (Page, error) {
	calculations, err := service.calculate(interval, page.warmup, page.Candles, configs)
	if err != nil {
		return Page{}, err
	}
	catalog := service.catalog.ChartCatalog(interval)
	indicators := make([]Calculated, len(calculations))
	for i, calculation := range calculations {
		indicators[i] = Calculated{ID: indicatorID(i), Calculation: calculation}
		if selection, err := service.indicators.Normalize(configs[i]); err == nil {
			if index := slices.IndexFunc(catalog, func(item CatalogIndicator) bool { return sameSelection(item.Selection, selection) }); index >= 0 {
				indicators[i].Scale = catalog[index].Scale
			}
		}
	}
	drawing, err := service.drawers.Draw(page.Candles, indicators)
	if err != nil {
		return Page{}, err
	}
	page.Shapes, page.Panes = drawing.Shapes, drawing.Panes
	return page, nil
}
