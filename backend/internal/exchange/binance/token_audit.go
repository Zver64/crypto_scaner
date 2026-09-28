package binance

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"time"

	"crypto-scanner/internal/tokensecurity"

	"golang.org/x/time/rate"
)

var _ tokensecurity.Auditor = (*TokenAuditor)(nil)

// tokenAuditChainIDs are Binance Web3 chain IDs of audited chains.
var tokenAuditChainIDs = map[tokensecurity.Chain]string{
	tokensecurity.ChainBSC:      "56",
	tokensecurity.ChainEthereum: "1",
	tokensecurity.ChainBase:     "8453",
	tokensecurity.ChainSolana:   "CT_501",
}

// TokenAuditor reads Binance Web3 token security audits, the data behind the
// Audit tab of the Binance app. The endpoint is public but has no SLA.
type TokenAuditor struct {
	url     string
	http    *http.Client
	limiter *rate.Limiter
}

// NewTokenAuditor creates an auditor; an empty base selects the Binance Web3 host.
func NewTokenAuditor(base string) *TokenAuditor {
	if base == "" {
		base = "https://web3.binance.com"
	}
	// The endpoint publishes no quota; audits are a slow background refresh.
	limiter := rate.NewLimiter(rate.Every(500*time.Millisecond), 1)
	return &TokenAuditor{url: base + "/bapi/defi/v1/public/wallet-direct/security/token/audit", http: &http.Client{Timeout: 15 * time.Second}, limiter: limiter}
}

type tokenAuditResponse struct {
	Code    string `json:"code"`
	Success bool   `json:"success"`
	Data    *struct {
		HasResult   bool `json:"hasResult"`
		IsSupported bool `json:"isSupported"`
		RiskItems   []struct {
			Details []struct {
				Title       string `json:"title"`
				Description string `json:"description"`
				IsHit       bool   `json:"isHit"`
				Order       int    `json:"order"`
				RiskType    string `json:"riskType"`
			} `json:"details"`
		} `json:"riskItems"`
	} `json:"data"`
}

func (auditor *TokenAuditor) Audit(ctx context.Context, chain tokensecurity.Chain, address string) (tokensecurity.Report, error) {
	chainID, ok := tokenAuditChainIDs[chain]
	if !ok {
		return tokensecurity.Report{}, fmt.Errorf("unsupported token audit chain %q", chain)
	}
	body, err := json.Marshal(map[string]string{"binanceChainId": chainID, "contractAddress": address, "requestId": requestID()})
	if err != nil {
		return tokensecurity.Report{}, err
	}
	if err := auditor.limiter.Wait(ctx); err != nil {
		return tokensecurity.Report{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, auditor.url, bytes.NewReader(body))
	if err != nil {
		return tokensecurity.Report{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept-Encoding", "identity")
	request.Header.Set("User-Agent", "binance-web3/1.4 (Skill)")
	request.Header.Set("source", "agent")
	response, err := auditor.http.Do(request)
	if err != nil {
		return tokensecurity.Report{}, err
	}
	defer response.Body.Close()
	if response.StatusCode/100 != 2 {
		return tokensecurity.Report{}, fmt.Errorf("Binance token audit status %d", response.StatusCode)
	}
	var decoded tokenAuditResponse
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return tokensecurity.Report{}, fmt.Errorf("decode Binance token audit: %w", err)
	}
	if !decoded.Success || decoded.Data == nil {
		return tokensecurity.Report{}, fmt.Errorf("Binance token audit failed with code %q", decoded.Code)
	}
	if !decoded.Data.IsSupported {
		return tokensecurity.Report{}, nil
	}
	if !decoded.Data.HasResult {
		return tokensecurity.Report{}, tokensecurity.ErrNoResult
	}
	type orderedFinding struct {
		order int
		tokensecurity.Finding
	}
	var hits []orderedFinding
	for _, item := range decoded.Data.RiskItems {
		for _, detail := range item.Details {
			if !detail.IsHit {
				continue
			}
			severity := tokensecurity.SeverityCaution
			if detail.RiskType == "RISK" {
				severity = tokensecurity.SeverityRisk
			}
			hits = append(hits, orderedFinding{order: detail.Order, Finding: tokensecurity.Finding{Severity: severity, Title: detail.Title, Description: detail.Description}})
		}
	}
	slices.SortStableFunc(hits, func(a, b orderedFinding) int { return a.order - b.order })
	report := tokensecurity.Report{Supported: true, Findings: make([]tokensecurity.Finding, len(hits))}
	for i, hit := range hits {
		report.Findings[i] = hit.Finding
	}
	return report, nil
}

// requestID returns a random UUID v4.
func requestID() string {
	var b [16]byte
	_, _ = rand.Read(b[:]) // never fails since Go 1.24
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
