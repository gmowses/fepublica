package api

import (
	"net/http"
	"strconv"

	"github.com/gmowses/fepublica/internal/store"
)

// cartaoView is the JSON shape served by the cartoes endpoints.
type cartaoView struct {
	ID             int64    `json:"id"`
	ExternalID     string   `json:"external_id"`
	DataTransacao  string   `json:"data_transacao,omitempty"`
	ValorTransacao *float64 `json:"valor_transacao,omitempty"`
	EstabNome      string   `json:"estab_nome,omitempty"`
	EstabCNPJ      string   `json:"estab_cnpj,omitempty"`
	PortadorNome   string   `json:"portador_nome,omitempty"`
	PortadorCPF    string   `json:"portador_cpf,omitempty"`
	OrgaoMaxNome   string   `json:"orgao_max_nome,omitempty"`
	OrgaoMaxSigla  string   `json:"orgao_max_sigla,omitempty"`
	UnidadeNome    string   `json:"unidade_nome,omitempty"`
}

func cartaoDTO(c *store.CartaoSummary) cartaoView {
	v := cartaoView{
		ID:             c.ID,
		ExternalID:     c.ExternalID,
		ValorTransacao: c.ValorTransacao,
		EstabNome:      c.EstabNome,
		EstabCNPJ:      c.EstabCNPJ,
		PortadorNome:   c.PortadorNome,
		PortadorCPF:    c.PortadorCPF,
		OrgaoMaxNome:   c.OrgaoMaxNome,
		OrgaoMaxSigla:  c.OrgaoMaxSigla,
		UnidadeNome:    c.UnidadeNome,
	}
	if c.DataTransacao != nil {
		v.DataTransacao = c.DataTransacao.Format("2006-01-02")
	}
	return v
}

func (s *Server) handleCartoesStats(w http.ResponseWriter, r *http.Request) {
	st, err := s.store.GetCartoesStats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleListCartoes(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	rows, total, err := s.store.ListCartoes(r.Context(), q, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]cartaoView, 0, len(rows))
	for i := range rows {
		out = append(out, cartaoDTO(&rows[i]))
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"total":    total,
		"cartoes":  out,
	})
}

func (s *Server) handleTopPortadores(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	rows, err := s.store.TopPortadores(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"top": rows})
}

func (s *Server) handleTopOrgaosCartoes(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	rows, err := s.store.TopOrgaosCartoes(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"top": rows})
}
