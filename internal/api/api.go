// Package api implements the public HTTP API.
package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/gmowses/fepublica/internal/merkle"
	"github.com/gmowses/fepublica/internal/store"
)

// Server is the HTTP API server.
type Server struct {
	store   *store.Store
	logger  zerolog.Logger
	version string
	baseURL string
	started time.Time
}

// New creates a Server.
func New(s *store.Store, logger zerolog.Logger, version, baseURL string) *Server {
	return &Server{
		store:   s,
		logger:  logger,
		version: version,
		baseURL: baseURL,
		started: time.Now(),
	}
}

// Routes returns an http.Handler with all endpoints wired.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /sources", s.handleListSources)
	mux.HandleFunc("GET /snapshots", s.handleListSnapshots)
	mux.HandleFunc("GET /snapshots/{id}", s.handleGetSnapshot)
	mux.HandleFunc("GET /snapshots/{id}/anchors", s.handleListSnapshotAnchors)
	mux.HandleFunc("GET /snapshots/{id}/events/{external_id}/proof", s.handleProof)
	return logging(s.logger, mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"version": s.version,
		"uptime":  time.Since(s.started).Round(time.Second).String(),
		"now":     time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleListSources(w http.ResponseWriter, r *http.Request) {
	sources, err := s.store.ListSources(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sources": sources})
}

func (s *Server) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	source := q.Get("source")
	limit := 50
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	snaps, err := s.store.ListSnapshots(r.Context(), source, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"snapshots": snapshotsDTO(snaps)})
}

func (s *Server) handleGetSnapshot(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	snap, err := s.store.GetSnapshot(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshotDTO(snap))
}

func (s *Server) handleListSnapshotAnchors(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	anchors, err := s.store.ListAnchorsForSnapshot(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"anchors": anchorsDTO(anchors)})
}

// ProofDTO is the exported JSON format the verify CLI consumes.
type ProofDTO struct {
	Version             int               `json:"version"`
	SourceID            string            `json:"source_id"`
	SnapshotID          int64             `json:"snapshot_id"`
	SnapshotCollectedAt time.Time         `json:"snapshot_collected_at"`
	Event               ProofEvent        `json:"event"`
	Merkle              ProofMerkle       `json:"merkle"`
	Anchors             []ProofAnchor     `json:"anchors"`
	GeneratedAt         time.Time         `json:"generated_at"`
}

type ProofEvent struct {
	ExternalID    string          `json:"external_id"`
	ContentHash   string          `json:"content_hash"`
	CanonicalJSON json.RawMessage `json:"canonical_json"`
}

type ProofMerkle struct {
	Root     string      `json:"root"`
	Index    int         `json:"index"`
	Siblings []ProofStep `json:"siblings"`
}

type ProofStep struct {
	Sibling string `json:"sibling"`
	Side    string `json:"side"` // "left" or "right"
}

type ProofAnchor struct {
	CalendarURL   string `json:"calendar_url"`
	ReceiptBase64 string `json:"receipt_base64"`
	Upgraded      bool   `json:"upgraded"`
	BlockHeight   *int   `json:"block_height,omitempty"`
	SubmittedAt   string `json:"submitted_at"`
}

func (s *Server) handleProof(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	externalID := r.PathValue("external_id")
	if externalID == "" {
		writeError(w, http.StatusBadRequest, errors.New("missing external_id"))
		return
	}

	snap, err := s.store.GetSnapshot(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if len(snap.MerkleRoot) != merkle.HashSize {
		writeError(w, http.StatusConflict, errors.New("snapshot has no merkle root yet"))
		return
	}

	// Locate the target event and also pull all events to rebuild the tree
	// (MVP: we recompute the tree on-demand; v0.2 will cache the intermediate nodes).
	events, err := s.store.ListEventsBySnapshot(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	var targetIdx = -1
	for i, ev := range events {
		if ev.ExternalID == externalID {
			targetIdx = i
			break
		}
	}
	if targetIdx == -1 {
		writeError(w, http.StatusNotFound,
			fmt.Errorf("event %q not found in snapshot %d", externalID, id))
		return
	}

	leaves := make([][merkle.HashSize]byte, len(events))
	for i, ev := range events {
		var h [merkle.HashSize]byte
		copy(h[:], ev.ContentHash)
		leaves[i] = h
	}
	tree, err := merkle.Build(leaves)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	proofSteps, err := tree.Proof(targetIdx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	anchors, err := s.store.ListAnchorsForSnapshot(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	dto := ProofDTO{
		Version:             1,
		SourceID:            snap.SourceID,
		SnapshotID:          snap.ID,
		SnapshotCollectedAt: snap.CollectedAt,
		Event: ProofEvent{
			ExternalID:    events[targetIdx].ExternalID,
			ContentHash:   store.HexHash(events[targetIdx].ContentHash),
			CanonicalJSON: json.RawMessage(events[targetIdx].CanonicalJSON),
		},
		Merkle: ProofMerkle{
			Root:  store.HexHash(snap.MerkleRoot),
			Index: targetIdx,
		},
		GeneratedAt: time.Now().UTC(),
	}
	for _, step := range proofSteps {
		side := "right"
		if step.Side == merkle.SideLeft {
			side = "left"
		}
		dto.Merkle.Siblings = append(dto.Merkle.Siblings, ProofStep{
			Sibling: store.HexHash(step.Sibling[:]),
			Side:    side,
		})
	}
	for _, a := range anchors {
		dto.Anchors = append(dto.Anchors, ProofAnchor{
			CalendarURL:   a.CalendarURL,
			ReceiptBase64: base64.StdEncoding.EncodeToString(a.Receipt),
			Upgraded:      a.Upgraded,
			BlockHeight:   a.BlockHeight,
			SubmittedAt:   a.SubmittedAt.UTC().Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, dto)
}

// snapshotDTO and helpers

type snapshotView struct {
	ID               int64  `json:"id"`
	SourceID         string `json:"source_id"`
	CollectedAt      string `json:"collected_at"`
	APIVersion       string `json:"api_version,omitempty"`
	RecordCount      int    `json:"record_count"`
	BytesSize        int64  `json:"bytes_size"`
	MerkleRoot       string `json:"merkle_root,omitempty"`
	MerkleComputedAt string `json:"merkle_computed_at,omitempty"`
	CollectorVersion string `json:"collector_version"`
	Notes            string `json:"notes,omitempty"`
}

func snapshotDTO(s *store.Snapshot) snapshotView {
	view := snapshotView{
		ID:               s.ID,
		SourceID:         s.SourceID,
		CollectedAt:      s.CollectedAt.UTC().Format(time.RFC3339),
		APIVersion:       s.APIVersion,
		RecordCount:      s.RecordCount,
		BytesSize:        s.BytesSize,
		CollectorVersion: s.CollectorVersion,
		Notes:            s.Notes,
	}
	if len(s.MerkleRoot) > 0 {
		view.MerkleRoot = store.HexHash(s.MerkleRoot)
	}
	if s.MerkleComputedAt != nil {
		view.MerkleComputedAt = s.MerkleComputedAt.UTC().Format(time.RFC3339)
	}
	return view
}

func snapshotsDTO(snaps []store.Snapshot) []snapshotView {
	out := make([]snapshotView, len(snaps))
	for i := range snaps {
		out[i] = snapshotDTO(&snaps[i])
	}
	return out
}

type anchorView struct {
	ID            int64  `json:"id"`
	SnapshotID    int64  `json:"snapshot_id"`
	CalendarURL   string `json:"calendar_url"`
	SubmittedAt   string `json:"submitted_at"`
	Upgraded      bool   `json:"upgraded"`
	UpgradedAt    string `json:"upgraded_at,omitempty"`
	BlockHeight   *int   `json:"block_height,omitempty"`
	ReceiptBytes  int    `json:"receipt_bytes"`
}

func anchorsDTO(anchors []store.Anchor) []anchorView {
	out := make([]anchorView, len(anchors))
	for i, a := range anchors {
		v := anchorView{
			ID:           a.ID,
			SnapshotID:   a.SnapshotID,
			CalendarURL:  a.CalendarURL,
			SubmittedAt:  a.SubmittedAt.UTC().Format(time.RFC3339),
			Upgraded:     a.Upgraded,
			BlockHeight:  a.BlockHeight,
			ReceiptBytes: len(a.Receipt),
		}
		if a.UpgradedAt != nil {
			v.UpgradedAt = a.UpgradedAt.UTC().Format(time.RFC3339)
		}
		out[i] = v
	}
	return out
}

// HTTP helpers

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func parseID(s string) (int64, error) {
	if s == "" {
		return 0, errors.New("missing id")
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid id %q", s)
	}
	return n, nil
}

// logging is a small middleware that logs each request at debug level.
func logging(logger zerolog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, code: 200}
		next.ServeHTTP(rec, r)
		logger.Debug().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("remote", clientIP(r)).
			Int("status", rec.code).
			Dur("elapsed", time.Since(start)).
			Msg("api: request")
	})
}

type statusRecorder struct {
	http.ResponseWriter
	code int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.code = code
	s.ResponseWriter.WriteHeader(code)
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	return r.RemoteAddr
}

// Shutdown is a convenience wrapper for callers building a Server directly.
func (s *Server) Shutdown(ctx context.Context) error {
	_ = ctx
	return nil
}
