package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gmowses/fepublica/internal/store"
)

// handleListGaps returns the gap catalog with filters.
func (s *Server) handleListGaps(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	params := store.ListGapsParams{
		Status:   q.Get("status"),
		Category: q.Get("category"),
		EnteID:   q.Get("ente"),
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
	rows, total, err := s.store.ListGaps(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]map[string]any, len(rows))
	for i, g := range rows {
		out[i] = gapDTO(&g)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"total":  total,
		"limit":  params.Limit,
		"offset": params.Offset,
		"gaps":   out,
	})
}

// handleCreateGap accepts a public gap report. Future versions will add
// moderation / auth. For now, any caller can POST a gap — it's marked as
// 'open' and becomes visible in the catalog.
func (s *Server) handleCreateGap(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusMethodNotAllowed, errors.New("public gap submission is disabled"))
}

func gapDTO(g *store.Gap) map[string]any {
	view := map[string]any{
		"id":            g.ID,
		"category":      g.Category,
		"title":         g.Title,
		"description":   g.Description,
		"severity":      g.Severity,
		"status":        g.Status,
		"first_seen_at": g.FirstSeenAt.UTC().Format(time.RFC3339),
		"last_seen_at":  g.LastSeenAt.UTC().Format(time.RFC3339),
	}
	if g.EnteID != nil {
		view["ente_id"] = *g.EnteID
	}
	if g.SourceID != nil {
		view["source_id"] = *g.SourceID
	}
	if g.LegalReference != nil {
		view["legal_reference"] = *g.LegalReference
	}
	if g.ResolvedAt != nil {
		view["resolved_at"] = g.ResolvedAt.UTC().Format(time.RFC3339)
	}
	return view
}
