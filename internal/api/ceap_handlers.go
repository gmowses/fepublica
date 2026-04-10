package api

import (
	"net/http"
)

func (s *Server) handleCEAPStats(w http.ResponseWriter, r *http.Request) {
	st, err := s.store.GetCEAPStats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}
