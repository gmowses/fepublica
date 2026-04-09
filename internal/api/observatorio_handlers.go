package api

import "net/http"

// handleObservatorioStats returns the aggregate counters for the Observatório
// dashboard hero. All JSON, all cheap queries.
func (s *Server) handleObservatorioStats(w http.ResponseWriter, r *http.Request) {
	st, err := s.store.GetObservatorioStats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// handleEntesByUF returns ente counts grouped by UF for the map heat.
func (s *Server) handleEntesByUF(w http.ResponseWriter, r *http.Request) {
	m, err := s.store.EntesByUF(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"by_uf": m})
}

// handleChangeEventsByUF returns change_event counts grouped by UF.
func (s *Server) handleChangeEventsByUF(w http.ResponseWriter, r *http.Request) {
	m, err := s.store.ChangeEventsByUF(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"by_uf": m})
}
