package httpapi

import (
	"context"
	"slices"
)

func (api *api) ListInstruments(ctx context.Context, request ListInstrumentsRequestObject) (ListInstrumentsResponseObject, error) {
	var coinM []string
	filtered := request.Params.Universe != nil && *request.Params.Universe == CoinmPerpetual
	if filtered {
		var err error
		if coinM, err = api.tables.CoinMSymbols(ctx); err != nil {
			if apiErr, ok := analysisError(ctx, err, ""); ok {
				return ListInstruments503JSONResponse{apiErr.unavailable()}, nil
			}
			return ListInstruments500JSONResponse{api.internalError(ctx, "list_coinm_symbols", err)}, nil
		}
	}
	instruments, err := api.history.ListActiveInstruments(ctx)
	if err != nil {
		return ListInstruments500JSONResponse{api.internalError(ctx, "list_instruments", err)}, nil
	}
	items := make([]InstrumentListItem, 0, len(instruments))
	for _, instrument := range instruments {
		if !filtered || slices.Contains(coinM, instrument.Symbol) {
			items = append(items, InstrumentListItem{Symbol: instrument.Symbol})
		}
	}
	return ListInstruments200JSONResponse{Items: items}, nil
}
