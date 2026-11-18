package binance

import (
	"context"
	"fmt"
	"time"

	"crypto-scanner/internal/market/gridlimits"

	usdm "github.com/binance/binance-connector-go/clients/derivativestradingusdsfutures"
	usdmmodels "github.com/binance/binance-connector-go/clients/derivativestradingusdsfutures/src/restapi/models"
	"github.com/binance/binance-connector-go/common/v2/common"
)

// UsdM reads Binance USD-M perpetual contracts from the official endpoint.
type UsdM struct {
	client *usdm.BinanceDerivativesTradingUsdsFuturesClient
}

// NewUsdM creates the USD-M reader served from the official Binance
// endpoint.
func NewUsdM() *UsdM {
	configuration := common.NewConfigurationRestAPI(
		common.WithBasePath(common.DerivativesTradingUsdsFuturesRestApiProdUrl),
		common.WithTimeout(30*time.Second),
	)
	return &UsdM{client: usdm.NewBinanceDerivativesTradingUsdsFuturesClient(usdm.WithRestAPI(configuration))}
}

// Contracts returns the trading USD-M USDT perpetual contracts keyed by base
// asset.
func (usdM *UsdM) Contracts(ctx context.Context) (map[string]gridlimits.UsdmContract, error) {
	response, err := usdM.client.RestApi.MarketDataAPI.ExchangeInformation(ctx).Execute()
	if err != nil {
		return nil, fmt.Errorf("list Binance USD-M exchange information: %w", err)
	}
	contracts, err := usdMContracts(response.Data.Symbols)
	if err != nil {
		return nil, fmt.Errorf("list Binance USD-M contracts: %w", err)
	}
	return contracts, nil
}

func usdMContracts(symbols []usdmmodels.ExchangeInformationResponseSymbolsInner) (map[string]gridlimits.UsdmContract, error) {
	contracts := make(map[string]gridlimits.UsdmContract)
	for _, symbol := range symbols {
		if symbol.GetContractType() != "PERPETUAL" || symbol.GetStatus() != "TRADING" ||
			symbol.GetQuoteAsset() != "USDT" || symbol.GetMarginAsset() != "USDT" {
			continue
		}
		if _, exists := contracts[symbol.GetBaseAsset()]; exists {
			return nil, fmt.Errorf("duplicate perpetual of base asset %q", symbol.GetBaseAsset())
		}
		contract := gridlimits.UsdmContract{Symbol: symbol.GetSymbol()}
		for _, filter := range symbol.Filters {
			var fields []filterField
			switch filter.GetFilterType() {
			case "PRICE_FILTER":
				fields = []filterField{{filter.GetTickSize(), &contract.TickSize}}
			case "LOT_SIZE":
				fields = []filterField{{filter.GetMinQty(), &contract.MinQty}, {filter.GetStepSize(), &contract.StepSize}}
			case "MIN_NOTIONAL":
				fields = []filterField{{filter.GetNotional(), &contract.MinNotional}}
			}
			if err := parseFilter(filter.GetFilterType(), fields); err != nil {
				return nil, fmt.Errorf("%s: %w", symbol.GetSymbol(), err)
			}
		}
		contracts[symbol.GetBaseAsset()] = contract
	}
	return contracts, nil
}

// MarkPrice returns the mark price of a USD-M contract.
func (usdM *UsdM) MarkPrice(ctx context.Context, symbol string) (float64, error) {
	response, err := usdM.client.RestApi.MarketDataAPI.MarkPrice(ctx).Symbol(symbol).Execute()
	if err != nil {
		return 0, fmt.Errorf("get Binance USD-M mark price for %s: %w", symbol, err)
	}
	item := response.Data.MarkPriceResponse1
	if item == nil || item.GetSymbol() != symbol {
		return 0, fmt.Errorf("get Binance USD-M mark price for %s: symbol missing from the response", symbol)
	}
	return parseMarkPrice("USD-M", symbol, item.GetMarkPrice())
}
