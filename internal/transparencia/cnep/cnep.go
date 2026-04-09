// Package cnep fetches records from the CNEP (Cadastro Nacional de Empresas Punidas)
// endpoint of the Portal da Transparência.
//
// The CNEP lists companies penalized under Lei 12.846/2013 (Lei Anticorrupção).
// Schema is a superset of CEIS — adds `valorMulta`. For our purposes the two sources
// are handled identically; we keep separate packages for clarity and ownership.
//
// Endpoint: https://api.portaldatransparencia.gov.br/api-de-dados/cnep
package cnep

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
	SourceID = "cnep"
	Path     = "/api-de-dados/cnep"
)

type recordShape struct {
	ID int64 `json:"id"`
}

// Fetch performs a full paginated scrape of the CNEP endpoint.
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
			return nil, fmt.Errorf("cnep: page %d: %w", page, err)
		}

		records, size, err := parsePage(raw)
		if err != nil {
			return nil, fmt.Errorf("cnep: parse page %d: %w", page, err)
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

func parsePage(raw json.RawMessage) ([]transparencia.RawRecord, int64, error) {
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, 0, fmt.Errorf("expected array: %w", err)
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
