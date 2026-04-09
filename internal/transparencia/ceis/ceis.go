// Package ceis fetches records from the CEIS (Cadastro de Empresas Inidôneas e Suspensas)
// endpoint of the Portal da Transparência.
//
// The CEIS lists companies barred from contracting with the Brazilian government.
// Public by Lei 12.846/2013 (Lei Anticorrupção) and Decreto 8.777/2016.
//
// Endpoint: https://api.portaldatransparencia.gov.br/api-de-dados/ceis
package ceis

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/gmowses/fepublica/internal/transparencia"
)

const (
	// SourceID is the canonical identifier used in the fepublica DB.
	SourceID = "ceis"

	// Path is the API path (appended to the client base URL).
	Path = "/api-de-dados/ceis"
)

// recordShape is the minimal subset of the CEIS response needed to extract
// an external ID. Every other field is preserved as raw JSON for hashing.
type recordShape struct {
	ID int64 `json:"id"`
}

// Fetch performs a full paginated scrape of the CEIS endpoint, stopping when
// an empty page is returned. Returns one FetchResult for the entire crawl.
func Fetch(ctx context.Context, client *transparencia.Client) (*transparencia.FetchResult, error) {
	result := &transparencia.FetchResult{
		Source:    SourceID,
		FetchedAt: time.Now().UTC().Format(time.RFC3339),
	}

	page := 1
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		q := url.Values{}
		q.Set("pagina", strconv.Itoa(page))

		var raw json.RawMessage
		if err := client.Get(ctx, Path, q, &raw); err != nil {
			return nil, fmt.Errorf("ceis: page %d: %w", page, err)
		}

		records, size, err := parsePage(raw)
		if err != nil {
			return nil, fmt.Errorf("ceis: parse page %d: %w", page, err)
		}

		if len(records) == 0 {
			break
		}

		result.Records = append(result.Records, records...)
		result.TotalBytes += size
		result.TotalPages = page
		page++
	}

	return result, nil
}

// parsePage unmarshals a single API response into RawRecord slices.
func parsePage(raw json.RawMessage) ([]transparencia.RawRecord, int64, error) {
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, 0, fmt.Errorf("expected array of records: %w", err)
	}
	out := make([]transparencia.RawRecord, 0, len(arr))
	var total int64
	for i, item := range arr {
		var meta recordShape
		if err := json.Unmarshal(item, &meta); err != nil {
			return nil, 0, fmt.Errorf("record %d: %w", i, err)
		}
		if meta.ID == 0 {
			return nil, 0, fmt.Errorf("record %d: missing id", i)
		}
		out = append(out, transparencia.RawRecord{
			ExternalID: strconv.FormatInt(meta.ID, 10),
			Raw:        []byte(item),
		})
		total += int64(len(item))
	}
	return out, total, nil
}
