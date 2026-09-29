package markettable

import (
	"context"

	"crypto-scanner/internal/analysis"
)

// Result is a market analysis together with the table built from its items.
type Result struct {
	Search analysis.SearchResult
	Table  Table
}

type Analyzer interface {
	Search(context.Context, analysis.SearchRequest) (analysis.SearchResult, error)
}

// Service runs market searches and presents the matched instruments as a table.
type Service struct {
	analyzer Analyzer
	catalog  Catalog
}

func NewService(analyzer Analyzer, catalog Catalog) *Service {
	return &Service{analyzer: analyzer, catalog: catalog}
}

func (service *Service) Search(ctx context.Context, request analysis.SearchRequest) (Result, error) {
	search, err := service.analyzer.Search(ctx, request)
	if err != nil {
		return Result{}, err
	}
	rows := make([]Row, len(search.Items))
	for i, item := range search.Items {
		rows[i] = RowFromSearchItem(item)
	}
	return Result{Search: search, Table: service.catalog.Build(rows)}, nil
}
