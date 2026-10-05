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
	Create(context.Context, []scannerindicator.Indicator) ([]scannerindicator.Entry, error)
	Update(context.Context, int64, bool, scannerindicator.Scale) (scannerindicator.Entry, error)
	Delete(context.Context, int64) error
	DeleteUnused(context.Context) error
	Reorder(context.Context, []int64) ([]scannerindicator.Entry, error)
	// Usage maps indicator ids to the names of the strategies that read them.
	Usage() map[int64][]string
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
	return ListScannerIndicators200JSONResponse(api.scannerIndicatorListDTO(api.scannerIndicators.List())), nil
}

func (api *api) scannerIndicatorListDTO(entries []scannerindicator.Entry) ScannerIndicatorList {
	usage := api.scannerIndicators.Usage()
	items := make([]ScannerIndicator, len(entries))
	for i, entry := range entries {
		items[i] = scannerIndicatorDTO(entry, usage)
	}
	return ScannerIndicatorList{Items: items}
}

func (api *api) CreateScannerIndicator(ctx context.Context, request CreateScannerIndicatorRequestObject) (CreateScannerIndicatorResponseObject, error) {
	selection := indicator.Selection{Type: indicator.Type(request.Body.Type), Parameters: indicator.Parameters(request.Body.Parameters)}
	scale := scaleFromDTO(request.Body.Scale)
	items := make([]scannerindicator.Indicator, len(request.Body.Intervals))
	for i, interval := range request.Body.Intervals {
		items[i] = scannerindicator.Indicator{Interval: market.CandleInterval(interval.Interval), Selection: selection, ShowInTable: interval.ShowInTable, Scale: scale}
	}
	entries, err := api.scannerIndicators.Create(ctx, items)
	switch {
	case err == nil:
		return CreateScannerIndicator201JSONResponse(api.scannerIndicatorListDTO(entries)), nil
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
	entry, err := api.scannerIndicators.Update(ctx, request.IndicatorId, request.Body.ShowInTable, scaleFromDTO(request.Body.Scale))
	switch {
	case err == nil:
		return UpdateScannerIndicator200JSONResponse(scannerIndicatorDTO(entry, api.scannerIndicators.Usage())), nil
	case errors.Is(err, scannerindicator.ErrInvalidArgument):
		return UpdateScannerIndicator400JSONResponse{invalidArgument(ctx, err.Error()).badRequest()}, nil
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
		return ReorderScannerIndicators200JSONResponse(api.scannerIndicatorListDTO(entries)), nil
	case errors.Is(err, scannerindicator.ErrInvalidArgument):
		return ReorderScannerIndicators400JSONResponse{invalidArgument(ctx, err.Error()).badRequest()}, nil
	case errors.Is(err, scannerindicator.ErrNotFound):
		return ReorderScannerIndicators404JSONResponse{ScannerIndicatorNotFoundJSONResponse(scannerIndicatorNotFound(ctx).body)}, nil
	default:
		return ReorderScannerIndicators500JSONResponse{api.internalError(ctx, "reorder_scanner_indicators", err)}, nil
	}
}

func (api *api) DeleteUnusedScannerIndicators(ctx context.Context, _ DeleteUnusedScannerIndicatorsRequestObject) (DeleteUnusedScannerIndicatorsResponseObject, error) {
	err := api.scannerIndicators.DeleteUnused(ctx)
	switch {
	case err == nil:
		return DeleteUnusedScannerIndicators204Response{}, nil
	case errors.Is(err, scannerindicator.ErrInUse):
		return DeleteUnusedScannerIndicators409JSONResponse{scannerIndicatorInUse(ctx)}, nil
	default:
		return DeleteUnusedScannerIndicators500JSONResponse{api.internalError(ctx, "delete_unused_scanner_indicators", err)}, nil
	}
}

func (api *api) DeleteScannerIndicator(ctx context.Context, request DeleteScannerIndicatorRequestObject) (DeleteScannerIndicatorResponseObject, error) {
	err := api.scannerIndicators.Delete(ctx, request.IndicatorId)
	switch {
	case err == nil:
		return DeleteScannerIndicator204Response{}, nil
	case errors.Is(err, scannerindicator.ErrNotFound):
		return DeleteScannerIndicator404JSONResponse{ScannerIndicatorNotFoundJSONResponse(scannerIndicatorNotFound(ctx).body)}, nil
	case errors.Is(err, scannerindicator.ErrInUse):
		return DeleteScannerIndicator409JSONResponse{scannerIndicatorInUse(ctx)}, nil
	default:
		return DeleteScannerIndicator500JSONResponse{api.internalError(ctx, "delete_scanner_indicator", err)}, nil
	}
}

func scannerIndicatorNotFound(ctx context.Context) apiError {
	return newAPIError(ctx, http.StatusNotFound, "scanner_indicator_not_found", "Scanner indicator does not exist", nil)
}

func scannerIndicatorInUse(ctx context.Context) ScannerIndicatorInUseJSONResponse {
	return ScannerIndicatorInUseJSONResponse(newAPIError(ctx, http.StatusConflict, "scanner_indicator_in_use", "A strategy reads the indicator", nil).body)
}

func scaleFromDTO(scale *ScannerIndicatorScale) scannerindicator.Scale {
	if scale == nil {
		return scannerindicator.Scale{}
	}
	return scannerindicator.Scale{Min: scale.Min, Max: scale.Max, Levels: scale.Levels}
}

func scannerIndicatorDTO(entry scannerindicator.Entry, usage map[int64][]string) ScannerIndicator {
	return ScannerIndicator{
		Id:          entry.ID,
		Interval:    CandleInterval(entry.Interval),
		Type:        string(entry.Selection.Type),
		Parameters:  entry.Selection.Parameters,
		ShowInTable: entry.ShowInTable,
		Scale:       ScannerIndicatorScale{Min: entry.Scale.Min, Max: entry.Scale.Max, Levels: append([]float64{}, entry.Scale.Levels...)},
		Title:       entry.Title,
		Placement:   ScannerIndicatorPlacement(entry.Placement),
		Outputs:     entry.Outputs,
		Strategies:  append([]string{}, usage[entry.ID]...),
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
