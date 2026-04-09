package api

import (
	"embed"
	"net/http"
	"strings"
)

//go:embed web/index.html
var webFS embed.FS

// indexHTML is the embedded HTML landing page served at GET /.
// Loaded once at init; it's a static asset, safe to cache in memory.
var indexHTML []byte

func init() {
	b, err := webFS.ReadFile("web/index.html")
	if err != nil {
		// Fall back to a minimal message if the file is missing.
		indexHTML = []byte("<!doctype html><title>Fé Pública</title><h1>Fé Pública</h1><p>index.html missing in build.</p>")
		return
	}
	indexHTML = b
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
