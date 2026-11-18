package binance

import (
	"context"
	"fmt"
	"time"

	"crypto-scanner/internal/market/gridlimits"
	"crypto-scanner/internal/platform/numeric"

	coinm "github.com/binance/binance-connector-go/clients/derivativestradingcoinfutures"
	"github.com/binance/binance-connector-go/clients/derivativestradingcoinfutures/src/restapi/models"
	"github.com/binance/binance-connector-go/common/v2/common"
)

// CoinM reads Binance COIN-M perpetual contracts from the official endpoint.
type CoinM struct {
	client *coinm.BinanceDerivativesTradingCoinFuturesClient
}

// NewCoinM creates the COIN-M reader served from the official Binance
// endpoint.
func NewCoinM() *CoinM {
	configuration := common.NewConfigurationRestAPI(
		common.WithBasePath(common.DerivativesTradingCoinFuturesRestApiProdUrl),
		common.WithTimeout(30*time.Second),
	)
	return &CoinM{client: coinm.NewBinanceDerivativesTradingCoinFuturesClient(coinm.WithRestAPI(configuration))}
}

// BaseAssets lists the base assets of trading COIN-M perpetual contracts.
func (coinM *CoinM) BaseAssets(ctx context.Context) ([]string, error) {
	response, err := coinM.client.RestApi.MarketDataAPI.ExchangeInformation(ctx).Execute()
	if err != nil {
		return nil, fmt.Errorf("list Binance COIN-M exchange information: %w", err)
	}
	return perpetualBaseAssets(response.Data.Symbols), nil
}

func perpetualBaseAssets(symbols []models.ExchangeInformationResponseSymbolsInner) []string {
	assets := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		if tradingCoinMPerpetual(symbol) {
			assets = append(assets, symbol.GetBaseAsset())
		}
	}
	return assets
}

func tradingCoinMPerpetual(symbol models.ExchangeInformationResponseSymbolsInner) bool {
	return symbol.GetContractType() == "PERPETUAL" && symbol.GetContractStatus() == "TRADING"
}

// Contracts returns the trading COIN-M USD perpetual contracts keyed by base
// asset.
func (coinM *CoinM) Contracts(ctx context.Context) (map[string]gridlimits.CoinmContract, error) {
	response, err := coinM.client.RestApi.MarketDataAPI.ExchangeInformation(ctx).Execute()
	if err != nil {
		return nil, fmt.Errorf("list Binance COIN-M exchange information: %w", err)
	}
	contracts, err := coinMContracts(response.Data.Symbols)
	if err != nil {
		return nil, fmt.Errorf("list Binance COIN-M contracts: %w", err)
	}
	return contracts, nil
}

func coinMContracts(symbols []models.ExchangeInformationResponseSymbolsInner) (map[string]gridlimits.CoinmContract, error) {
	contracts := make(map[string]gridlimits.CoinmContract)
	for _, symbol := range symbols {
		if !tradingCoinMPerpetual(symbol) || symbol.GetQuoteAsset() != "USD" {
			continue
		}
		if _, exists := contracts[symbol.GetBaseAsset()]; exists {
			return nil, fmt.Errorf("duplicate perpetual of base asset %q", symbol.GetBaseAsset())
		}
		contract := gridlimits.CoinmContract{
			Symbol:            symbol.GetSymbol(),
			ContractSize:      float64(symbol.GetContractSize()),
			EqualQtyPrecision: int(symbol.GetEqualQtyPrecision()),
		}
		for _, filter := range symbol.Filters {
			var fields []filterField
			switch filter.GetFilterType() {
			case "PRICE_FILTER":
				fields = []filterField{{filter.GetTickSize(), &contract.TickSize}}
			case "LOT_SIZE":
				fields = []filterField{{filter.GetMinQty(), &contract.MinQty}, {filter.GetStepSize(), &contract.StepSize}}
			}
			if err := parseFilter(filter.GetFilterType(), fields); err != nil {
				return nil, fmt.Errorf("%s: %w", symbol.GetSymbol(), err)
			}
		}
		contracts[symbol.GetBaseAsset()] = contract
	}
	return contracts, nil
}

// MarkPrice returns the mark price of a COIN-M contract.
func (coinM *CoinM) MarkPrice(ctx context.Context, symbol string) (float64, error) {
	response, err := coinM.client.RestApi.MarketDataAPI.IndexPriceAndMarkPrice(ctx).Symbol(symbol).Execute()
	if err != nil {
		return 0, fmt.Errorf("get Binance COIN-M mark price for %s: %w", symbol, err)
	}
	for _, item := range response.Data.Items {
		if item.GetSymbol() == symbol {
			return parseMarkPrice("COIN-M", symbol, item.GetMarkPrice())
		}
	}
	return 0, fmt.Errorf("get Binance COIN-M mark price for %s: symbol missing from the response", symbol)
}

// parseMarkPrice parses a mark price, which must be positive.
func parseMarkPrice(marketName, symbol, value string) (float64, error) {
	price, err := numeric.ParseFinite(value)
	if err != nil || price <= 0 {
		return 0, fmt.Errorf("get Binance %s mark price for %s: invalid price %q", marketName, symbol, value)
	}
	return price, nil
}
