package api

import (
	"net/http"
	"strconv"

	"github.com/gmowses/fepublica/internal/feed"
	"github.com/gmowses/fepublica/internal/store"
)

// handleFeedAtomAll serves the global Atom feed of recent change events.
func (s *Server) handleFeedAtomAll(w http.ResponseWriter, r *http.Request) {
	s.serveFeed(w, r, "all", "atom", "")
}

// handleFeedJSONAll serves the global JSON feed.
func (s *Server) handleFeedJSONAll(w http.ResponseWriter, r *http.Request) {
	s.serveFeed(w, r, "all", "json", "")
}

// handleFeedAtomBySource serves an Atom feed filtered by source_id.
func (s *Server) handleFeedAtomBySource(w http.ResponseWriter, r *http.Request) {
	sourceID := r.PathValue("source_id")
	s.serveFeed(w, r, sourceID, "atom", sourceID)
}

// handleFeedJSONBySource serves a JSON feed filtered by source_id.
func (s *Server) handleFeedJSONBySource(w http.ResponseWriter, r *http.Request) {
	sourceID := r.PathValue("source_id")
	s.serveFeed(w, r, sourceID, "json", sourceID)
}

// serveFeed builds and serves a feed document.
func (s *Server) serveFeed(w http.ResponseWriter, r *http.Request, label, format, sourceFilter string) {
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	events, _, err := s.store.ListChangeEvents(r.Context(), store.ListChangeEventsParams{
		SourceID: sourceFilter,
		Limit:    limit,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	title := "Fé Pública — mudanças detectadas"
	subtitle := "Feed de alterações detectadas em dados públicos brasileiros"
	feedURL := s.baseURL + "/api/feeds/all." + format
	if sourceFilter != "" {
		title = "Fé Pública — " + sourceFilter
		subtitle = "Mudanças detectadas na fonte " + sourceFilter
		feedURL = s.baseURL + "/api/feeds/sources/" + sourceFilter + "." + format
	}

	meta := feed.Meta{
		Title:      title,
		Subtitle:   subtitle,
		ID:         feedURL,
		BaseURL:    s.baseURL,
		AuthorName: "Fé Pública",
	}
	entries := feed.FromChangeEvents(meta, events)

	var body []byte
	var contentType string
	switch format {
	case "atom":
		body, err = feed.Atom(meta, entries)
		contentType = "application/atom+xml; charset=utf-8"
	case "json":
		body, err = feed.JSONFeed(meta, entries)
		contentType = "application/feed+json; charset=utf-8"
	default:
		writeError(w, http.StatusBadRequest, http.ErrNotSupported)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	h := w.Header()
	h.Set("Content-Type", contentType)
	h.Set("Cache-Control", "public, max-age=60")
	h.Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(body)
	_ = label // reserved for metrics labels
}
