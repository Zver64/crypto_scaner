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

// ReferencePrice returns the price Binance checks order price ranges against.
func (exchange *Exchange) ReferencePrice(ctx context.Context, symbol string) (float64, error) {
	symbol = market.NormalizeSymbol(symbol)
	var response struct {
		ReferencePrice string `json:"referencePrice"`
	}
	if err := exchange.getSymbolJSON(ctx, "/api/v3/referencePrice", symbol, &response); err != nil {
		return 0, fmt.Errorf("get Binance reference price for %s: %w", symbol, err)
	}
	price, err := numeric.ParseFinite(response.ReferencePrice)
	if err != nil || price <= 0 {
		return 0, fmt.Errorf("get Binance reference price for %s: invalid price %q", symbol, response.ReferencePrice)
	}
	return price, nil
}

// PriceRange returns the PRICE_RANGE execution rule of a symbol, or nil when
// Binance defines none.
func (exchange *Exchange) PriceRange(ctx context.Context, symbol string) (*market.PriceRange, error) {
	symbol = market.NormalizeSymbol(symbol)
	var response struct {
		SymbolRules []struct {
			Symbol string `json:"symbol"`
			Rules  []struct {
				RuleType         string `json:"ruleType"`
				BidLimitMultDown string `json:"bidLimitMultDown"`
				AskLimitMultUp   string `json:"askLimitMultUp"`
			} `json:"rules"`
		} `json:"symbolRules"`
	}
	if err := exchange.getSymbolJSON(ctx, "/api/v3/executionRules", symbol, &response); err != nil {
		return nil, fmt.Errorf("get Binance execution rules for %s: %w", symbol, err)
	}
	for _, symbolRules := range response.SymbolRules {
		if market.NormalizeSymbol(symbolRules.Symbol) != symbol {
			continue
		}
		for _, rule := range symbolRules.Rules {
			if rule.RuleType != "PRICE_RANGE" {
				continue
			}
			down, downErr := numeric.ParseFinite(rule.BidLimitMultDown)
			up, upErr := numeric.ParseFinite(rule.AskLimitMultUp)
			if downErr != nil || upErr != nil || down <= 0 || down > 1 || up < 1 {
				return nil, fmt.Errorf("get Binance execution rules for %s: invalid price range %q–%q", symbol, rule.BidLimitMultDown, rule.AskLimitMultUp)
			}
			return &market.PriceRange{BidLimitMultDown: down, AskLimitMultUp: up}, nil
		}
	}
	return nil, nil
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
