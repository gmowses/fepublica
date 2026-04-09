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
	"os"
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

	// PageSize balances "few enough to avoid timeouts" with "enough to cover
	// typical volume". PNCP rejects values below 10.
	PageSize = 50

	// WindowDays is the default rolling window used when no explicit range
	// is provided by the caller. Narrow enough to fit a single paged fetch
	// without timing out the slow PNCP backend.
	WindowDays = 7
)

// pageResponse is the envelope shape returned by PNCP.
type pageResponse struct {
	Data             []json.RawMessage `json:"data"`
	TotalPaginas     int               `json:"totalPaginas"`
	TotalRegistros   int               `json:"totalRegistros"`
	NumeroPagina     int               `json:"numeroPagina"`
	PaginasRestantes int               `json:"paginasRestantes"`
	Empty            bool              `json:"empty"`
}

// cap on the number of pages we walk in a single run, to avoid runaway
// crawls on exceptionally busy days or upstream bugs.
const maxPages = 200

// recordShape is the minimal subset we need to extract an external id.
// PNCP uses "numeroControlePncpCompra" as the stable identifier for a
// contract in the consulta v1 API (format: "<cnpj>-<seq>-<num>/<ano>").
type recordShape struct {
	NumeroControlePncpCompra string `json:"numeroControlePncpCompra"`
	NiFornecedor             string `json:"niFornecedor"`
	DataAssinatura           string `json:"dataAssinatura"`
	OrgaoEntidade            struct {
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
//
// The PNCP consulta API is noticeably slow and occasionally times out on
// large page sizes. We use a conservative page size and a long per-request
// timeout so individual requests can wait, while the outer context from
// the caller still bounds the total run.
func Fetch(ctx context.Context, _ *transparencia.Client) (*transparencia.FetchResult, error) {
	httpClient := &http.Client{Timeout: 5 * time.Minute}
	result := &transparencia.FetchResult{
		Source:    SourceID,
		FetchedAt: time.Now().UTC().Format(time.RFC3339),
	}

	// PNCP requires dataInicial and dataFinal on contratos. The public
	// consulta API is historically slow and often 500s on wider windows,
	// so we use a narrow default (1 day back from today). Run it multiple
	// days per week via the scheduler to accumulate history.
	//
	// For backfills or testing, the window can be overridden with the
	// PNCP_FROM and PNCP_TO env vars (YYYYMMDD format), e.g. to seed
	// contratos data from a known-good historical window.
	now := time.Now().UTC()
	dataFinal := now.Format("20060102")
	dataInicial := now.AddDate(0, 0, -1).Format("20060102")
	if v := os.Getenv("PNCP_FROM"); v != "" {
		dataInicial = v
	}
	if v := os.Getenv("PNCP_TO"); v != "" {
		dataFinal = v
	}
	_ = WindowDays // reserved for a future explicit-window caller

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

		body, status, err := doWithRetry(ctx, httpClient, fullURL)
		if err != nil {
			return nil, fmt.Errorf("pncp: request page %d: %w", page, err)
		}
		if status == http.StatusNoContent || len(body) == 0 {
			break
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf("pncp: unexpected status %d on page %d: %s",
				status, page, truncate(string(body), 200))
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
			externalID := meta.NumeroControlePncpCompra
			if externalID == "" {
				// Fallback: compose from orgao + supplier + signature date.
				if meta.OrgaoEntidade.CNPJ != "" && meta.NiFornecedor != "" && meta.DataAssinatura != "" {
					externalID = fmt.Sprintf("%s-%s-%s", meta.OrgaoEntidade.CNPJ, meta.NiFornecedor, meta.DataAssinatura)
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

		if envelope.Empty {
			break
		}
		if envelope.TotalPaginas > 0 && page >= envelope.TotalPaginas {
			break
		}
		if envelope.PaginasRestantes == 0 {
			break
		}
		if len(envelope.Data) < PageSize {
			break
		}
		if page >= maxPages {
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

// doWithRetry performs a GET with exponential backoff on transient failures.
// PNCP's consulta API is historically flaky and frequently returns HTTP 500
// "Erro na comunicação com o banco de dados" under load. Retrying with a
// generous delay usually recovers.
func doWithRetry(ctx context.Context, client *http.Client, url string) ([]byte, int, error) {
	const maxAttempts = 5
	backoff := 5 * time.Second

	var lastErr error
	var lastStatus int
	var lastBody []byte

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			select {
			case <-ctx.Done():
				return nil, 0, ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > 60*time.Second {
				backoff = 60 * time.Second
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, 0, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "fepublica/0.1 (+https://github.com/gmowses/fepublica)")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		lastBody = body
		lastStatus = resp.StatusCode

		// Retry transient upstream failures.
		if resp.StatusCode >= 500 && resp.StatusCode != http.StatusNotImplemented {
			lastErr = fmt.Errorf("upstream status %d: %s", resp.StatusCode, truncate(string(body), 200))
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			lastErr = fmt.Errorf("upstream 429 rate limited")
			continue
		}

		// Success or non-retryable status — return immediately.
		return body, resp.StatusCode, nil
	}

	if lastStatus != 0 {
		return lastBody, lastStatus, lastErr
	}
	return nil, 0, fmt.Errorf("all %d attempts failed: %w", maxAttempts, lastErr)
}
