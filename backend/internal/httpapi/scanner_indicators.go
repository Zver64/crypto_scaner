package httpapi

import (
	"context"
	"errors"
	"net/http"

	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/scannerindicator"
)

// ScannerIndicators manages the global indicator configuration.
type ScannerIndicators interface {
	List() []scannerindicator.Entry
	Create(context.Context, scannerindicator.Indicator) (scannerindicator.Entry, error)
	Update(context.Context, int64, []scannerindicator.Period, scannerindicator.Scale) (scannerindicator.Entry, error)
	Delete(context.Context, int64) error
	Reorder(context.Context, []int64) ([]scannerindicator.Entry, error)
}

// IndicatorTypes describes every indicator the scanner can calculate.
type IndicatorTypes interface {
	Descriptors() []indicator.Descriptor
}

func (api *api) GetCurrentUser(ctx context.Context, _ GetCurrentUserRequestObject) (GetCurrentUserResponseObject, error) {
	user, _ := UserFromContext(ctx)
	return GetCurrentUser200JSONResponse{Administrator: user.Administrator}, nil
}

func (api *api) ListIndicatorTypes(context.Context, ListIndicatorTypesRequestObject) (ListIndicatorTypesResponseObject, error) {
	descriptors := api.indicatorTypes.Descriptors()
	items := make([]IndicatorType, len(descriptors))
	for i, descriptor := range descriptors {
		items[i] = indicatorTypeDTO(descriptor)
	}
	return ListIndicatorTypes200JSONResponse{Items: items}, nil
}

func (api *api) ListScannerIndicators(context.Context, ListScannerIndicatorsRequestObject) (ListScannerIndicatorsResponseObject, error) {
	return ListScannerIndicators200JSONResponse(scannerIndicatorListDTO(api.scannerIndicators.List())), nil
}

func scannerIndicatorListDTO(entries []scannerindicator.Entry) ScannerIndicatorList {
	items := make([]ScannerIndicator, len(entries))
	for i, entry := range entries {
		items[i] = scannerIndicatorDTO(entry)
	}
	return ScannerIndicatorList{Items: items}
}

func (api *api) CreateScannerIndicator(ctx context.Context, request CreateScannerIndicatorRequestObject) (CreateScannerIndicatorResponseObject, error) {
	entry, err := api.scannerIndicators.Create(ctx, scannerindicator.Indicator{
		Selection: indicator.Selection{Type: indicator.Type(request.Body.Type), Parameters: indicator.Parameters(request.Body.Parameters)},
		Periods:   periodsFromDTO(request.Body.Periods),
		Scale:     scaleFromDTO(request.Body.Scale),
	})
	switch {
	case err == nil:
		return CreateScannerIndicator201JSONResponse(scannerIndicatorDTO(entry)), nil
	case errors.Is(err, scannerindicator.ErrInvalidArgument):
		return CreateScannerIndicator400JSONResponse{invalidArgument(ctx, err.Error()).badRequest()}, nil
	case errors.Is(err, scannerindicator.ErrConflict):
		return CreateScannerIndicator409JSONResponse{ScannerIndicatorConflictJSONResponse(newAPIError(ctx, http.StatusConflict, "scanner_indicator_exists", err.Error(), nil).body)}, nil
	case errors.Is(err, scannerindicator.ErrLimit):
		return CreateScannerIndicator409JSONResponse{ScannerIndicatorConflictJSONResponse(newAPIError(ctx, http.StatusConflict, "scanner_indicator_limit", err.Error(), nil).body)}, nil
	default:
		return CreateScannerIndicator500JSONResponse{api.internalError(ctx, "create_scanner_indicator", err)}, nil
	}
}

func (api *api) UpdateScannerIndicator(ctx context.Context, request UpdateScannerIndicatorRequestObject) (UpdateScannerIndicatorResponseObject, error) {
	entry, err := api.scannerIndicators.Update(ctx, request.IndicatorId, periodsFromDTO(request.Body.Periods), scaleFromDTO(request.Body.Scale))
	switch {
	case err == nil:
		return UpdateScannerIndicator200JSONResponse(scannerIndicatorDTO(entry)), nil
	case errors.Is(err, scannerindicator.ErrInvalidArgument):
		return UpdateScannerIndicator400JSONResponse{invalidArgument(ctx, err.Error()).badRequest()}, nil
	case errors.Is(err, scannerindicator.ErrLimit):
		return UpdateScannerIndicator409JSONResponse{ScannerIndicatorConflictJSONResponse(newAPIError(ctx, http.StatusConflict, "scanner_indicator_limit", err.Error(), nil).body)}, nil
	case errors.Is(err, scannerindicator.ErrNotFound):
		return UpdateScannerIndicator404JSONResponse{ScannerIndicatorNotFoundJSONResponse(scannerIndicatorNotFound(ctx).body)}, nil
	default:
		return UpdateScannerIndicator500JSONResponse{api.internalError(ctx, "update_scanner_indicator", err)}, nil
	}
}

func (api *api) ReorderScannerIndicators(ctx context.Context, request ReorderScannerIndicatorsRequestObject) (ReorderScannerIndicatorsResponseObject, error) {
	entries, err := api.scannerIndicators.Reorder(ctx, request.Body.Ids)
	switch {
	case err == nil:
		return ReorderScannerIndicators200JSONResponse(scannerIndicatorListDTO(entries)), nil
	case errors.Is(err, scannerindicator.ErrInvalidArgument):
		return ReorderScannerIndicators400JSONResponse{invalidArgument(ctx, err.Error()).badRequest()}, nil
	case errors.Is(err, scannerindicator.ErrNotFound):
		return ReorderScannerIndicators404JSONResponse{ScannerIndicatorNotFoundJSONResponse(scannerIndicatorNotFound(ctx).body)}, nil
	default:
		return ReorderScannerIndicators500JSONResponse{api.internalError(ctx, "reorder_scanner_indicators", err)}, nil
	}
}

func (api *api) DeleteScannerIndicator(ctx context.Context, request DeleteScannerIndicatorRequestObject) (DeleteScannerIndicatorResponseObject, error) {
	err := api.scannerIndicators.Delete(ctx, request.IndicatorId)
	switch {
	case err == nil:
		return DeleteScannerIndicator204Response{}, nil
	case errors.Is(err, scannerindicator.ErrNotFound):
		return DeleteScannerIndicator404JSONResponse{ScannerIndicatorNotFoundJSONResponse(scannerIndicatorNotFound(ctx).body)}, nil
	default:
		return DeleteScannerIndicator500JSONResponse{api.internalError(ctx, "delete_scanner_indicator", err)}, nil
	}
}

func scannerIndicatorNotFound(ctx context.Context) apiError {
	return newAPIError(ctx, http.StatusNotFound, "scanner_indicator_not_found", "Scanner indicator does not exist", nil)
}

func periodsFromDTO(items ScannerIndicatorPeriods) []scannerindicator.Period {
	periods := make([]scannerindicator.Period, len(items))
	for i, item := range items {
		periods[i] = scannerindicator.Period{Interval: market.CandleInterval(item.Interval), ShowInTable: item.ShowInTable, ShowInChart: item.ShowInChart}
	}
	return periods
}

func scaleFromDTO(scale *ScannerIndicatorScale) scannerindicator.Scale {
	if scale == nil {
		return scannerindicator.Scale{}
	}
	return scannerindicator.Scale{Min: scale.Min, Max: scale.Max, Levels: scale.Levels}
}

func scannerIndicatorDTO(entry scannerindicator.Entry) ScannerIndicator {
	periods := make(ScannerIndicatorPeriods, len(entry.Periods))
	for i, period := range entry.Periods {
		periods[i] = ScannerIndicatorPeriod{Interval: CandleInterval(period.Interval), ShowInTable: period.ShowInTable, ShowInChart: period.ShowInChart}
	}
	return ScannerIndicator{
		Id:         entry.ID,
		Type:       string(entry.Selection.Type),
		Parameters: entry.Selection.Parameters,
		Periods:    periods,
		Scale:      ScannerIndicatorScale{Min: entry.Scale.Min, Max: entry.Scale.Max, Levels: append([]float64{}, entry.Scale.Levels...)},
		Title:      entry.Title,
		Placement:  ScannerIndicatorPlacement(entry.Placement),
		Outputs:    entry.Outputs,
	}
}

func indicatorTypeDTO(descriptor indicator.Descriptor) IndicatorType {
	item := IndicatorType{
		Type:       string(descriptor.Type),
		Title:      descriptor.Title,
		Group:      descriptor.Group,
		Overlay:    descriptor.Overlay,
		Parameters: make([]IndicatorParameter, len(descriptor.Parameters)),
		Outputs:    make([]string, len(descriptor.Outputs)),
	}
	for i, parameter := range descriptor.Parameters {
		// Clients choose a source among its candle fields.
		if parameter.Kind == indicator.ParameterSource {
			parameter.Kind = indicator.ParameterChoice
		}
		dto := IndicatorParameter{Key: parameter.Key, Title: parameter.Title, Description: parameter.Description, Kind: IndicatorParameterKind(parameter.Kind), Default: parameter.Default}
		if parameter.Kind == indicator.ParameterChoice {
			choices := make([]IndicatorChoice, len(parameter.Choices))
			for j, choice := range parameter.Choices {
				choices[j] = IndicatorChoice{Value: choice.Value, Title: choice.Title}
			}
			dto.Choices = &choices
		} else {
			dto.Minimum, dto.Maximum = &parameter.Minimum, &parameter.Maximum
		}
		item.Parameters[i] = dto
	}
	for i, output := range descriptor.Outputs {
		item.Outputs[i] = output.Name
	}
	return item
}
