package binance

import (
	"encoding/json"
	"slices"
	"testing"

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
