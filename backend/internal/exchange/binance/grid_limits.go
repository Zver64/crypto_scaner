package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"crypto-scanner/internal/market"
	"crypto-scanner/internal/platform/numeric"
)

// invalidSymbolCode is the Binance error code for an unknown symbol.
const invalidSymbolCode = -1121

// AveragePrice returns the symbol's average price over the last few minutes,
// which Binance checks order prices against.
func (exchange *Exchange) AveragePrice(ctx context.Context, symbol string) (float64, error) {
	symbol = market.NormalizeSymbol(symbol)
	var response struct {
		Price string `json:"price"`
	}
	if err := exchange.getSymbolJSON(ctx, "/api/v3/avgPrice", symbol, &response); err != nil {
		return 0, fmt.Errorf("get Binance average price for %s: %w", symbol, err)
	}
	price, err := numeric.ParseFinite(response.Price)
	if err != nil || price <= 0 {
		return 0, fmt.Errorf("get Binance average price for %s: invalid price %q", symbol, response.Price)
	}
	return price, nil
}

// PriceFilters returns the order price filters of a symbol that bound how far
// from the average price its orders may go.
func (exchange *Exchange) PriceFilters(ctx context.Context, symbol string) (market.PriceFilters, error) {
	symbol = market.NormalizeSymbol(symbol)
	var response struct {
		Symbols []struct {
			Symbol  string `json:"symbol"`
			Filters []struct {
				FilterType        string `json:"filterType"`
				MinPrice          string `json:"minPrice"`
				MaxPrice          string `json:"maxPrice"`
				TickSize          string `json:"tickSize"`
				BidMultiplierDown string `json:"bidMultiplierDown"`
				AskMultiplierUp   string `json:"askMultiplierUp"`
				MultiplierDown    string `json:"multiplierDown"`
				MultiplierUp      string `json:"multiplierUp"`
			} `json:"filters"`
		} `json:"symbols"`
	}
	if err := exchange.getSymbolJSON(ctx, "/api/v3/exchangeInfo", symbol, &response); err != nil {
		return market.PriceFilters{}, fmt.Errorf("get Binance price filters for %s: %w", symbol, err)
	}
	for _, info := range response.Symbols {
		if market.NormalizeSymbol(info.Symbol) != symbol {
			continue
		}
		var filters market.PriceFilters
		for _, filter := range info.Filters {
			type field struct {
				value  string
				target *float64
			}
			var fields []field
			switch filter.FilterType {
			case "PRICE_FILTER":
				fields = []field{{filter.MinPrice, &filters.MinPrice}, {filter.MaxPrice, &filters.MaxPrice}, {filter.TickSize, &filters.TickSize}}
			case "PERCENT_PRICE_BY_SIDE":
				fields = []field{{filter.BidMultiplierDown, &filters.BidMultiplierDown}, {filter.AskMultiplierUp, &filters.AskMultiplierUp}}
			case "PERCENT_PRICE":
				fields = []field{{filter.MultiplierDown, &filters.MultiplierDown}, {filter.MultiplierUp, &filters.MultiplierUp}}
			}
			for _, field := range fields {
				parsed, err := numeric.ParseFinite(field.value)
				if err != nil || parsed < 0 {
					return market.PriceFilters{}, fmt.Errorf("get Binance price filters for %s: invalid %s value %q", symbol, filter.FilterType, field.value)
				}
				*field.target = parsed
			}
		}
		return filters, nil
	}
	return market.PriceFilters{}, market.ErrInstrumentNotFound
}

// getSymbolJSON decodes a public endpoint that takes a symbol through the
// shared limiter and retry policy. An unknown symbol is market.ErrInstrumentNotFound.
func (exchange *Exchange) getSymbolJSON(ctx context.Context, path, symbol string, target any) error {
	if symbol == "" {
		return fmt.Errorf("symbol is required")
	}
	endpoint := exchange.client.BaseURL + path + "?" + url.Values{"symbol": {symbol}}.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	response, err := exchange.client.HTTPClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		var apiError struct {
			Code    int    `json:"code"`
			Message string `json:"msg"`
		}
		_ = json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&apiError)
		if apiError.Code == invalidSymbolCode {
			return market.ErrInstrumentNotFound
		}
		return fmt.Errorf("unexpected status %d: %s", response.StatusCode, apiError.Message)
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
