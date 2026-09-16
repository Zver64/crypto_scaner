package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"crypto-scanner/internal/market"
)

const (
	defaultCandlePageSize = 200
	maxCandlePageSize     = 500
)

type CandleHistory interface {
	GetActiveInstrumentBySymbol(context.Context, string) (market.Instrument, error)
	ListCandlePage(context.Context, int64, market.CandleInterval, *time.Time, int) (market.CandlePage, error)
}

type candlePageResponse struct {
	Symbol     string           `json:"symbol"`
	Interval   string           `json:"interval"`
	Candles    []candleResponse `json:"candles"`
	HasMore    bool             `json:"has_more"`
	NextBefore *time.Time       `json:"next_before,omitempty"`
}

type candleResponse struct {
	OpenTime         time.Time `json:"open_time"`
	CloseTime        time.Time `json:"close_time"`
	Open             float64   `json:"open"`
	High             float64   `json:"high"`
	Low              float64   `json:"low"`
	Close            float64   `json:"close"`
	Volume           float64   `json:"volume"`
	QuoteAssetVolume float64   `json:"quote_asset_volume"`
	TradeCount       int64     `json:"trade_count"`
}

func listCandles(history CandleHistory) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		interval := market.CandleInterval(request.URL.Query().Get("interval"))
		if !interval.Valid() {
			writeAPIError(response, http.StatusBadRequest, "invalid_argument", "Unsupported candle interval", nil)
			return
		}
		limit, ok := candlePageLimit(request.URL.Query().Get("limit"))
		if !ok {
			writeAPIError(response, http.StatusBadRequest, "invalid_argument", "Invalid candle page limit", nil)
			return
		}
		before, ok := candleBefore(request.URL.Query().Get("before"))
		if !ok {
			writeAPIError(response, http.StatusBadRequest, "invalid_argument", "Invalid candle cursor", nil)
			return
		}
		symbol := strings.ToUpper(strings.TrimSpace(request.PathValue("symbol")))
		if symbol == "" {
			writeAPIError(response, http.StatusBadRequest, "invalid_argument", "Symbol is required", nil)
			return
		}
		instrument, err := history.GetActiveInstrumentBySymbol(request.Context(), symbol)
		if errors.Is(err, market.ErrInstrumentNotFound) {
			writeAPIError(response, http.StatusNotFound, "symbol_not_found", "Symbol is unknown or inactive", nil)
			return
		}
		if err != nil {
			writeAPIError(response, http.StatusInternalServerError, "internal_error", "Internal server error", nil)
			return
		}
		page, err := history.ListCandlePage(request.Context(), instrument.ID, interval, before, limit)
		if err != nil {
			writeAPIError(response, http.StatusInternalServerError, "internal_error", "Internal server error", nil)
			return
		}
		candles := make([]candleResponse, len(page.Candles))
		for index, candle := range page.Candles {
			candles[index] = candleResponse{
				OpenTime: candle.OpenTime.UTC(), CloseTime: candle.CloseTime.UTC(),
				Open: candle.Open, High: candle.High, Low: candle.Low, Close: candle.Close,
				Volume: candle.Volume, QuoteAssetVolume: candle.QuoteAssetVolume, TradeCount: candle.TradeCount,
			}
		}
		var nextBefore *time.Time
		if page.HasMore && len(page.Candles) > 0 {
			value := page.Candles[0].OpenTime.UTC()
			nextBefore = &value
		}
		writeJSON(response, http.StatusOK, candlePageResponse{
			Symbol: instrument.Symbol, Interval: string(interval), Candles: candles,
			HasMore: page.HasMore, NextBefore: nextBefore,
		})
	}
}

func candlePageLimit(value string) (int, bool) {
	if value == "" {
		return defaultCandlePageSize, true
	}
	limit, err := strconv.Atoi(value)
	return limit, err == nil && limit >= 1 && limit <= maxCandlePageSize
}

func candleBefore(value string) (*time.Time, bool) {
	if value == "" {
		return nil, true
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, false
	}
	parsed = parsed.UTC()
	return &parsed, true
}
