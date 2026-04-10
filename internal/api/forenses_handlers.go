package api

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

func (s *Server) handleForensesSummary(w http.ResponseWriter, r *http.Request) {
	sum, err := s.store.GetForensesSummary(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, sum)
}

func (s *Server) handleForensesSancionados(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	rows, err := s.store.FindSancionadosContratados(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"findings": rows})
}

func (s *Server) handleForensesConcentracao(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	rows, err := s.store.FindConcentracaoOrgao(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"findings": rows})
}

func (s *Server) handleForensesOutliers(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	rows, err := s.store.FindValorOutliers(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"findings": rows})
}

func (s *Server) handleForensesCPGFAlto(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	rows, err := s.store.FindCPGFAltoValor(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"findings": rows})
}

func (s *Server) handleForensesCPGFOpaco(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	rows, err := s.store.FindCPGFEstabOpaco(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"findings": rows})
}

// handleForensesCounts returns counts from the persisted findings table.
func (s *Server) handleForensesCounts(w http.ResponseWriter, r *http.Request) {
	c, err := s.store.GetFindingsCounts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// handleForensesPersisted lists persisted findings (curated state).
func (s *Server) handleForensesPersisted(w http.ResponseWriter, r *http.Request) {
	fType := r.URL.Query().Get("type")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	includeDismissed := r.URL.Query().Get("include_dismissed") == "1"
	rows, err := s.store.ListPersistedFindings(r.Context(), fType, includeDismissed, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]map[string]interface{}, 0, len(rows))
	for _, pf := range rows {
		m := map[string]interface{}{
			"id":           pf.ID,
			"type":         string(pf.Finding.Type),
			"severity":     string(pf.Finding.Severity),
			"title":        pf.Finding.Title,
			"subject":      pf.Finding.Subject,
			"valor":        pf.Finding.Valor,
			"evidence":     pf.Finding.Evidence,
			"link":         pf.Finding.Link,
			"first_seen":   pf.FirstSeenAt,
			"last_seen":    pf.LastSeenAt,
			"dismissed_at": pf.DismissedAt,
			"confirmed_at": pf.ConfirmedAt,
		}
		out = append(out, m)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"findings": out})
}

// atomEntry mirrors the shape used by the existing feeds package.
type forensesAtomFeed struct {
	XMLName xml.Name           `xml:"feed"`
	Xmlns   string             `xml:"xmlns,attr"`
	Title   string             `xml:"title"`
	Link    forensesAtomLink   `xml:"link"`
	Updated string             `xml:"updated"`
	ID      string             `xml:"id"`
	Entries []forensesAtomEntry `xml:"entry"`
}

type forensesAtomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr,omitempty"`
}

type forensesAtomEntry struct {
	Title     string           `xml:"title"`
	ID        string           `xml:"id"`
	Updated   string           `xml:"updated"`
	Published string           `xml:"published"`
	Link      forensesAtomLink `xml:"link"`
	Summary   string           `xml:"summary"`
	Category  forensesAtomCat  `xml:"category"`
}

type forensesAtomCat struct {
	Term string `xml:"term,attr"`
}

// handleForensesAtom serves an Atom feed of new findings (high severity by
// default), so journalists/researchers can subscribe.
func (s *Server) handleForensesAtom(w http.ResponseWriter, r *http.Request) {
	rows, err := s.store.ListPersistedFindings(r.Context(), "", false, 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	feed := forensesAtomFeed{
		Xmlns:   "http://www.w3.org/2005/Atom",
		Title:   "Fé Pública — Forenses",
		Link:    forensesAtomLink{Href: s.baseURL + "/forenses", Rel: "alternate"},
		Updated: time.Now().UTC().Format(time.RFC3339),
		ID:      s.baseURL + "/api/feeds/forenses/atom",
	}
	for _, pf := range rows {
		var summary string
		if expl, ok := pf.Finding.Evidence["explanation"].(string); ok {
			summary = expl
		}
		valStr := ""
		if pf.Finding.Valor != nil {
			valStr = fmt.Sprintf(" — R$ %.0f", *pf.Finding.Valor)
		}
		feed.Entries = append(feed.Entries, forensesAtomEntry{
			Title:     fmt.Sprintf("[%s] %s%s", pf.Finding.Severity, pf.Finding.Subject, valStr),
			ID:        fmt.Sprintf("%s/forenses/finding/%d", s.baseURL, pf.ID),
			Updated:   pf.LastSeenAt.UTC().Format(time.RFC3339),
			Published: pf.FirstSeenAt.UTC().Format(time.RFC3339),
			Link:      forensesAtomLink{Href: s.baseURL + pf.Finding.Link},
			Summary:   pf.Finding.Title + " — " + summary,
			Category:  forensesAtomCat{Term: string(pf.Finding.Type)},
		})
	}
	w.Header().Set("Content-Type", "application/atom+xml; charset=utf-8")
	w.Write([]byte(xml.Header))
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(feed); err != nil {
		writeError(w, http.StatusInternalServerError, err)
	}
}
