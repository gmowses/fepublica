package main

import (
	"testing"

	"github.com/gmowses/fepublica/internal/transparencia/cartoes"
	"github.com/gmowses/fepublica/internal/transparencia/ceis"
	"github.com/gmowses/fepublica/internal/transparencia/cnep"
	"github.com/gmowses/fepublica/internal/transparencia/pncp"
)

func TestRequiresTransparenciaAPIKey(t *testing.T) {
	for _, sourceID := range []string{ceis.SourceID, cnep.SourceID, cartoes.SourceID} {
		if !requiresTransparenciaAPIKey(sourceID) {
			t.Fatalf("%q should require an API key", sourceID)
		}
	}
	if requiresTransparenciaAPIKey(pncp.SourceID) {
		t.Fatal("PNCP should work without an API key")
	}
}
