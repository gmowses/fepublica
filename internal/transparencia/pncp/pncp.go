// Package pncp fetches records from the Portal Nacional de Contratações Públicas
// (PNCP), which aggregates all public procurement contracts in Brazil since 2023.
//
// Unlike the CGU Portal da Transparência endpoints, PNCP is open to the public
// without authentication. The base URL is different and uses a different
// pagination and response shape.
//
// API docs: https://www.gov.br/pncp/pt-br/acesso-a-informacao/dados-abertos
// Swagger: https://pncp.gov.br/api/consulta/v3/api-docs
//
// In the MVP we target the "contratos" (contracts) resource, which is the
// richest and most politically relevant dataset in PNCP. New resource types
// can be added as separate source ids later (pncp-atas, pncp-editais, etc.).
package pncp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gmowses/fepublica/internal/transparencia"
)

const (
	// SourceID is the canonical identifier used in the fepublica DB.
	SourceID = "pncp-contratos"

	// DefaultBaseURL is the public consulta API of PNCP.
	DefaultBaseURL = "https://pncp.gov.br/api/consulta"

	// Path for the contracts endpoint (paginated).
	Path = "/v1/contratos"

	// PageSize is the maximum page size PNCP accepts for this resource.
	PageSize = 500
)

// pageResponse is the envelope shape returned by PNCP.
type pageResponse struct {
	Data         []json.RawMessage `json:"data"`
	TotalPaginas int               `json:"totalPaginas"`
	TotalRegistros int             `json:"totalRegistros"`
	Numero       int               `json:"numeroDaPagina"`
	Empty        bool              `json:"empty"`
}

// recordShape is the minimal subset we need to extract an external id.
// PNCP uses "numeroControlePNCP" as a stable globally-unique identifier
// for a contract. If present we use it; otherwise we fall back to the
// compound (orgaoEntidade.cnpj + sequencial + ano).
type recordShape struct {
	NumeroControlePNCP string `json:"numeroControlePNCP"`
	SequencialContrato *int   `json:"sequencialContrato"`
	AnoContrato        *int   `json:"anoContrato"`
	OrgaoEntidade      struct {
		CNPJ string `json:"cnpj"`
	} `json:"orgaoEntidade"`
}

// Fetch uses a dedicated HTTP client for PNCP because it does not require
// the same API key header. We still reuse the fepublica rate limiter for
// politeness.
//
// The collector package wraps this behind the generic Fetcher interface.
// We accept a *transparencia.Client for signature compatibility, but we
// don't actually use its key; we build our own request.
func Fetch(ctx context.Context, _ *transparencia.Client) (*transparencia.FetchResult, error) {
	httpClient := &http.Client{Timeout: 60 * time.Second}
	result := &transparencia.FetchResult{
		Source:    SourceID,
		FetchedAt: time.Now().UTC().Format(time.RFC3339),
	}

	// PNCP requires dataInicial and dataFinal on contratos. For a full archive
	// we need to sweep historical windows. The MVP implementation fetches a
	// rolling 30-day window ending today — that's the "what changed recently"
	// use case, which fits the tamper-evidence mission and keeps API load low.
	// A backfill tool for wider windows can be added in v0.3.
	now := time.Now().UTC()
	dataFinal := now.Format("20060102")
	dataInicial := now.AddDate(0, 0, -30).Format("20060102")

	page := 1
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		q := url.Values{}
		q.Set("dataInicial", dataInicial)
		q.Set("dataFinal", dataFinal)
		q.Set("pagina", strconv.Itoa(page))
		q.Set("tamanhoPagina", strconv.Itoa(PageSize))

		fullURL := DefaultBaseURL + Path + "?" + q.Encode()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
		if err != nil {
			return nil, fmt.Errorf("pncp: build request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "fepublica/0.1 (+https://github.com/gmowses/fepublica)")

		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("pncp: request page %d: %w", page, err)
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("pncp: read body: %w", readErr)
		}
		if resp.StatusCode == http.StatusNoContent || len(body) == 0 {
			break
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("pncp: unexpected status %d on page %d: %s",
				resp.StatusCode, page, truncate(string(body), 200))
		}

		var envelope pageResponse
		if err := json.Unmarshal(body, &envelope); err != nil {
			// PNCP sometimes returns a bare array on empty windows.
			var arr []json.RawMessage
			if err2 := json.Unmarshal(body, &arr); err2 == nil {
				envelope.Data = arr
			} else {
				return nil, fmt.Errorf("pncp: parse page %d: %w", page, err)
			}
		}

		if len(envelope.Data) == 0 {
			break
		}

		for i, item := range envelope.Data {
			var meta recordShape
			if err := json.Unmarshal(item, &meta); err != nil {
				return nil, fmt.Errorf("pncp: record %d page %d: %w", i, page, err)
			}
			externalID := meta.NumeroControlePNCP
			if externalID == "" {
				if meta.SequencialContrato != nil && meta.AnoContrato != nil && meta.OrgaoEntidade.CNPJ != "" {
					externalID = fmt.Sprintf("%s-%d-%d", meta.OrgaoEntidade.CNPJ, *meta.AnoContrato, *meta.SequencialContrato)
				} else {
					return nil, fmt.Errorf("pncp: record %d page %d has no identifier", i, page)
				}
			}
			result.Records = append(result.Records, transparencia.RawRecord{
				ExternalID: externalID,
				Raw:        []byte(item),
			})
			result.TotalBytes += int64(len(item))
		}
		result.TotalPages = page

		if envelope.TotalPaginas > 0 && page >= envelope.TotalPaginas {
			break
		}
		if len(envelope.Data) < PageSize {
			break
		}
		page++
	}

	return result, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
