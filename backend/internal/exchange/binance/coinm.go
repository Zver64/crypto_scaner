package binance

import (
	"context"
	"fmt"
	"time"

	coinm "github.com/binance/binance-connector-go/clients/derivativestradingcoinfutures"
	"github.com/binance/binance-connector-go/clients/derivativestradingcoinfutures/src/restapi/models"
	"github.com/binance/binance-connector-go/common/v2/common"
)

// CoinMPerpetuals lists the base assets of trading Binance COIN-M perpetual
// contracts.
type CoinMPerpetuals struct {
	client *coinm.BinanceDerivativesTradingCoinFuturesClient
}

// NewCoinMPerpetuals creates the COIN-M perpetual list served from the
// official Binance endpoint.
func NewCoinMPerpetuals() *CoinMPerpetuals {
	configuration := common.NewConfigurationRestAPI(
		common.WithBasePath(common.DerivativesTradingCoinFuturesRestApiProdUrl),
		common.WithTimeout(30*time.Second),
	)
	return &CoinMPerpetuals{client: coinm.NewBinanceDerivativesTradingCoinFuturesClient(coinm.WithRestAPI(configuration))}
}

func (perpetuals *CoinMPerpetuals) BaseAssets(ctx context.Context) ([]string, error) {
	response, err := perpetuals.client.RestApi.MarketDataAPI.ExchangeInformation(ctx).Execute()
	if err != nil {
		return nil, fmt.Errorf("list Binance COIN-M exchange information: %w", err)
	}
	return perpetualBaseAssets(response.Data.Symbols), nil
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
