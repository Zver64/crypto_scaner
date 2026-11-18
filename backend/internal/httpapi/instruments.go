package httpapi

import (
	"context"

	"crypto-scanner/internal/markettable"
)

func (api *api) ListInstruments(ctx context.Context, request ListInstrumentsRequestObject) (ListInstrumentsResponseObject, error) {
	universe := markettable.UniverseSpot
	if request.Params.Universe != nil {
		universe = markettable.Universe(*request.Params.Universe)
	}
	symbols, err := api.MarketTables.Symbols(ctx, currentUserID(ctx), universe)
	if err != nil {
		return ListInstruments500JSONResponse{api.internalError(ctx, "list_instruments", err)}, nil
	}
	items := make([]InstrumentListItem, len(symbols))
	for i, symbol := range symbols {
		items[i] = InstrumentListItem{Symbol: symbol}
	}
	return ListInstruments200JSONResponse{Items: items}, nil
}
