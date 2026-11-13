package postgres

import (
	"context"
	"fmt"

	generated "crypto-scanner/internal/storage/postgres/sqlc"
	"crypto-scanner/internal/strategy"
)

// RewriteLegacyStrategies replaces the rules of the strategies stored in the
// syntax before indicator calls with those rewrite returns, in one
// transaction, and marks them rewritten. It goes with the syntax column once
// the rewrite has run in production.
func (store *Store) RewriteLegacyStrategies(ctx context.Context, rewrite func(strategy.Strategy) strategy.Strategy) error {
	tx, err := store.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin strategy rewrite: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	queries := store.queries.WithTx(tx)
	rows, err := queries.ListLegacyStrategyRules(ctx)
	if err != nil {
		return fmt.Errorf("list strategies of the old syntax: %w", err)
	}
	if len(rows) == 0 {
		return nil
	}
	var params generated.RewriteLegacyStrategyRulesParams
	for _, row := range rows {
		item := rewrite(strategy.Strategy{
			ID: row.ID, Name: row.Name, Expression: row.Expression, ExitExpression: row.ExitExpression,
			TakeProfitExpression: row.TakeProfitExpression, StopLossExpression: row.StopLossExpression,
		})
		params.Ids = append(params.Ids, row.ID)
		params.Expressions = append(params.Expressions, item.Expression)
		params.ExitExpressions = append(params.ExitExpressions, item.ExitExpression)
		params.TakeProfitExpressions = append(params.TakeProfitExpressions, item.TakeProfitExpression)
		params.StopLossExpressions = append(params.StopLossExpressions, item.StopLossExpression)
	}
	if err := queries.RewriteLegacyStrategyRules(ctx, params); err != nil {
		return fmt.Errorf("rewrite strategies of the old syntax: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit strategy rewrite: %w", err)
	}
	return nil
}
