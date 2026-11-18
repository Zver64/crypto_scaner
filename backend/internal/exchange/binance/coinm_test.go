package binance

import (
	"encoding/json"
	"maps"
	"slices"
	"testing"

	"crypto-scanner/internal/market/gridlimits"

	"github.com/binance/binance-connector-go/clients/derivativestradingcoinfutures/src/restapi/models"
)

func TestPerpetualBaseAssetsKeepsOnlyTradingPerpetuals(t *testing.T) {
	var response models.ExchangeInformationResponse
	payload := `{"symbols":[
		{"symbol":"BTCUSD_PERP","contractType":"PERPETUAL","contractStatus":"TRADING","baseAsset":"BTC"},
		{"symbol":"BTCUSD_251226","contractType":"CURRENT_QUARTER","contractStatus":"TRADING","baseAsset":"BTC"},
		{"symbol":"LUNAUSD_PERP","contractType":"PERPETUAL","contractStatus":"SETTLING","baseAsset":"LUNA"},
		{"symbol":"ETHUSD_PERP","contractType":"PERPETUAL","contractStatus":"TRADING","baseAsset":"ETH"}
	]}`
	if err := json.Unmarshal([]byte(payload), &response); err != nil {
		t.Fatal(err)
	}
	if got := perpetualBaseAssets(response.Symbols); !slices.Equal(got, []string{"BTC", "ETH"}) {
		t.Fatalf("base assets = %v", got)
	}
}

func TestCoinMContractsKeyTradingUSDPerpetualsByBaseAsset(t *testing.T) {
	var response models.ExchangeInformationResponse
	payload := `{"symbols":[
		{"symbol":"BTCUSD_PERP","contractType":"PERPETUAL","contractStatus":"TRADING","baseAsset":"BTC","quoteAsset":"USD","contractSize":100,"equalQtyPrecision":4,"filters":[
			{"filterType":"PRICE_FILTER","minPrice":"1000","maxPrice":"4520958","tickSize":"0.1"},
			{"filterType":"LOT_SIZE","minQty":"1","maxQty":"1000000","stepSize":"1"},
			{"filterType":"MARKET_LOT_SIZE","minQty":"1","maxQty":"60000","stepSize":"1"}]},
		{"symbol":"BTCUSD_251226","contractType":"CURRENT_QUARTER","contractStatus":"TRADING","baseAsset":"BTC","quoteAsset":"USD","contractSize":100,"filters":[]},
		{"symbol":"LUNAUSD_PERP","contractType":"PERPETUAL","contractStatus":"SETTLING","baseAsset":"LUNA","quoteAsset":"USD","contractSize":10,"filters":[]}
	]}`
	if err := json.Unmarshal([]byte(payload), &response); err != nil {
		t.Fatal(err)
	}
	got, err := coinMContracts(response.Symbols)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]gridlimits.CoinmContract{"BTC": {Symbol: "BTCUSD_PERP", TickSize: 0.1, MinQty: 1, StepSize: 1, ContractSize: 100, EqualQtyPrecision: 4}}
	if !maps.Equal(got, want) {
		t.Fatalf("contracts = %+v, want %+v", got, want)
	}
}
