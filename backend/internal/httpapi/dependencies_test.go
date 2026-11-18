package httpapi_test

import (
	"log/slog"
	"net/http"
	"reflect"

	"crypto-scanner/internal/httpapi"
)

// stubDependencies stand in for the dependencies a test does not use;
// calling one of them panics.
var stubDependencies = httpapi.Dependencies{
	Readiness:         struct{ httpapi.Readiness }{},
	Analysis:          struct{ httpapi.Analysis }{},
	MarketTables:      struct{ httpapi.MarketTables }{},
	History:           struct{ httpapi.CandleHistory }{},
	GridLimits:        struct{ httpapi.GridLimits }{},
	Sessions:          struct{ httpapi.Sessions }{},
	APITokens:         struct{ httpapi.APITokens }{},
	Chart:             struct{ httpapi.ChartService }{},
	LiveCandles:       struct{ httpapi.LiveCandles }{},
	Favorites:         struct{ httpapi.Favorites }{},
	Alerts:            struct{ httpapi.PriceAlerts }{},
	ScannerIndicators: struct{ httpapi.ScannerIndicators }{},
	IndicatorTypes:    struct{ httpapi.IndicatorTypes }{},
	Users:             struct{ httpapi.Users }{},
	Strategies:        struct{ httpapi.Strategies }{},
	HistoryLoads:      struct{ httpapi.HistoryLoads }{},
}

// testDependencies fills the dependencies a test leaves out with stubs.
func testDependencies(dependencies httpapi.Dependencies) httpapi.Dependencies {
	value, stubs := reflect.ValueOf(&dependencies).Elem(), reflect.ValueOf(stubDependencies)
	for i := range value.NumField() {
		if value.Field(i).IsNil() {
			value.Field(i).Set(stubs.Field(i))
		}
	}
	return dependencies
}

// newTestHandler builds the API through New with the dependencies a test
// uses and stubs for the others.
func newTestHandler(logger *slog.Logger, dependencies httpapi.Dependencies, options httpapi.Options) http.Handler {
	handler, err := httpapi.New(logger, testDependencies(dependencies), options)
	if err != nil {
		panic(err)
	}
	return handler
}

// newTestHandlerWithAuthentication builds the API like newTestHandler with
// authenticate in place of session verification.
func newTestHandlerWithAuthentication(logger *slog.Logger, dependencies httpapi.Dependencies, options httpapi.Options, authenticate func(http.Handler) http.Handler) http.Handler {
	handler, err := httpapi.NewWithAuthentication(logger, testDependencies(dependencies), options, authenticate)
	if err != nil {
		panic(err)
	}
	return handler
}
