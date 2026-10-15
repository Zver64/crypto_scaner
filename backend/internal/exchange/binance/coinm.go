package binance

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"crypto-scanner/internal/platform/backoff"

	coinm "github.com/binance/binance-connector-go/clients/derivativestradingcoinfutures"
	"github.com/binance/binance-connector-go/clients/derivativestradingcoinfutures/src/restapi/models"
	"github.com/binance/binance-connector-go/common/v2/common"
	"golang.org/x/sync/singleflight"
)

const (
	// coinMPerpetualsTTL bounds how stale the COIN-M contract list may become;
	// Binance lists new COIN-M perpetuals rarely.
	coinMPerpetualsTTL = time.Hour
	// coinMPerpetualsRetryDelay is the first delay after a failed refresh; it
	// doubles with every further failure up to coinMPerpetualsTTL.
	coinMPerpetualsRetryDelay = time.Minute
)

// CoinMPerpetuals lists the base assets of trading Binance COIN-M perpetual
// contracts. The list is cached; concurrent callers share one refresh, and a
// failed refresh keeps serving the last list until a delayed retry.
type CoinMPerpetuals struct {
	client  *coinm.BinanceDerivativesTradingCoinFuturesClient
	logger  *slog.Logger
	refresh singleflight.Group

	mu        sync.Mutex
	assets    []string
	err       error
	failures  int
	refreshAt time.Time
}

// NewCoinMPerpetuals creates the COIN-M perpetual list served from the
// official Binance endpoint.
func NewCoinMPerpetuals(logger *slog.Logger) *CoinMPerpetuals {
	configuration := common.NewConfigurationRestAPI(
		common.WithBasePath(common.DerivativesTradingCoinFuturesRestApiProdUrl),
		common.WithTimeout(30*time.Second),
	)
	return &CoinMPerpetuals{
		client: coinm.NewBinanceDerivativesTradingCoinFuturesClient(coinm.WithRestAPI(configuration)),
		logger: logger.With("module", "binance"),
	}
}

func (perpetuals *CoinMPerpetuals) BaseAssets(ctx context.Context) ([]string, error) {
	perpetuals.mu.Lock()
	due := !time.Now().Before(perpetuals.refreshAt)
	perpetuals.mu.Unlock()
	if due {
		perpetuals.refresh.Do("", func() (any, error) {
			perpetuals.load(ctx)
			return nil, nil
		})
	}
	perpetuals.mu.Lock()
	defer perpetuals.mu.Unlock()
	switch {
	case perpetuals.assets != nil:
		return slices.Clone(perpetuals.assets), nil
	case perpetuals.err != nil:
		return nil, perpetuals.err
	default:
		// The shared refresh was abandoned by the caller that started it.
		return nil, errors.New("Binance COIN-M perpetuals are not loaded")
	}
}

func (perpetuals *CoinMPerpetuals) load(ctx context.Context) {
	response, err := perpetuals.client.RestApi.MarketDataAPI.ExchangeInformation(ctx).Execute()
	if ctx.Err() != nil {
		// The caller left; the next caller retries.
		return
	}
	perpetuals.mu.Lock()
	defer perpetuals.mu.Unlock()
	if err != nil {
		perpetuals.err = fmt.Errorf("list Binance COIN-M exchange information: %w", err)
		perpetuals.refreshAt = time.Now().Add(backoff.Exponential(coinMPerpetualsRetryDelay, coinMPerpetualsTTL, perpetuals.failures))
		perpetuals.failures++
		perpetuals.logger.WarnContext(ctx, "Binance COIN-M perpetuals refresh failed", "stale", perpetuals.assets != nil, "error", err)
		return
	}
	perpetuals.assets = perpetualBaseAssets(response.Data.Symbols)
	perpetuals.err = nil
	perpetuals.failures = 0
	perpetuals.refreshAt = time.Now().Add(coinMPerpetualsTTL)
}

func perpetualBaseAssets(symbols []models.ExchangeInformationResponseSymbolsInner) []string {
	assets := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		if symbol.GetContractType() == "PERPETUAL" && symbol.GetContractStatus() == "TRADING" {
			assets = append(assets, symbol.GetBaseAsset())
		}
	}
	return assets
}
