package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	generated "crypto-scanner/internal/storage/postgres/sqlc"
	"crypto-scanner/internal/tokensecurity"

	"github.com/jackc/pgx/v5/pgtype"
)

var _ tokensecurity.Store = (*Store)(nil)

// storedFinding is the JSON form of a finding in token_security_audits.issues.
type storedFinding struct {
	Severity    tokensecurity.Severity `json:"severity"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
}

func (store *Store) ListAuditedCoinIDs(ctx context.Context) ([]string, error) {
	return store.queries.ListAuditedCoinIDs(ctx)
}

// ListTokenAudits returns the audited contracts and times without findings.
func (store *Store) ListTokenAudits(ctx context.Context) ([]tokensecurity.Audit, error) {
	rows, err := store.queries.ListTokenAudits(ctx)
	if err != nil {
		return nil, err
	}
	audits := make([]tokensecurity.Audit, len(rows))
	for i, row := range rows {
		audits[i] = tokensecurity.Audit{Contract: tokensecurity.Contract{CoinID: row.CoinID, Chain: tokensecurity.Chain(row.Chain), Address: row.ContractAddress}, AuditedAt: row.AuditedAt.Time}
	}
	return audits, nil
}

func (store *Store) SaveTokenAudit(ctx context.Context, audit tokensecurity.Audit) error {
	findings := make([]storedFinding, len(audit.Findings))
	for i, finding := range audit.Findings {
		findings[i] = storedFinding(finding)
	}
	issues, err := json.Marshal(findings)
	if err != nil {
		return err
	}
	return store.queries.UpsertTokenAudit(ctx, generated.UpsertTokenAuditParams{
		CoinID: audit.CoinID, Chain: string(audit.Chain), ContractAddress: audit.Address,
		Supported: audit.Supported, Issues: issues, AuditedAt: pgtype.Timestamptz{Time: audit.AuditedAt, Valid: true},
	})
}

func (store *Store) DeleteTokenAuditsExcept(ctx context.Context, contracts []tokensecurity.Contract) error {
	params := generated.DeleteTokenAuditsExceptParams{CoinIds: make([]string, len(contracts)), Chains: make([]string, len(contracts)), Addresses: make([]string, len(contracts))}
	for i, contract := range contracts {
		params.CoinIds[i], params.Chains[i], params.Addresses[i] = contract.CoinID, string(contract.Chain), contract.Address
	}
	return store.queries.DeleteTokenAuditsExcept(ctx, params)
}

func (store *Store) ListTokenSecurityIssues(ctx context.Context, baseAssets []string) (map[string][]tokensecurity.Issue, error) {
	rows, err := store.queries.ListTokenSecurityIssues(ctx, baseAssets)
	if err != nil {
		return nil, err
	}
	issues := make(map[string][]tokensecurity.Issue)
	for _, row := range rows {
		var findings []storedFinding
		if err := json.Unmarshal(row.Issues, &findings); err != nil {
			return nil, fmt.Errorf("decode token audit issues of %s: %w", row.BaseAsset, err)
		}
		for _, finding := range findings {
			issues[row.BaseAsset] = append(issues[row.BaseAsset], tokensecurity.Issue{Chain: tokensecurity.Chain(row.Chain), Finding: tokensecurity.Finding(finding)})
		}
	}
	return issues, nil
}
