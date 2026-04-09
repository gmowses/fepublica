package api

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:spa
var spaFS embed.FS

// spaRoot is the embedded filesystem rooted at spa/ (i.e. the Vite dist/).
var spaRoot fs.FS

func init() {
	sub, err := fs.Sub(spaFS, "spa")
	if err != nil {
		// If the embed failed (e.g. spa directory missing during dev), fall
		// back to an empty FS so the server still starts. The API endpoints
		// continue to work; only the frontend is absent.
		spaRoot = emptyFS{}
		return
	}
	spaRoot = sub
}

// serveSPA serves static files from the embedded SPA root. On any request
// for a path that is not a real file (or any HTML accept), it falls back to
// serving index.html so client-side routing works.
func serveSPA(w http.ResponseWriter, r *http.Request) {
	// Never intercept /api paths.
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}

	reqPath := strings.TrimPrefix(r.URL.Path, "/")
	if reqPath == "" {
		reqPath = "index.html"
	}

	// Try to open the exact file first.
	f, err := spaRoot.Open(reqPath)
	if err != nil {
		// Fall back to index.html for any missing path (SPA client routing).
		serveIndex(w, r)
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil || stat.IsDir() {
		serveIndex(w, r)
		return
	}

	// Set cache headers: assets directory is fingerprinted so safe to
	// cache long; everything else is short-lived.
	if strings.HasPrefix(reqPath, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=300")
	}
	w.Header().Set("Content-Type", contentTypeFor(reqPath))

	http.ServeContent(w, r, path.Base(reqPath), stat.ModTime(), asReadSeeker(f))
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	f, err := spaRoot.Open("index.html")
	if err != nil {
		http.Error(w, "index.html missing from embedded SPA build", http.StatusInternalServerError)
		return
	}
	defer f.Close()
	stat, _ := f.Stat()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=60")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	if stat != nil {
		http.ServeContent(w, r, "index.html", stat.ModTime(), asReadSeeker(f))
		return
	}
	_, _ = copyFile(w, f)
}

func contentTypeFor(p string) string {
	switch {
	case strings.HasSuffix(p, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(p, ".js"), strings.HasSuffix(p, ".mjs"):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(p, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(p, ".json"):
		return "application/json; charset=utf-8"
	case strings.HasSuffix(p, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(p, ".png"):
		return "image/png"
	case strings.HasSuffix(p, ".jpg"), strings.HasSuffix(p, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(p, ".webp"):
		return "image/webp"
	case strings.HasSuffix(p, ".woff2"):
		return "font/woff2"
	case strings.HasSuffix(p, ".woff"):
		return "font/woff"
	case strings.HasSuffix(p, ".map"):
		return "application/json; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}

// asReadSeeker tries to cast the fs.File to a ReadSeeker so http.ServeContent
// can range-serve. Falls back to a wrapper that buffers the whole file.
func asReadSeeker(f fs.File) seekReader {
	if rs, ok := f.(seekReader); ok {
		return rs
	}
	// Buffered fallback.
	return nil
}

type seekReader interface {
	Read(p []byte) (n int, err error)
	Seek(offset int64, whence int) (int64, error)
}

func copyFile(w http.ResponseWriter, f fs.File) (int64, error) {
	buf := make([]byte, 32*1024)
	var total int64
	for {
		n, err := f.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return total, werr
			}
			total += int64(n)
		}
		if err != nil {
			if err.Error() == "EOF" {
				return total, nil
			}
			return total, err
		}
	}
}

// emptyFS is used as fallback when the spa directory is missing at build time.
type emptyFS struct{}

func (emptyFS) Open(name string) (fs.File, error) {
	return nil, fs.ErrNotExist
}
