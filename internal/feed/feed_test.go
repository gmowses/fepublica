package feed

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/gmowses/fepublica/internal/store"
)

func sampleEvents() []store.ChangeEvent {
	eid1 := "ibge:3550308"
	return []store.ChangeEvent{
		{
			ID:         1,
			DiffRunID:  1,
			SourceID:   "ceis",
			EnteID:     &eid1,
			ExternalID: "277765",
			ChangeType: "removed",
			DetectedAt: time.Date(2026, 4, 9, 12, 0, 0, 0, time.UTC),
			Severity:   "warn",
		},
		{
			ID:         2,
			DiffRunID:  1,
			SourceID:   "pncp-contratos",
			ExternalID: "95595120000195-1-000009/2026",
			ChangeType: "modified",
			DetectedAt: time.Date(2026, 4, 9, 13, 0, 0, 0, time.UTC),
			Severity:   "warn",
		},
	}
}

func sampleMeta() Meta {
	return Meta{
		Title:      "Fé Pública — mudanças detectadas",
		Subtitle:   "Feed de mudanças em dados públicos brasileiros",
		ID:         "https://fepublica.gmowses.cloud/api/feeds/all.atom",
		BaseURL:    "https://fepublica.gmowses.cloud",
		AuthorName: "Fé Pública",
	}
}

func TestFromChangeEvents(t *testing.T) {
	events := sampleEvents()
	entries := FromChangeEvents(sampleMeta(), events)
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}
	if !strings.Contains(entries[0].Title, "ceis") || !strings.Contains(entries[0].Title, "removed") {
		t.Errorf("entry[0] title missing source/type: %q", entries[0].Title)
	}
	if len(entries[0].Categories) != 3 {
		t.Errorf("entry[0] should have 3 categories, got %d", len(entries[0].Categories))
	}
}

func TestAtomEmitsValidXML(t *testing.T) {
	entries := FromChangeEvents(sampleMeta(), sampleEvents())
	data, err := Atom(sampleMeta(), entries)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte("<?xml")) {
		t.Errorf("atom output should start with <?xml declaration")
	}
	// Must parse as valid XML.
	var doc struct {
		XMLName xml.Name
		Entries []struct {
			Title string `xml:"title"`
		} `xml:"entry"`
	}
	if err := xml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("atom output is not valid XML: %v", err)
	}
	if doc.XMLName.Local != "feed" {
		t.Errorf("root element should be 'feed', got %q", doc.XMLName.Local)
	}
	if len(doc.Entries) != 2 {
		t.Errorf("expected 2 entries in parsed xml, got %d", len(doc.Entries))
	}
}

func TestAtomEmptyEntries(t *testing.T) {
	data, err := Atom(sampleMeta(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("<feed")) {
		t.Error("empty atom should still render feed element")
	}
}

func TestJSONFeedEmitsValidJSON(t *testing.T) {
	entries := FromChangeEvents(sampleMeta(), sampleEvents())
	data, err := JSONFeed(sampleMeta(), entries)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json feed is not valid JSON: %v", err)
	}
	if parsed["version"] != "https://jsonfeed.org/version/1.1" {
		t.Errorf("version wrong: %v", parsed["version"])
	}
	items, ok := parsed["items"].([]any)
	if !ok {
		t.Fatal("items should be array")
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestJSONFeedEmptyItems(t *testing.T) {
	data, err := JSONFeed(sampleMeta(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("\"items\"")) {
		t.Error("empty JSON feed should still include items key")
	}
}
