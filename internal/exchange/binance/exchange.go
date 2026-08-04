package binance

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"crypto-scanner/internal/market"

	connector "github.com/binance/binance-connector-go"
)

const spotPermission = "SPOT"

// Exchange adapts the official Binance connector to market domain values.
// Connector request and response types do not leave this package.
type Exchange struct {
	client *connector.Client
}

// New creates a public Binance Spot exchange adapter for the official endpoint.
func New() *Exchange {
	return &Exchange{client: connector.NewClient("", "")}
}

// NewWithHTTPClient creates an adapter with an explicit HTTP boundary. It is
// useful for deterministic fixtures without exposing official connector types.
func NewWithHTTPClient(baseURL string, httpClient *http.Client) *Exchange {
	client := connector.NewClient("", "", baseURL)
	client.HTTPClient = httpClient
	return &Exchange{client: client}
}

// ListInstruments returns a complete Binance Spot/USDT discovery snapshot.
func (exchange *Exchange) ListInstruments(ctx context.Context) ([]market.Instrument, error) {
	response, err := exchange.client.NewExchangeInfoService().Permissions(spotPermission).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("list Binance Spot exchange information: %w", err)
	}

	items := make([]market.Instrument, 0, len(response.Symbols))
	seen := make(map[string]struct{}, len(response.Symbols))
	for index, symbol := range response.Symbols {
		if symbol == nil {
			return nil, fmt.Errorf("Binance Spot exchange information contains an empty symbol at index %d", index)
		}
		item := market.Instrument{
			Symbol:     strings.ToUpper(strings.TrimSpace(symbol.Symbol)),
			BaseAsset:  strings.ToUpper(strings.TrimSpace(symbol.BaseAsset)),
			QuoteAsset: strings.ToUpper(strings.TrimSpace(symbol.QuoteAsset)),
			Status:     strings.ToUpper(strings.TrimSpace(symbol.Status)),
		}
		if item.Symbol == "" || item.BaseAsset == "" || item.QuoteAsset == "" || item.Status == "" {
			return nil, fmt.Errorf("Binance Spot exchange information contains an incomplete symbol at index %d", index)
		}
		active, knownStatus := spotStatusActive(item.Status)
		if !knownStatus {
			return nil, fmt.Errorf("Binance Spot exchange information contains symbol %q with unknown status %q", item.Symbol, item.Status)
		}
		if item.QuoteAsset != "USDT" {
			continue
		}
		spot, err := supportsSpot(symbol)
		if err != nil {
			return nil, fmt.Errorf("Binance Spot exchange information contains an incomplete symbol %q: %w", item.Symbol, err)
		}
		if !spot {
			continue
		}
		if _, exists := seen[item.Symbol]; exists {
			return nil, fmt.Errorf("Binance Spot exchange information contains duplicate symbol %q", item.Symbol)
		}
		seen[item.Symbol] = struct{}{}
		item.Active = active
		items = append(items, item)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("Binance Spot exchange information contains no USDT instruments")
	}
	return items, nil
}

func spotStatusActive(status string) (active, known bool) {
	switch status {
	case "TRADING":
		return true, true
	case "PRE_TRADING", "POST_TRADING", "END_OF_DAY", "HALT", "AUCTION_MATCH", "BREAK":
		return false, true
	default:
		return false, false
	}
}

func supportsSpot(symbol *connector.SymbolInfo) (bool, error) {
	if symbol.IsSpotTradingAllowed {
		return true, nil
	}
	hasPermissionMetadata := false
	for _, permission := range symbol.Permissions {
		if strings.TrimSpace(permission) != "" {
			hasPermissionMetadata = true
		}
		if strings.EqualFold(permission, spotPermission) {
			return true, nil
		}
	}
	for _, set := range symbol.PermissionSets {
		for _, permission := range set {
			if strings.TrimSpace(permission) != "" {
				hasPermissionMetadata = true
			}
			if strings.EqualFold(permission, spotPermission) {
				return true, nil
			}
		}
	}
	if !hasPermissionMetadata {
		return false, fmt.Errorf("missing Spot permission metadata")
	}
	return false, nil
}
