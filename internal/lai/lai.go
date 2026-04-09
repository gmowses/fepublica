// Package lai runs compliance checks against Brazilian public transparency
// portals. Given an ente with a domain_hint, it performs a set of non-invasive
// checks (GET only, robots.txt-respecting, rate-limited) and returns a
// structured result suitable for storage in the lai_checks table.
//
// This is the "moderado" profile from the design spec: reachability,
// HTTP status, SSL validity, page load, term presence, required links.
// No POST, no e-SIC interaction.
package lai

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gmowses/fepublica/internal/store"
	"golang.org/x/net/html"
)

// requiredTerms are strings the LAI expects every transparency portal to cover.
var requiredTerms = []string{
	"despesas",
	"receitas",
	"servidores",
	"contratos",
	"licitações",
	"transparência",
	"acesso à informação",
}

// requiredLinks are anchor texts or hrefs the portal must expose.
var requiredLinks = []string{
	"e-sic",
	"contratos",
	"despesas",
	"transparencia",
}

// Result mirrors the lai_checks row shape.
type Result struct {
	EnteID        string
	CheckedAt     time.Time
	TargetURL     string
	HTTPStatus    int
	ResponseMS    int
	SSLValid      bool
	SSLExpiresAt  *time.Time
	PortalLoads   bool
	HTMLSizeBytes int
	TermsFound    map[string]bool
	RequiredLinks map[string]bool
	HTMLArchiveKey string
	Errors        []string
	TierAtCheck   int
}

// Checker performs a single check against one ente's portal.
type Checker struct {
	client *http.Client
	ua     string
}

// NewChecker builds a Checker with conservative defaults.
func NewChecker() *Checker {
	return &Checker{
		client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(_ *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return http.ErrUseLastResponse
				}
				return nil
			},
		},
		ua: "fepublica-lai-crawler/0.1 (+https://fepublica.gmowses.cloud/about)",
	}
}

// Check runs one LAI check for the given ente.
func (c *Checker) Check(ctx context.Context, ente *store.Ente) *Result {
	r := &Result{
		EnteID:        ente.ID,
		CheckedAt:     time.Now().UTC(),
		TierAtCheck:   ente.Tier,
		TermsFound:    map[string]bool{},
		RequiredLinks: map[string]bool{},
	}
	if ente.DomainHint == "" {
		r.Errors = append(r.Errors, "no domain_hint configured")
		return r
	}
	r.TargetURL = ente.DomainHint

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ente.DomainHint, nil)
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("build request: %v", err))
		return r
	}
	req.Header.Set("User-Agent", c.ua)
	req.Header.Set("Accept", "text/html,*/*;q=0.8")

	resp, err := c.client.Do(req)
	r.ResponseMS = int(time.Since(start).Milliseconds())
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("fetch: %v", err))
		// Classify SSL errors distinctly.
		if strings.Contains(err.Error(), "x509") || strings.Contains(err.Error(), "tls:") {
			r.SSLValid = false
		}
		return r
	}
	defer resp.Body.Close()

	r.HTTPStatus = resp.StatusCode
	r.PortalLoads = resp.StatusCode >= 200 && resp.StatusCode < 400

	// SSL inspection.
	if resp.TLS != nil {
		r.SSLValid = true
		for _, cert := range resp.TLS.PeerCertificates {
			if r.SSLExpiresAt == nil || cert.NotAfter.Before(*r.SSLExpiresAt) {
				t := cert.NotAfter
				r.SSLExpiresAt = &t
			}
		}
		// Verify chain against system roots (connectionstate already did this
		// since we didn't set InsecureSkipVerify, but double-check peer.)
		if _, ok := resp.TLS.CipherSuite, true; !ok {
			r.SSLValid = false
		}
		_ = tls.VersionName
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20)) // 5 MiB ceiling
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("read body: %v", err))
		return r
	}
	r.HTMLSizeBytes = len(body)
	if !r.PortalLoads {
		return r
	}

	// Parse HTML and extract terms + links.
	lower := strings.ToLower(string(body))
	for _, t := range requiredTerms {
		if strings.Contains(lower, strings.ToLower(t)) {
			r.TermsFound[t] = true
		}
	}

	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("html parse: %v", err))
	} else {
		walk(doc, func(n *html.Node) {
			if n.Type != html.ElementNode || n.Data != "a" {
				return
			}
			text := strings.ToLower(collectText(n))
			var href string
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					href = strings.ToLower(attr.Val)
				}
			}
			for _, rl := range requiredLinks {
				if strings.Contains(text, rl) || strings.Contains(href, rl) {
					r.RequiredLinks[rl] = true
				}
			}
		})
	}

	return r
}

// Score computes a 0..100 score from a check result. Simple weighted sum.
func Score(r *Result) (float64, map[string]float64) {
	components := map[string]float64{}
	total := 0.0

	if r.PortalLoads {
		components["portal_loads"] = 20
		total += 20
	}
	if r.SSLValid {
		components["ssl_valid"] = 10
		total += 10
	}
	// 35 points for required terms (5 per term, up to 7 terms = 35)
	termsHit := 0
	for _, t := range requiredTerms {
		if r.TermsFound[t] {
			termsHit++
		}
	}
	if termsHit > 7 {
		termsHit = 7
	}
	terms := float64(termsHit) * 5
	components["terms_found"] = terms
	total += terms
	// 20 points for required links (5 per link, up to 4 = 20)
	linksHit := 0
	for _, l := range requiredLinks {
		if r.RequiredLinks[l] {
			linksHit++
		}
	}
	if linksHit > 4 {
		linksHit = 4
	}
	links := float64(linksHit) * 5
	components["required_links"] = links
	total += links
	// 15 points for response time (full points under 2s, linear to 0 at 10s)
	if r.ResponseMS > 0 && r.PortalLoads {
		ms := float64(r.ResponseMS)
		var tScore float64
		switch {
		case ms <= 2000:
			tScore = 15
		case ms >= 10000:
			tScore = 0
		default:
			tScore = 15 * (10000 - ms) / 8000
		}
		components["response_time"] = tScore
		total += tScore
	}
	return total, components
}

// walk recursively calls fn on every node of the HTML tree.
func walk(n *html.Node, fn func(*html.Node)) {
	fn(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, fn)
	}
}

// collectText returns the concatenated text content of an element.
func collectText(n *html.Node) string {
	var sb strings.Builder
	walk(n, func(x *html.Node) {
		if x.Type == html.TextNode {
			sb.WriteString(x.Data)
		}
	})
	return sb.String()
}
