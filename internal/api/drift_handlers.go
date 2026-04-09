package api

import (
	"encoding/hex"
	"net/http"
	"strconv"
	"time"

	"github.com/gmowses/fepublica/internal/store"
)

// diffRunView is the JSON shape returned by the diff_runs endpoints.
type diffRunView struct {
	ID             int64  `json:"id"`
	SourceID       string `json:"source_id"`
	SnapshotAID    int64  `json:"snapshot_a_id"`
	SnapshotBID    int64  `json:"snapshot_b_id"`
	AddedCount     int    `json:"added_count"`
	RemovedCount   int    `json:"removed_count"`
	ModifiedCount  int    `json:"modified_count"`
	RanAt          string `json:"ran_at"`
	DurationMS     int    `json:"duration_ms"`
}

func diffRunDTO(dr *store.DiffRun) diffRunView {
	return diffRunView{
		ID:            dr.ID,
		SourceID:      dr.SourceID,
		SnapshotAID:   dr.SnapshotAID,
		SnapshotBID:   dr.SnapshotBID,
		AddedCount:    dr.AddedCount,
		RemovedCount:  dr.RemovedCount,
		ModifiedCount: dr.ModifiedCount,
		RanAt:         dr.RanAt.UTC().Format(time.RFC3339),
		DurationMS:    dr.DurationMS,
	}
}

// handleListDiffRuns returns a paginated list of diff_runs, newest first.
// Supports ?source=<id> and ?limit=N.
func (s *Server) handleListDiffRuns(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	sourceID := q.Get("source")
	limit := 50
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	runs, err := s.store.ListDiffRuns(r.Context(), sourceID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]diffRunView, len(runs))
	for i := range runs {
		out[i] = diffRunDTO(&runs[i])
	}
	writeJSON(w, http.StatusOK, map[string]any{"diff_runs": out})
}

// handleGetDiffRun returns a single diff_run by id.
func (s *Server) handleGetDiffRun(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	dr, err := s.store.GetDiffRun(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, diffRunDTO(dr))
}

// changeEventView is the JSON shape returned by the change_events endpoints.
type changeEventView struct {
	ID           int64   `json:"id"`
	DiffRunID    int64   `json:"diff_run_id"`
	SourceID     string  `json:"source_id"`
	EnteID       *string `json:"ente_id,omitempty"`
	ExternalID   string  `json:"external_id"`
	ChangeType   string  `json:"change_type"`
	ContentHashA string  `json:"content_hash_a,omitempty"`
	ContentHashB string  `json:"content_hash_b,omitempty"`
	DetectedAt   string  `json:"detected_at"`
	Severity     string  `json:"severity"`
	Published    map[string]bool `json:"published"`
}

func changeEventDTO(ce *store.ChangeEvent) changeEventView {
	v := changeEventView{
		ID:         ce.ID,
		DiffRunID:  ce.DiffRunID,
		SourceID:   ce.SourceID,
		EnteID:     ce.EnteID,
		ExternalID: ce.ExternalID,
		ChangeType: ce.ChangeType,
		DetectedAt: ce.DetectedAt.UTC().Format(time.RFC3339),
		Severity:   ce.Severity,
		Published: map[string]bool{
			"rss":      ce.PublishedRSS,
			"telegram": ce.PublishedTelegram,
			"mastodon": ce.PublishedMastodon,
			"webhook":  ce.PublishedWebhook,
			"email":    ce.PublishedEmail,
		},
	}
	if len(ce.ContentHashA) > 0 {
		v.ContentHashA = hex.EncodeToString(ce.ContentHashA)
	}
	if len(ce.ContentHashB) > 0 {
		v.ContentHashB = hex.EncodeToString(ce.ContentHashB)
	}
	return v
}

// handleListChangeEvents returns a paginated list of change_events with filters.
//
// Query params:
//
//	?source=<id>     filter by source
//	?ente=<id>       filter by ente_id
//	?severity=<lvl>  info|warn|alert
//	?type=<t>        added|removed|modified
//	?since=<iso>     only events at or after this timestamp
//	?limit=N         max 500, default 50
//	?offset=N        pagination offset
func (s *Server) handleListChangeEvents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	params := store.ListChangeEventsParams{
		SourceID:   q.Get("source"),
		EnteID:     q.Get("ente"),
		Severity:   q.Get("severity"),
		ChangeType: q.Get("type"),
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
	if v := q.Get("since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			params.Since = &t
		}
	}

	events, total, err := s.store.ListChangeEvents(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]changeEventView, len(events))
	for i := range events {
		out[i] = changeEventDTO(&events[i])
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"total":         total,
		"limit":         params.Limit,
		"offset":        params.Offset,
		"change_events": out,
	})
}

// handleGetChangeEvent returns a single change_event by id.
func (s *Server) handleGetChangeEvent(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	ce, err := s.store.GetChangeEvent(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, changeEventDTO(ce))
}
