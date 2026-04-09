package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gmowses/fepublica/internal/store"
)

type enteView struct {
	ID         string `json:"id"`
	Nome       string `json:"nome"`
	NomeCurto  string `json:"nome_curto,omitempty"`
	Esfera     string `json:"esfera"`
	Tipo       string `json:"tipo"`
	Poder      string `json:"poder,omitempty"`
	UF         string `json:"uf,omitempty"`
	IBGECode   string `json:"ibge_code,omitempty"`
	CNPJ       string `json:"cnpj,omitempty"`
	Populacao  int    `json:"populacao,omitempty"`
	DomainHint string `json:"domain_hint,omitempty"`
	ParentID   string `json:"parent_id,omitempty"`
	Tier       int    `json:"tier"`
	Active     bool   `json:"active"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

func enteDTO(e *store.Ente) enteView {
	return enteView{
		ID:         e.ID,
		Nome:       e.Nome,
		NomeCurto:  e.NomeCurto,
		Esfera:     e.Esfera,
		Tipo:       e.Tipo,
		Poder:      e.Poder,
		UF:         e.UF,
		IBGECode:   e.IBGECode,
		CNPJ:       e.CNPJ,
		Populacao:  e.Populacao,
		DomainHint: e.DomainHint,
		ParentID:   e.ParentID,
		Tier:       e.Tier,
		Active:     e.Active,
		CreatedAt:  e.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:  e.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// handleListEntes returns a paginated list of entes with filters.
func (s *Server) handleListEntes(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	params := store.ListEntesParams{
		Esfera: q.Get("esfera"),
		UF:     q.Get("uf"),
		Search: q.Get("q"),
	}
	if v := q.Get("tier"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			params.Tier = n
		}
	}
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1000 {
			params.Limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			params.Offset = n
		}
	}

	rows, total, err := s.store.ListEntes(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]enteView, len(rows))
	for i := range rows {
		out[i] = enteDTO(&rows[i])
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"total":  total,
		"limit":  params.Limit,
		"offset": params.Offset,
		"entes":  out,
	})
}

// handleGetEnte returns a single ente by id.
func (s *Server) handleGetEnte(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, http.ErrAbortHandler)
		return
	}
	e, err := s.store.GetEnte(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, enteDTO(e))
}
