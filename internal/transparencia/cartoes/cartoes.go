// Package cartoes fetches transactions from the CPGF (Cartão de Pagamento do
// Governo Federal) endpoint of the Portal da Transparência.
//
// CPGF is the federal government's corporate card. Every transaction is
// public by Lei 12.527/2011 (LAI). The data is politically hot — abuses
// of corporate cards have been the trigger for multiple national scandals.
//
// Endpoint: https://api.portaldatransparencia.gov.br/api-de-dados/cartoes
//
// The API requires a month range (mesExtratoInicio/mesExtratoFim, MM/YYYY).
// We fetch one calendar month per call to keep responses small.
package cartoes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/gmowses/fepublica/internal/transparencia"
)

const (
	// SourceID is the canonical identifier used in the fepublica DB.
	SourceID = "cartoes-cpgf"

	// Path is the API path appended to the client base URL.
	Path = "/api-de-dados/cartoes"
)

// recordShape captures the minimum needed to derive an external_id.
type recordShape struct {
	ID int64 `json:"id"`
}

// Fetch performs a paginated scrape of CPGF transactions for the configured
// month range. The window can be overridden via CARTOES_MES env var
// (format MM/YYYY); when unset we fetch the previous calendar month.
func Fetch(ctx context.Context, client *transparencia.Client) (*transparencia.FetchResult, error) {
	result := &transparencia.FetchResult{
		Source:    SourceID,
		FetchedAt: time.Now().UTC().Format(time.RFC3339),
	}

	mes := os.Getenv("CARTOES_MES")
	if mes == "" {
		// Default: previous month, since the current month is rarely complete
		// in upstream by the first few days.
		prev := time.Now().UTC().AddDate(0, -1, 0)
		mes = prev.Format("01/2006")
	}

	page := 1
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		q := url.Values{}
		q.Set("mesExtratoInicio", mes)
		q.Set("mesExtratoFim", mes)
		q.Set("pagina", strconv.Itoa(page))

		var raw json.RawMessage
		if err := client.Get(ctx, Path, q, &raw); err != nil {
			return nil, fmt.Errorf("cartoes: page %d (%s): %w", page, mes, err)
		}

		records, size, err := parsePage(raw)
		if err != nil {
			return nil, fmt.Errorf("cartoes: parse page %d: %w", page, err)
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
