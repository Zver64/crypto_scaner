package httpapi

import "context"

func (api *api) ListInstruments(ctx context.Context, _ ListInstrumentsRequestObject) (ListInstrumentsResponseObject, error) {
	instruments, err := api.history.ListActiveInstruments(ctx)
	if err != nil {
		return ListInstruments500JSONResponse{api.internalError(ctx, "list_instruments", err)}, nil
	}
	items := make([]InstrumentListItem, len(instruments))
	for i, instrument := range instruments {
		items[i] = InstrumentListItem{Symbol: instrument.Symbol}
	}
	return ListInstruments200JSONResponse{Items: items}, nil
}
