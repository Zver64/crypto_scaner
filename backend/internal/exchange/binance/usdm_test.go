package binance

import (
	"encoding/json"
	"maps"
	"testing"

	"crypto-scanner/internal/market/gridlimits"

	usdmmodels "github.com/binance/binance-connector-go/clients/derivativestradingusdsfutures/src/restapi/models"
)

func TestUsdMContractsKeyTradingUSDTPerpetualsByBaseAsset(t *testing.T) {
	var response usdmmodels.ExchangeInformationResponse
	payload := `{"symbols":[
		{"symbol":"BTCUSDT","contractType":"PERPETUAL","status":"TRADING","baseAsset":"BTC","quoteAsset":"USDT","marginAsset":"USDT","filters":[
			{"filterType":"PRICE_FILTER","minPrice":"261.10","maxPrice":"809484","tickSize":"0.10"},
			{"filterType":"LOT_SIZE","minQty":"0.001","maxQty":"1000","stepSize":"0.001"},
			{"filterType":"MARKET_LOT_SIZE","minQty":"0.001","maxQty":"120","stepSize":"0.001"},
			{"filterType":"MIN_NOTIONAL","notional":"100"},
			{"filterType":"PERCENT_PRICE","multiplierUp":"1.0500","multiplierDown":"0.9500","multiplierDecimal":"4"}]},
		{"symbol":"BTCUSDC","contractType":"PERPETUAL","status":"TRADING","baseAsset":"BTC","quoteAsset":"USDC","marginAsset":"USDC","filters":[]},
		{"symbol":"BTCUSDT_251226","contractType":"CURRENT_QUARTER","status":"TRADING","baseAsset":"BTC","quoteAsset":"USDT","marginAsset":"USDT","filters":[]},
		{"symbol":"LUNAUSDT","contractType":"PERPETUAL","status":"SETTLING","baseAsset":"LUNA","quoteAsset":"USDT","marginAsset":"USDT","filters":[]},
		{"symbol":"1000PEPEUSDT","contractType":"PERPETUAL","status":"TRADING","baseAsset":"1000PEPE","quoteAsset":"USDT","marginAsset":"USDT","filters":[]}
	]}`
	if err := json.Unmarshal([]byte(payload), &response); err != nil {
		t.Fatal(err)
	}
	got, err := usdMContracts(response.Symbols)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]gridlimits.UsdmContract{"1000PEPE": {Symbol: "1000PEPEUSDT"}, "BTC": {Symbol: "BTCUSDT", TickSize: 0.1, MinQty: 0.001, StepSize: 0.001, MinNotional: 100}}
	if !maps.Equal(got, want) {
		t.Fatalf("contracts = %+v, want %+v", got, want)
	}
}
