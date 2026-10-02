package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gmowses/fepublica/internal/store"
)

func TestServeSPAAddsSecurityHeadersToIndexAndFallback(t *testing.T) {
	for _, requestPath := range []string{"/", "/client-route"} {
		t.Run(requestPath, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, requestPath, nil)
			w := httptest.NewRecorder()
			serveSPA(w, r)
			if got := w.Header().Get("Content-Security-Policy"); got == "" {
				t.Fatal("missing Content-Security-Policy")
			} else if !strings.Contains(got, "connect-src 'self' https://raw.githubusercontent.com") {
				t.Fatalf("Content-Security-Policy = %q, want raw.githubusercontent.com in connect-src", got)
			}
			if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
				t.Fatalf("X-Content-Type-Options = %q", got)
			}
			if got := w.Header().Get("Referrer-Policy"); got != "strict-origin-when-cross-origin" {
				t.Fatalf("Referrer-Policy = %q", got)
			}
		})
	}
}

func TestCartaoDTOMasksCPF(t *testing.T) {
	got := cartaoDTO(&store.CartaoSummary{PortadorCPF: "123.456.789-01"})
	if want := "***.456.789-**"; got.PortadorCPF != want {
		t.Fatalf("PortadorCPF = %q, want %q", got.PortadorCPF, want)
	}
}
