// Package feed generates Atom and JSON Feed documents from change_events.
//
// Both formats follow established standards:
//   - Atom 1.0 (RFC 4287)
//   - JSON Feed 1.1 (https://www.jsonfeed.org/version/1.1/)
//
// The functions are pure: they take a slice of events plus metadata and
// return bytes. They do not hit the store. They do not hit the network.
// This makes them trivially testable and usable from both the API handlers
// and the notifier worker.
package feed

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"time"

	"github.com/gmowses/fepublica/internal/store"
)

// Meta describes the feed channel itself.
type Meta struct {
	Title       string
	Subtitle    string
	ID          string // stable URI (e.g. "https://fepublica.gmowses.cloud/api/feeds/all.atom")
	BaseURL     string
	AuthorName  string
	AuthorEmail string
}

// Entry is the common projection of a change_event for feed rendering.
type Entry struct {
	ID         string
	Title      string
	Summary    string
	Link       string
	Published  time.Time
	Updated    time.Time
	Categories []string
}

// FromChangeEvents builds feed entries from a slice of change_events.
func FromChangeEvents(meta Meta, events []store.ChangeEvent) []Entry {
	out := make([]Entry, 0, len(events))
	for i := range events {
		out = append(out, entryFromEvent(meta, &events[i]))
	}
	return out
}

func entryFromEvent(meta Meta, e *store.ChangeEvent) Entry {
	id := fmt.Sprintf("%s/api/change-events/%d", meta.BaseURL, e.ID)
	link := fmt.Sprintf("%s/recent?source=%s", meta.BaseURL, e.SourceID)
	return Entry{
		ID:        id,
		Title:     formatTitle(e),
		Summary:   formatSummary(e),
		Link:      link,
		Published: e.DetectedAt,
		Updated:   e.DetectedAt,
		Categories: []string{
			"source:" + e.SourceID,
			"type:" + e.ChangeType,
			"severity:" + e.Severity,
		},
	}
}

func formatTitle(e *store.ChangeEvent) string {
	symbol := map[string]string{
		"added":    "+",
		"removed":  "-",
		"modified": "~",
	}[e.ChangeType]
	return fmt.Sprintf("[%s] %s %s · %s", e.SourceID, symbol, e.ChangeType, e.ExternalID)
}

func formatSummary(e *store.ChangeEvent) string {
	action := map[string]string{
		"added":    "adicionado ao arquivo",
		"removed":  "removido do arquivo (possível edição silenciosa da fonte)",
		"modified": "teve o conteúdo alterado entre duas coletas consecutivas",
	}[e.ChangeType]
	sev := ""
	if e.Severity == "warn" {
		sev = " Severidade warn."
	} else if e.Severity == "alert" {
		sev = " Severidade alert."
	}
	return fmt.Sprintf("Registro %s (%s) %s.%s", e.ExternalID, e.SourceID, action, sev)
}

// ---- Atom 1.0 ----

type atomFeed struct {
	XMLName  xml.Name    `xml:"feed"`
	Xmlns    string      `xml:"xmlns,attr"`
	Title    string      `xml:"title"`
	Subtitle string      `xml:"subtitle,omitempty"`
	ID       string      `xml:"id"`
	Updated  string      `xml:"updated"`
	Links    []atomLink  `xml:"link"`
	Author   *atomAuthor `xml:"author,omitempty"`
	Entries  []atomEntry `xml:"entry"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr,omitempty"`
	Type string `xml:"type,attr,omitempty"`
}

type atomAuthor struct {
	Name  string `xml:"name"`
	Email string `xml:"email,omitempty"`
}

type atomEntry struct {
	Title     string         `xml:"title"`
	ID        string         `xml:"id"`
	Link      atomLink       `xml:"link"`
	Published string         `xml:"published"`
	Updated   string         `xml:"updated"`
	Summary   string         `xml:"summary"`
	Category  []atomCategory `xml:"category"`
}

type atomCategory struct {
	Term string `xml:"term,attr"`
}

// Atom renders the feed as Atom 1.0.
func Atom(meta Meta, entries []Entry) ([]byte, error) {
	updated := time.Now().UTC()
	if len(entries) > 0 {
		updated = entries[0].Updated
	}

	f := atomFeed{
		Xmlns:    "http://www.w3.org/2005/Atom",
		Title:    meta.Title,
		Subtitle: meta.Subtitle,
		ID:       meta.ID,
		Updated:  updated.UTC().Format(time.RFC3339),
		Links: []atomLink{
			{Href: meta.ID, Rel: "self", Type: "application/atom+xml"},
			{Href: meta.BaseURL, Rel: "alternate", Type: "text/html"},
		},
	}
	if meta.AuthorName != "" {
		f.Author = &atomAuthor{Name: meta.AuthorName, Email: meta.AuthorEmail}
	}
	for _, e := range entries {
		cats := make([]atomCategory, len(e.Categories))
		for i, c := range e.Categories {
			cats[i] = atomCategory{Term: c}
		}
		f.Entries = append(f.Entries, atomEntry{
			Title:     e.Title,
			ID:        e.ID,
			Link:      atomLink{Href: e.Link, Rel: "alternate", Type: "text/html"},
			Published: e.Published.UTC().Format(time.RFC3339),
			Updated:   e.Updated.UTC().Format(time.RFC3339),
			Summary:   e.Summary,
			Category:  cats,
		})
	}
	buf, err := xml.MarshalIndent(f, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("feed: atom marshal: %w", err)
	}
	return append([]byte(xml.Header), buf...), nil
}

// ---- JSON Feed 1.1 ----

type jsonFeed struct {
	Version     string       `json:"version"`
	Title       string       `json:"title"`
	Description string       `json:"description,omitempty"`
	HomePageURL string       `json:"home_page_url,omitempty"`
	FeedURL     string       `json:"feed_url,omitempty"`
	Authors     []jsonAuthor `json:"authors,omitempty"`
	Items       []jsonItem   `json:"items"`
}

type jsonAuthor struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

type jsonItem struct {
	ID            string    `json:"id"`
	URL           string    `json:"url"`
	Title         string    `json:"title"`
	ContentText   string    `json:"content_text"`
	DatePublished time.Time `json:"date_published"`
	DateModified  time.Time `json:"date_modified"`
	Tags          []string  `json:"tags,omitempty"`
}

// JSONFeed renders the feed as JSON Feed 1.1.
func JSONFeed(meta Meta, entries []Entry) ([]byte, error) {
	f := jsonFeed{
		Version:     "https://jsonfeed.org/version/1.1",
		Title:       meta.Title,
		Description: meta.Subtitle,
		HomePageURL: meta.BaseURL,
		FeedURL:     meta.ID,
	}
	if meta.AuthorName != "" {
		f.Authors = []jsonAuthor{{Name: meta.AuthorName, URL: meta.BaseURL}}
	}
	for _, e := range entries {
		f.Items = append(f.Items, jsonItem{
			ID:            e.ID,
			URL:           e.Link,
			Title:         e.Title,
			ContentText:   e.Summary,
			DatePublished: e.Published.UTC(),
			DateModified:  e.Updated.UTC(),
			Tags:          e.Categories,
		})
	}
	return json.MarshalIndent(f, "", "  ")
}
