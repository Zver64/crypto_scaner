// Package tokensecurity keeps on-chain token security audits of the coins
// behind active instruments and reports the checks that found a problem.
package tokensecurity

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"crypto-scanner/internal/platform/backoff"
)

// Chain is an audited blockchain. Adapters translate it to their own IDs.
type Chain string

const (
	ChainBSC      Chain = "bsc"
	ChainEthereum Chain = "ethereum"
	ChainBase     Chain = "base"
	ChainSolana   Chain = "solana"
)

// Severity follows the audit provider's split into risks and cautions.
type Severity string

const (
	SeverityRisk    Severity = "risk"
	SeverityCaution Severity = "caution"
)

// Finding is one failed audit check.
type Finding struct {
	Severity           Severity
	Title, Description string
}

// Report is the outcome of one contract audit. Unsupported contracts have no
// findings, which does not mean they are safe.
type Report struct {
	Supported bool
	Findings  []Finding
}

// Issue is a finding of one coin on one chain.
type Issue struct {
	Chain Chain
	Finding
}

// Contract is a coin's token contract on one supported chain.
type Contract struct {
	CoinID  string
	Chain   Chain
	Address string
}

// Audit is a persisted contract audit.
type Audit struct {
	Contract
	Report
	AuditedAt time.Time
}

// Progress reports the current or last audit pass.
type Progress struct {
	Running bool
	// Failed means the last pass stopped with an error and will be retried.
	Failed bool
	// Completed counts audited or failed contracts of the Total due in the pass.
	Completed, Total int
}

// ErrNoResult means the provider has no audit of a contract yet. It is not a
// provider failure: the contract is retried later and its last audit is kept.
var ErrNoResult = errors.New("token audit has no result")

// errNoCoins means coin mappings are not available yet.
var errNoCoins = errors.New("no audited coins yet")

// Auditor audits one token contract. It returns ErrNoResult when the
// provider has no audit of the contract yet.
type Auditor interface {
	Audit(context.Context, Chain, string) (Report, error)
}

// ContractSource lists the contracts of coins on supported chains, keyed by coin ID.
type ContractSource interface {
	Contracts(context.Context) (map[string][]Contract, error)
}

type Store interface {
	// ListAuditedCoinIDs returns the resolved CoinGecko IDs of active instruments.
	ListAuditedCoinIDs(context.Context) ([]string, error)
	ListTokenAudits(context.Context) ([]Audit, error)
	SaveTokenAudit(context.Context, Audit) error
	// DeleteTokenAuditsExcept removes audits of contracts absent from contracts,
	// including audits of a coin's previous address on a chain.
	DeleteTokenAuditsExcept(context.Context, []Contract) error
	// ListTokenSecurityIssues returns the findings of audited coins keyed by base asset.
	ListTokenSecurityIssues(context.Context, []string) (map[string][]Issue, error)
}

const (
	// auditTTL is how long an audit stays current.
	auditTTL = 24 * time.Hour
	// checkInterval spaces passes that audit new and expired contracts.
	checkInterval = time.Hour
	// retryDelay spaces passes after the contract list could not be loaded.
	retryDelay = 10 * time.Minute
	// maxConsecutiveFailures ends a pass when the audit provider keeps failing.
	maxConsecutiveFailures = 5
	// coinsWaitDelay spaces passes until coin mappings are available.
	coinsWaitDelay = time.Minute
)

// Service audits contracts in the background and serves their issues.
type Service struct {
	store     Store
	contracts ContractSource
	auditor   Auditor
	logger    *slog.Logger
	now       func() time.Time
	// known caches the contract list for auditTTL; only Run touches it.
	known          map[string][]Contract
	knownFetchedAt time.Time
	mu             sync.Mutex
	progress       Progress
}

func New(store Store, contracts ContractSource, auditor Auditor, logger *slog.Logger) *Service {
	return &Service{store: store, contracts: contracts, auditor: auditor, logger: logger.With("module", "token_security"), now: time.Now}
}

func (s *Service) Run(ctx context.Context) error {
	for {
		delay := checkInterval
		err := s.refresh(ctx)
		if ctx.Err() != nil {
			return nil
		}
		if errors.Is(err, errNoCoins) {
			if backoff.Sleep(ctx, coinsWaitDelay) != nil {
				return nil
			}
			continue
		}
		s.updateProgress(func(p *Progress) { p.Running, p.Failed = false, err != nil })
		if err != nil {
			s.logger.WarnContext(ctx, "token security refresh failed", "error", err)
			delay = retryDelay
		}
		if backoff.Sleep(ctx, delay) != nil {
			return nil
		}
	}
}

// Progress returns the state of the current or last audit pass.
func (s *Service) Progress() Progress {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.progress
}

func (s *Service) updateProgress(update func(*Progress)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	update(&s.progress)
}

// Issues returns the findings of the base assets that have any. It never
// fails: when audits cannot be read, no issues are reported.
func (s *Service) Issues(ctx context.Context, baseAssets []string) map[string][]Issue {
	if len(baseAssets) == 0 {
		return map[string][]Issue{}
	}
	issues, err := s.store.ListTokenSecurityIssues(ctx, baseAssets)
	if err != nil {
		s.logger.WarnContext(ctx, "list token security issues failed", "error", err)
		return map[string][]Issue{}
	}
	return issues
}

// refresh audits every new, moved, or expired contract of active coins. Failed
// audits are retried on the next pass.
func (s *Service) refresh(ctx context.Context) error {
	coinIDs, err := s.store.ListAuditedCoinIDs(ctx)
	if err != nil {
		return fmt.Errorf("list audited coins: %w", err)
	}
	if len(coinIDs) == 0 {
		return errNoCoins
	}
	if s.known == nil || s.now().Sub(s.knownFetchedAt) >= auditTTL {
		known, err := s.contracts.Contracts(ctx)
		if err != nil {
			return fmt.Errorf("list token contracts: %w", err)
		}
		s.known, s.knownFetchedAt = known, s.now()
	}
	var contracts []Contract
	for _, id := range coinIDs {
		contracts = append(contracts, s.known[id]...)
	}
	if err := s.store.DeleteTokenAuditsExcept(ctx, contracts); err != nil {
		return fmt.Errorf("delete stale token audits: %w", err)
	}
	stored, err := s.store.ListTokenAudits(ctx)
	if err != nil {
		return fmt.Errorf("list token audits: %w", err)
	}
	current := make(map[Contract]bool, len(stored))
	for _, audit := range stored {
		if s.now().Sub(audit.AuditedAt) < auditTTL {
			current[audit.Contract] = true
		}
	}
	due := make([]Contract, 0, len(contracts))
	for _, contract := range contracts {
		if !current[contract] {
			due = append(due, contract)
		}
	}
	if len(due) == 0 {
		return nil
	}
	s.updateProgress(func(p *Progress) { *p = Progress{Running: true, Total: len(due)} })
	audited, pending, failed, consecutiveFailures := 0, 0, 0, 0
	for _, contract := range due {
		report, err := s.auditor.Audit(ctx, contract.Chain, contract.Address)
		s.updateProgress(func(p *Progress) { p.Completed++ })
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errors.Is(err, ErrNoResult) {
			pending++
			continue
		}
		if err != nil {
			failed++
			consecutiveFailures++
			s.logger.DebugContext(ctx, "token audit failed", "coin_id", contract.CoinID, "chain", contract.Chain, "error", err)
			// A failing provider is retried later instead of per contract.
			if consecutiveFailures == maxConsecutiveFailures {
				return fmt.Errorf("audit %s on %s: %w", contract.CoinID, contract.Chain, err)
			}
			continue
		}
		consecutiveFailures = 0
		if err := s.store.SaveTokenAudit(ctx, Audit{Contract: contract, Report: report, AuditedAt: s.now()}); err != nil {
			return fmt.Errorf("save token audit: %w", err)
		}
		audited++
	}
	s.logger.InfoContext(ctx, "token security audits refreshed", "audited", audited, "pending", pending, "failed", failed)
	return nil
}
