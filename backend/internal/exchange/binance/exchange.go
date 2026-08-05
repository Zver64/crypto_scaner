package binance

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"crypto-scanner/internal/market"

	connector "github.com/binance/binance-connector-go"
	"golang.org/x/time/rate"
)

const spotPermission = "SPOT"

const dailyInterval = "1d"

// Exchange adapts the official Binance connector to market domain values.
// Connector request and response types do not leave this package.
type Exchange struct {
	client    *connector.Client
	limiter   *rate.Limiter
	transport *retryTransport
}

// Options configures the shared public Binance HTTP policy.
type Options struct {
	BaseURL        string
	HTTPClient     *http.Client
	RetryAttempts  int
	RetryBaseDelay time.Duration
	Limiter        *rate.Limiter
}

// New creates a public Binance Spot exchange adapter for the official endpoint.
func New() *Exchange {
	return NewWithOptions(Options{})
}

// NewWithHTTPClient creates an adapter with an explicit HTTP boundary. It is
// useful for deterministic fixtures without exposing official connector types.
func NewWithHTTPClient(baseURL string, httpClient *http.Client) *Exchange {
	return NewWithOptions(Options{BaseURL: baseURL, HTTPClient: httpClient})
}

// NewWithOptions creates an adapter whose discovery and candle calls share one
// limiter and retry policy.
func NewWithOptions(options Options) *Exchange {
	if options.BaseURL == "" {
		options.BaseURL = "https://api.binance.com"
	}
	if options.HTTPClient == nil {
		options.HTTPClient = http.DefaultClient
	}
	if options.RetryAttempts < 1 {
		options.RetryAttempts = 5
	}
	if options.RetryBaseDelay <= 0 {
		options.RetryBaseDelay = 200 * time.Millisecond
	}
	if options.Limiter == nil {
		options.Limiter = rate.NewLimiter(rate.Limit(10), 4)
	}
	client := connector.NewClient("", "", options.BaseURL)
	httpClient := *options.HTTPClient
	baseTransport := options.HTTPClient.Transport
	if baseTransport == nil {
		baseTransport = http.DefaultTransport
	}
	transport := &retryTransport{
		base: baseTransport, limiter: options.Limiter, attempts: options.RetryAttempts, baseDelay: options.RetryBaseDelay,
	}
	httpClient.Transport = transport
	client.HTTPClient = &httpClient
	return &Exchange{client: client, limiter: options.Limiter, transport: transport}
}

// RetryCount returns the cumulative number of retry attempts made by this adapter.
func (exchange *Exchange) RetryCount() uint64 { return exchange.transport.retryCount.Load() }

// ListInstruments returns a complete Binance Spot/USDT discovery snapshot.
func (exchange *Exchange) ListInstruments(ctx context.Context) ([]market.Instrument, error) {
	response, err := exchange.client.NewExchangeInfoService().Permissions(spotPermission).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("list Binance Spot exchange information: %w", err)
	}
	exchange.applyRateLimits(response.RateLimits)

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

func (exchange *Exchange) applyRateLimits(limits []*connector.RateLimit) {
	for _, limit := range limits {
		if limit == nil || limit.RateLimitType != "REQUEST_WEIGHT" || limit.Interval != "MINUTE" || limit.Limit <= 0 {
			continue
		}
		// Keep ten percent headroom for other users of the public IP.
		exchange.limiter.SetLimit(rate.Limit(float64(limit.Limit) * 0.9 / 60))
		return
	}
}

// ListClosedCandles returns Binance klines that are complete at the request's
// exclusive cutoff. Connector response types remain confined to this adapter.
func (exchange *Exchange) ListClosedCandles(ctx context.Context, request market.CandleRequest) ([]market.Candle, error) {
	symbol := strings.ToUpper(strings.TrimSpace(request.Symbol))
	if symbol == "" {
		return nil, fmt.Errorf("list Binance candles: symbol is required")
	}
	if request.Interval != dailyInterval {
		return nil, fmt.Errorf("list Binance candles for %s: unsupported interval %q", symbol, request.Interval)
	}
	if request.Limit <= 0 || request.Limit > 1000 {
		return nil, fmt.Errorf("list Binance candles for %s: limit must be between 1 and 1000", symbol)
	}
	if request.ClosedBefore.IsZero() || request.ClosedBefore.UnixMilli() <= 0 {
		return nil, fmt.Errorf("list Binance candles for %s: closed-before cutoff is required", symbol)
	}
	cutoffUTC := request.ClosedBefore.UTC()
	currentDayStartedAt := time.Date(cutoffUTC.Year(), cutoffUTC.Month(), cutoffUTC.Day(), 0, 0, 0, 0, time.UTC)
	if currentDayStartedAt.UnixMilli() <= 0 {
		return nil, fmt.Errorf("list Binance candles for %s: closed-before cutoff must follow the Unix epoch day", symbol)
	}

	service := exchange.client.NewKlinesService().
		Symbol(symbol).
		Interval(request.Interval).
		Limit(request.Limit).
		EndTime(uint64(currentDayStartedAt.UnixMilli() - 1))
	if request.AfterOpenTime != nil {
		start := request.AfterOpenTime.UTC().UnixMilli() + 1
		if start <= 0 || start >= currentDayStartedAt.UnixMilli() {
			return nil, fmt.Errorf("list Binance candles for %s: after-open-time must precede the current UTC day", symbol)
		}
		service.StartTime(uint64(start))
	}
	response, err := service.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("list Binance candles for %s: %w", symbol, err)
	}

	items := make([]market.Candle, 0, len(response))
	for index, kline := range response {
		if kline == nil {
			return nil, fmt.Errorf("list Binance candles for %s: empty kline at index %d", symbol, index)
		}
		closeTime := time.UnixMilli(int64(kline.CloseTime)).UTC()
		if !closeTime.Before(request.ClosedBefore) {
			continue
		}
		values := []string{kline.Open, kline.High, kline.Low, kline.Close, kline.Volume, kline.QuoteAssetVolume}
		converted := make([]float64, len(values))
		for valueIndex, value := range values {
			converted[valueIndex], err = strconv.ParseFloat(value, 64)
			if err != nil || math.IsNaN(converted[valueIndex]) || math.IsInf(converted[valueIndex], 0) {
				return nil, fmt.Errorf("list Binance candles for %s: invalid numeric value %q at kline %d", symbol, value, index)
			}
		}
		if kline.NumberOfTrades > math.MaxInt64 {
			return nil, fmt.Errorf("list Binance candles for %s: trade count at kline %d exceeds int64", symbol, index)
		}
		items = append(items, market.Candle{
			Interval: request.Interval, OpenTime: time.UnixMilli(int64(kline.OpenTime)).UTC(), CloseTime: closeTime,
			Open: converted[0], High: converted[1], Low: converted[2], Close: converted[3], Volume: converted[4],
			QuoteAssetVolume: converted[5], TradeCount: int64(kline.NumberOfTrades),
		})
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
