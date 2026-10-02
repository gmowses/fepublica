package store

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func completeProjectionEvent(t *testing.T, s *Store, source, externalID string) (int64, int64) {
	t.Helper()
	ctx := context.Background()
	snapshotID := projectionSnapshot(t, s, source, 1)
	if _, err := s.InsertEventsBatch(ctx, []InsertEventParams{{
		SnapshotID:    snapshotID,
		SourceID:      source,
		ExternalID:    externalID,
		ContentHash:   make([]byte, 32),
		CanonicalJSON: []byte(fmt.Sprintf(`{"id":%q}`, externalID)),
	}}); err != nil {
		t.Fatalf("insert %s event %q: %v", source, externalID, err)
	}
	if err := s.CompleteSnapshot(ctx, snapshotID); err != nil {
		t.Fatalf("complete %s snapshot: %v", source, err)
	}
	var eventID int64
	if err := s.pool.QueryRow(ctx, `SELECT id FROM events WHERE snapshot_id = $1`, snapshotID).Scan(&eventID); err != nil {
		t.Fatalf("get %s event: %v", source, err)
	}
	return snapshotID, eventID
}

func projectionEventAt(t *testing.T, s *Store, source, externalID string, collectedAt time.Time, status string) (int64, int64) {
	t.Helper()
	ctx := context.Background()
	snapshotID, err := s.CreateSnapshotWithCounts(ctx, CreateSnapshotParams{
		SourceID: source, CollectedAt: collectedAt, CollectorVersion: "test",
	}, 1, 0, "")
	if err != nil {
		t.Fatalf("create %s snapshot: %v", source, err)
	}
	if _, err := s.InsertEventsBatch(ctx, []InsertEventParams{{
		SnapshotID:    snapshotID,
		SourceID:      source,
		ExternalID:    externalID,
		ContentHash:   make([]byte, 32),
		CanonicalJSON: []byte(fmt.Sprintf(`{"id":%q}`, externalID)),
	}}); err != nil {
		t.Fatalf("insert %s event %q: %v", source, externalID, err)
	}
	if status == "failed" {
		if err := s.FailSnapshot(ctx, snapshotID); err != nil {
			t.Fatalf("fail %s snapshot: %v", source, err)
		}
	}
	var eventID int64
	if err := s.pool.QueryRow(ctx, `SELECT id FROM events WHERE snapshot_id = $1`, snapshotID).Scan(&eventID); err != nil {
		t.Fatalf("get %s event: %v", source, err)
	}
	return snapshotID, eventID
}

func TestCurrentProjectionsUseLatestVersionAndRetainNewRecords(t *testing.T) {
	s := integrationTestStore(t,
		"001_initial.sql", "002_add_pncp.sql", "007_add_contratos.sql",
		"008_add_cartoes.sql", "010_add_ceap.sql", "011_snapshot_lifecycle.sql",
		"013_current_projections.sql",
	)
	ctx := context.Background()

	contractOldSnapshot, contractOldEvent := completeProjectionEvent(t, s, "pncp-contratos", "contract-same")
	contractNewSnapshot, contractNewEvent := completeProjectionEvent(t, s, "pncp-contratos", "contract-same")
	contractExtraSnapshot, contractExtraEvent := completeProjectionEvent(t, s, "pncp-contratos", "contract-new")
	for _, row := range []struct {
		eventID, snapshotID int64
		externalID          string
		value               int
	}{
		{contractOldEvent, contractOldSnapshot, "contract-same", 100},
		{contractNewEvent, contractNewSnapshot, "contract-same", 120},
		{contractExtraEvent, contractExtraSnapshot, "contract-new", 50},
	} {
		if _, err := s.pool.Exec(ctx, `INSERT INTO contratos (event_id, snapshot_id, external_id, fornecedor_ni, fornecedor_nome, orgao_razao_social, valor_global, collected_at) VALUES ($1, $2, $3, 'supplier', 'Supplier', 'Agency', $4, NOW())`, row.eventID, row.snapshotID, row.externalID, row.value); err != nil {
			t.Fatal(err)
		}
	}
	// Newer incomplete and failed collections remain in the raw projection,
	// but cannot supersede the latest complete version.
	future := time.Now().UTC().Add(time.Hour)
	contractCollectingSnapshot, contractCollectingEvent := projectionEventAt(t, s, "pncp-contratos", "contract-same", future, "collecting")
	contractFailedSnapshot, contractFailedEvent := projectionEventAt(t, s, "pncp-contratos", "contract-same", future.Add(time.Hour), "failed")
	for _, row := range []struct {
		eventID, snapshotID int64
		value               int
	}{
		{contractCollectingEvent, contractCollectingSnapshot, 999},
		{contractFailedEvent, contractFailedSnapshot, 999},
	} {
		if _, err := s.pool.Exec(ctx, `INSERT INTO contratos (event_id, snapshot_id, external_id, fornecedor_ni, fornecedor_nome, orgao_razao_social, valor_global, collected_at) VALUES ($1, $2, 'contract-same', 'supplier', 'Supplier', 'Agency', $3, NOW())`, row.eventID, row.snapshotID, row.value); err != nil {
			t.Fatal(err)
		}
	}
	var currentValue float64
	if err := s.pool.QueryRow(ctx, `SELECT valor_global FROM current_contratos WHERE external_id = 'contract-same'`).Scan(&currentValue); err != nil || currentValue != 120 {
		t.Fatalf("current contract value = %v, %v; want 120", currentValue, err)
	}
	var historicalVersions int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM contratos WHERE external_id = 'contract-same'`).Scan(&historicalVersions); err != nil || historicalVersions != 4 {
		t.Fatalf("historical contract versions = %d, %v; want 4", historicalVersions, err)
	}

	cardOldSnapshot, cardOldEvent := completeProjectionEvent(t, s, "cartoes-cpgf", "card-same")
	cardNewSnapshot, cardNewEvent := completeProjectionEvent(t, s, "cartoes-cpgf", "card-same")
	cardExtraSnapshot, cardExtraEvent := completeProjectionEvent(t, s, "cartoes-cpgf", "card-new")
	for _, row := range []struct {
		eventID, snapshotID int64
		externalID          string
		value               int
	}{
		{cardOldEvent, cardOldSnapshot, "card-same", 100},
		{cardNewEvent, cardNewSnapshot, "card-same", 120},
		{cardExtraEvent, cardExtraSnapshot, "card-new", 30},
	} {
		if _, err := s.pool.Exec(ctx, `INSERT INTO cartoes (event_id, snapshot_id, external_id, portador_cpf, orgao_max_codigo, valor_transacao, collected_at) VALUES ($1, $2, $3, 'holder', 'agency', $4, NOW())`, row.eventID, row.snapshotID, row.externalID, row.value); err != nil {
			t.Fatal(err)
		}
	}

	ceapOldSnapshot, ceapOldEvent := completeProjectionEvent(t, s, "camara-ceap", "ceap-same")
	ceapNewSnapshot, ceapNewEvent := completeProjectionEvent(t, s, "camara-ceap", "ceap-same")
	ceapExtraSnapshot, ceapExtraEvent := completeProjectionEvent(t, s, "camara-ceap", "ceap-new")
	for _, row := range []struct {
		eventID, snapshotID int64
		externalID          string
		value               int
	}{
		{ceapOldEvent, ceapOldSnapshot, "ceap-same", 100},
		{ceapNewEvent, ceapNewSnapshot, "ceap-same", 120},
		{ceapExtraEvent, ceapExtraSnapshot, "ceap-new", 50},
	} {
		if _, err := s.pool.Exec(ctx, `INSERT INTO ceap (event_id, snapshot_id, external_id, deputado_nome, partido, uf, fornecedor_cnpj, fornecedor_nome, valor_liquido, collected_at) VALUES ($1, $2, $3, 'Deputy', 'P', 'SP', 'supplier', 'Supplier', $4, NOW())`, row.eventID, row.snapshotID, row.externalID, row.value); err != nil {
			t.Fatal(err)
		}
	}

	contracts, err := s.GetGastosStats(ctx)
	if err != nil || contracts.TotalContratos != 2 || contracts.ValorTotalGlobal != 170 {
		t.Fatalf("contract stats = %#v, %v; want count 2 total 170", contracts, err)
	}
	cards, err := s.GetCartoesStats(ctx)
	if err != nil || cards.TotalTransacoes != 2 || cards.ValorTotal != 150 {
		t.Fatalf("card stats = %#v, %v; want count 2 total 150", cards, err)
	}
	ceap, err := s.GetCEAPStats(ctx)
	if err != nil || ceap.TotalDespesas != 2 || ceap.ValorTotal != 170 {
		t.Fatalf("ceap stats = %#v, %v; want count 2 total 170", ceap, err)
	}

	found, err := s.FindFornecedorDuplo(ctx, 10)
	if err != nil || len(found) != 1 || *found[0].Valor != 340 {
		t.Fatalf("fornecedor duplo = %#v, %v; want one finding totaling 340", found, err)
	}
}
