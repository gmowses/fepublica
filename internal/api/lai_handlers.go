package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gmowses/fepublica/internal/store"
)

// handleListLaiScores returns the top N entes by LAI score.
func (s *Server) handleListLaiScores(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}
	rows, err := s.store.ListLaiScores(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]map[string]any, len(rows))
	for i, r := range rows {
		out[i] = map[string]any{
			"ente_id":         r.EnteID,
			"nome":            r.Nome,
			"uf":              r.UF,
			"esfera":          r.Esfera,
			"score":           r.Score,
			"last_calculated": r.LastCalculated.UTC().Format(time.RFC3339),
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"scores": out})
}

// handleListEnteLaiChecks returns the latest LAI checks for a single ente.
func (s *Server) handleListEnteLaiChecks(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	checks, err := s.store.ListLaiChecksByEnte(r.Context(), id, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]map[string]any, len(checks))
	for i, c := range checks {
		out[i] = laiCheckDTO(&c)
	}
	writeJSON(w, http.StatusOK, map[string]any{"checks": out})
}

func laiCheckDTO(c *store.LaiCheck) map[string]any {
	view := map[string]any{
		"id":              c.ID,
		"ente_id":         c.EnteID,
		"checked_at":      c.CheckedAt.UTC().Format(time.RFC3339),
		"target_url":      c.TargetURL,
		"http_status":     c.HTTPStatus,
		"response_ms":     c.ResponseMS,
		"ssl_valid":       c.SSLValid,
		"portal_loads":    c.PortalLoads,
		"html_size_bytes": c.HTMLSizeBytes,
		"terms_found":     c.TermsFound,
		"required_links":  c.RequiredLinks,
		"errors":          c.Errors,
		"tier_at_check":   c.TierAtCheck,
	}
	if c.SSLExpiresAt != nil {
		view["ssl_expires_at"] = c.SSLExpiresAt.UTC().Format(time.RFC3339)
	}
	return view
}
