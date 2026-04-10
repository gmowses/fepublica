// Package camara fetches data from the Câmara dos Deputados public API
// (dadosabertos.camara.leg.br). The MVP focuses on CEAP — Cota para o
// Exercício da Atividade Parlamentar — the per-deputado expense allowance,
// where every receipt is public including the supplier CNPJ.
//
// Endpoints:
//   GET /api/v2/deputados             list of current legislatura's deputados
//   GET /api/v2/deputados/{id}/despesas?ano=YYYY  CEAP receipts by year
//
// CEAP is politically hot data: cross-referencing supplier CNPJs against
// CEIS/CNEP and against contratos PNCP surfaces "deputado paid empresa
// sancionada", "same supplier dominates many deputados", etc.
package camara

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
	SourceID       = "camara-ceap"
	DefaultBaseURL = "https://dadosabertos.camara.leg.br/api/v2"
)

type pageEnvelope struct {
	Dados []json.RawMessage `json:"dados"`
	Links []struct {
		Rel  string `json:"rel"`
		Href string `json:"href"`
	} `json:"links"`
}

type deputado struct {
	ID    int    `json:"id"`
	Nome  string `json:"nome"`
	Sigla string `json:"siglaPartido"`
	UF    string `json:"siglaUf"`
}

type despesa struct {
	CodDocumento  json.RawMessage `json:"codDocumento"`
	Ano           int             `json:"ano"`
	Mes           int             `json:"mes"`
	CnpjCpf       string          `json:"cnpjCpfFornecedor"`
}

// Package-level deputado metadata maps populated by listDeputados, used
// by fetchDespesasFor to inject deputado info into each despesa record.
var (
	depMetaNome    = map[int]string{}
	depMetaPartido = map[int]string{}
	depMetaUF      = map[int]string{}
)

// Fetch coleta despesas CEAP. Por padrão, ano = ano corrente. Override
// via env var CAMARA_ANO=YYYY.
func Fetch(ctx context.Context, _ *transparencia.Client) (*transparencia.FetchResult, error) {
	httpClient := &http.Client{Timeout: 60 * time.Second}
	result := &transparencia.FetchResult{
		Source:    SourceID,
		FetchedAt: time.Now().UTC().Format(time.RFC3339),
	}

	ano := time.Now().UTC().Year()
	if v := os.Getenv("CAMARA_ANO"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			ano = n
		}
	}

	deputados, err := listDeputados(ctx, httpClient)
	if err != nil {
		return nil, fmt.Errorf("camara: list deputados: %w", err)
	}

	for i, d := range deputados {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		count, bytes, err := fetchDespesasFor(ctx, httpClient, d.ID, ano, result)
		if err != nil {
			// log mas continua — um deputado falhar não para a coleta
			continue
		}
		_ = count
		_ = bytes
		_ = i
	}
	return result, nil
}

func listDeputados(ctx context.Context, c *http.Client) ([]deputado, error) {
	var all []deputado
	page := 1
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		q := url.Values{}
		q.Set("ordem", "ASC")
		q.Set("ordenarPor", "nome")
		q.Set("itens", "100")
		q.Set("pagina", strconv.Itoa(page))
		body, err := getJSON(ctx, c, DefaultBaseURL+"/deputados?"+q.Encode())
		if err != nil {
			return nil, err
		}
		var env pageEnvelope
		if err := json.Unmarshal(body, &env); err != nil {
			return nil, err
		}
		if len(env.Dados) == 0 {
			break
		}
		for _, raw := range env.Dados {
			var d deputado
			if err := json.Unmarshal(raw, &d); err == nil && d.ID > 0 {
				all = append(all, d)
				depMetaNome[d.ID] = d.Nome
				depMetaPartido[d.ID] = d.Sigla
				depMetaUF[d.ID] = d.UF
			}
		}
		// Walk via "next" link if present
		hasNext := false
		for _, l := range env.Links {
			if l.Rel == "next" {
				hasNext = true
				break
			}
		}
		if !hasNext {
			break
		}
		page++
		if page > 20 { // safety cap (513 deputados / 100 = 6 pages)
			break
		}
	}
	return all, nil
}

func fetchDespesasFor(ctx context.Context, c *http.Client, deputadoID, ano int, result *transparencia.FetchResult) (int, int64, error) {
	page := 1
	count := 0
	var bytes int64
	for {
		select {
		case <-ctx.Done():
			return count, bytes, ctx.Err()
		default:
		}
		q := url.Values{}
		q.Set("ano", strconv.Itoa(ano))
		q.Set("itens", "100")
		q.Set("pagina", strconv.Itoa(page))
		path := fmt.Sprintf("%s/deputados/%d/despesas?%s", DefaultBaseURL, deputadoID, q.Encode())
		body, err := getJSON(ctx, c, path)
		if err != nil {
			return count, bytes, err
		}
		var env pageEnvelope
		if err := json.Unmarshal(body, &env); err != nil {
			return count, bytes, err
		}
		if len(env.Dados) == 0 {
			break
		}
		for _, raw := range env.Dados {
			var d despesa
			if err := json.Unmarshal(raw, &d); err != nil {
				continue
			}
			ext := fmt.Sprintf("%d-%s", deputadoID, string(d.CodDocumento))

			// Inject deputado metadata into the canonical JSON so the parser
			// can read it later. We do this by decoding to a generic map,
			// adding the fields, and re-encoding.
			var generic map[string]interface{}
			if err := json.Unmarshal(raw, &generic); err == nil {
				generic["_deputadoId"] = deputadoID
				generic["_deputadoNome"] = depMetaNome[deputadoID]
				generic["_deputadoPartido"] = depMetaPartido[deputadoID]
				generic["_deputadoUf"] = depMetaUF[deputadoID]
				if reraw, err := json.Marshal(generic); err == nil {
					raw = reraw
				}
			}

			result.Records = append(result.Records, transparencia.RawRecord{
				ExternalID: ext,
				Raw:        raw,
			})
			result.TotalBytes += int64(len(raw))
			count++
			bytes += int64(len(raw))
		}
		hasNext := false
		for _, l := range env.Links {
			if l.Rel == "next" {
				hasNext = true
				break
			}
		}
		if !hasNext {
			break
		}
		page++
		if page > 200 {
			break
		}
	}
	result.TotalPages++
	return count, bytes, nil
}

func getJSON(ctx context.Context, c *http.Client, urlStr string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "fepublica/1.0 (+https://github.com/gmowses/fepublica)")
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 50<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("camara: status %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	return body, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
