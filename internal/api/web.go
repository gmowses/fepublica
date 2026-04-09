package api

import (
	"embed"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"

	"github.com/gmowses/fepublica/internal/store"
)

//go:embed web/index.html web/snapshot.html web/event.html
var webFS embed.FS

// indexHTML is the embedded HTML landing page served at GET /.
var indexHTML []byte
var snapshotTpl []byte
var eventTpl []byte

func init() {
	if b, err := webFS.ReadFile("web/index.html"); err == nil {
		indexHTML = b
	} else {
		indexHTML = []byte("<!doctype html><title>Fé Pública</title><h1>Fé Pública</h1><p>index.html missing in build.</p>")
	}
	if b, err := webFS.ReadFile("web/snapshot.html"); err == nil {
		snapshotTpl = b
	}
	if b, err := webFS.ReadFile("web/event.html"); err == nil {
		eventTpl = b
	}
}

// wantsHTML reports whether the client prefers HTML over JSON based on the
// Accept header. Browsers send "text/html,..." so we default to HTML only
// when the client explicitly asks for it. Curl and most API clients get JSON.
func wantsHTML(r *http.Request) bool {
	if r.URL.Query().Get("format") == "json" {
		return false
	}
	if r.URL.Query().Get("format") == "html" {
		return true
	}
	accept := r.Header.Get("Accept")
	if accept == "" {
		return false
	}
	// Prefer HTML only if explicitly listed.
	return strings.Contains(accept, "text/html")
}

// serveIndexHTML writes the embedded landing page.
func serveIndexHTML(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "public, max-age=60")
	h.Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(indexHTML)
}

// serveSnapshotHTML renders the snapshot browse page by substituting placeholders
// in the embedded template with escaped values from the snapshot row. Using a
// simple placeholder substitution rather than html/template keeps the HTML
// source editable without Go-specific templating syntax.
func serveSnapshotHTML(w http.ResponseWriter, snap *store.Snapshot) {
	if snapshotTpl == nil {
		http.Error(w, "snapshot template missing", http.StatusInternalServerError)
		return
	}
	bytesStr := fmt.Sprintf("%d B", snap.BytesSize)
	if snap.BytesSize >= 1<<20 {
		bytesStr = fmt.Sprintf("%.2f MB", float64(snap.BytesSize)/(1<<20))
	} else if snap.BytesSize >= 1<<10 {
		bytesStr = fmt.Sprintf("%.2f KB", float64(snap.BytesSize)/(1<<10))
	}
	merkle := "pendente"
	if len(snap.MerkleRoot) > 0 {
		merkle = store.HexHash(snap.MerkleRoot)
	}
	collected := snap.CollectedAt.Format("2006-01-02 15:04:05 UTC")
	body := string(snapshotTpl)
	body = strings.ReplaceAll(body, "{{SNAPSHOT_ID}}", fmt.Sprintf("%d", snap.ID))
	body = strings.ReplaceAll(body, "{{SOURCE_ID}}", html.EscapeString(snap.SourceID))
	body = strings.ReplaceAll(body, "{{COLLECTED_AT}}", html.EscapeString(collected))
	body = strings.ReplaceAll(body, "{{RECORD_COUNT}}", fmt.Sprintf("%d", snap.RecordCount))
	body = strings.ReplaceAll(body, "{{BYTES_SIZE}}", html.EscapeString(bytesStr))
	body = strings.ReplaceAll(body, "{{MERKLE_ROOT}}", html.EscapeString(merkle))
	body = strings.ReplaceAll(body, "{{COLLECTOR_VERSION}}", html.EscapeString(snap.CollectorVersion))
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "public, max-age=30")
	h.Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
}

// serveEventHTML renders the event detail page. We do light substitution so
// the page can display while the JS fetches the canonical_json.
func serveEventHTML(w http.ResponseWriter, snapshotID int64, externalID string) {
	if eventTpl == nil {
		http.Error(w, "event template missing", http.StatusInternalServerError)
		return
	}
	extURL := url.PathEscape(externalID)
	extFile := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		}
		return '_'
	}, externalID)
	extJSON, _ := json.Marshal(externalID)

	body := string(eventTpl)
	body = strings.ReplaceAll(body, "{{SNAPSHOT_ID}}", fmt.Sprintf("%d", snapshotID))
	body = strings.ReplaceAll(body, "{{EXTERNAL_ID_URL}}", extURL)
	body = strings.ReplaceAll(body, "{{EXTERNAL_ID_FILE}}", extFile)
	body = strings.ReplaceAll(body, "{{EXTERNAL_ID_JSON}}", string(extJSON))
	body = strings.ReplaceAll(body, "{{EXTERNAL_ID}}", html.EscapeString(externalID))
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "public, max-age=30")
	h.Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
}
