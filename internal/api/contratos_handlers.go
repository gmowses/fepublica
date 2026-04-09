package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gmowses/fepublica/internal/store"
)

// contratoView is the JSON shape served by the contratos endpoints.
type contratoView struct {
	ID                    int64    `json:"id"`
	ExternalID            string   `json:"external_id"`
	OrgaoCNPJ             string   `json:"orgao_cnpj,omitempty"`
	OrgaoRazaoSocial      string   `json:"orgao_razao_social,omitempty"`
	UF                    string   `json:"uf,omitempty"`
	FornecedorNI          string   `json:"fornecedor_ni,omitempty"`
	FornecedorNome        string   `json:"fornecedor_nome,omitempty"`
	ValorGlobal           *float64 `json:"valor_global,omitempty"`
	DataAssinatura        string   `json:"data_assinatura,omitempty"`
	ObjetoContrato        string   `json:"objeto_contrato,omitempty"`
	FornecedorSancionado  bool     `json:"fornecedor_sancionado"`
}

func contratoDTO(c *store.ContratoSummary) contratoView {
	v := contratoView{
		ID:                   c.ID,
		ExternalID:           c.ExternalID,
		OrgaoCNPJ:            c.OrgaoCNPJ,
		OrgaoRazaoSocial:     c.OrgaoRazaoSocial,
		UF:                   c.UF,
		FornecedorNI:         c.FornecedorNI,
		FornecedorNome:       c.FornecedorNome,
		ValorGlobal:          c.ValorGlobal,
		ObjetoContrato:       c.ObjetoContrato,
		FornecedorSancionado: c.FornecedorSancionado,
	}
	if c.DataAssinatura != nil {
		v.DataAssinatura = c.DataAssinatura.Format("2006-01-02")
	}
	return v
}

// handleGastosStats returns the hero numbers for /gastos.
func (s *Server) handleGastosStats(w http.ResponseWriter, r *http.Request) {
	st, err := s.store.GetGastosStats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// handleListContratos returns a paginated list of contratos with filters.
func (s *Server) handleListContratos(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	params := store.ListContratosParams{
		Search:  q.Get("q"),
		UF:      q.Get("uf"),
		OrderBy: q.Get("order"),
	}
	if v := q.Get("min_valor"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			params.MinValor = n
		}
	}
	if v := q.Get("max_valor"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			params.MaxValor = n
		}
	}
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			params.Limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			params.Offset = n
		}
	}

	rows, total, err := s.store.ListContratos(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]contratoView, len(rows))
	for i := range rows {
		out[i] = contratoDTO(&rows[i])
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"total":     total,
		"limit":     params.Limit,
		"offset":    params.Offset,
		"contratos": out,
	})
}

// handleTopFornecedores returns the top N suppliers.
func (s *Server) handleTopFornecedores(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	rows, err := s.store.TopFornecedores(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"top": rows})
}

// handleTopOrgaos returns the top N buying agencies.
func (s *Server) handleTopOrgaos(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	rows, err := s.store.TopOrgaos(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"top": rows})
}

// handleGetFornecedor returns the aggregated view for a supplier.
func (s *Server) handleGetFornecedor(w http.ResponseWriter, r *http.Request) {
	ni := r.PathValue("ni")
	if ni == "" {
		http.Error(w, "missing ni", http.StatusBadRequest)
		return
	}
	f, err := s.store.GetFornecedor(r.Context(), ni)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	contratos, err := s.store.ListContratosByFornecedor(r.Context(), ni, 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]contratoView, len(contratos))
	for i := range contratos {
		out[i] = contratoDTO(&contratos[i])
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"fornecedor": f,
		"contratos":  out,
	})
}

// handleGastosTimeseries returns daily sums of valor_global for the last
// 90 days. Used for the timeline chart on /gastos.
func (s *Server) handleGastosTimeseries(w http.ResponseWriter, r *http.Request) {
	rows, err := s.store.GastosTimeseries(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"series": rows})
}

// (Placeholder DTO for time; actual time series endpoint defined above)
var _ = time.Time{}
