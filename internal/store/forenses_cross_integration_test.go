package store

import (
	"context"
	"fmt"
	"testing"
)

func TestFindFornecedorDuploPreAggregatesEachSide(t *testing.T) {
	s := integrationTestStore(t, "001_initial.sql", "002_add_pncp.sql", "007_add_contratos.sql", "008_add_cartoes.sql", "010_add_ceap.sql", "011_snapshot_lifecycle.sql", "013_current_projections.sql")
	ctx := context.Background()
	ceapSnap := projectionSnapshot(t, s, "camara-ceap", 2)
	contractSnap := projectionSnapshot(t, s, "pncp-contratos", 3)
	insertDiffEvents(t, s, ceapSnap, diffEvent(ceapSnap, "ceap-1", 1), diffEvent(ceapSnap, "ceap-2", 2))
	insertDiffEvents(t, s, contractSnap, diffEvent(contractSnap, "contract-1", 3), diffEvent(contractSnap, "contract-2", 4), diffEvent(contractSnap, "contract-3", 5))
	if err := s.CompleteSnapshot(ctx, ceapSnap); err != nil {
		t.Fatalf("complete CEAP snapshot: %v", err)
	}
	if err := s.CompleteSnapshot(ctx, contractSnap); err != nil {
		t.Fatalf("complete contract snapshot: %v", err)
	}
	var ceapIDs []int64
	rows, err := s.pool.Query(ctx, `SELECT id FROM events WHERE snapshot_id = $1 ORDER BY id`, ceapSnap)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ceapIDs = append(ceapIDs, id)
	}
	rows.Close()
	for i, value := range []int{100, 200} {
		if _, err := s.pool.Exec(ctx, `INSERT INTO ceap(event_id,snapshot_id,external_id,deputado_nome,partido,uf,fornecedor_cnpj,fornecedor_nome,valor_liquido,collected_at) VALUES ($1,$2,$3,'Dep','P','SP','X','Fornecedor',$4,NOW())`, ceapIDs[i], ceapSnap, fmt.Sprintf("ceap-%d", i), value); err != nil {
			t.Fatal(err)
		}
	}
	var contractIDs []int64
	rows, err = s.pool.Query(ctx, `SELECT id FROM events WHERE snapshot_id = $1 ORDER BY id`, contractSnap)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		contractIDs = append(contractIDs, id)
	}
	rows.Close()
	for i, value := range []int{1000, 2000, 3000} {
		if _, err := s.pool.Exec(ctx, `INSERT INTO contratos(event_id,snapshot_id,external_id,fornecedor_ni,orgao_razao_social,valor_global,collected_at) VALUES ($1,$2,$3,'X','Órgão',$4,NOW())`, contractIDs[i], contractSnap, fmt.Sprintf("contract-%d", i), value); err != nil {
			t.Fatal(err)
		}
	}
	found, err := s.FindFornecedorDuplo(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 {
		t.Fatalf("findings=%d", len(found))
	}
	if got := *found[0].Valor; got != 6300 {
		t.Fatalf("combined total=%v want 6300", got)
	}
	if got := found[0].Evidence["total_ceap"]; got != float64(300) {
		t.Fatalf("ceap=%v want 300", got)
	}
	if got := found[0].Evidence["total_contratos"]; got != float64(6000) {
		t.Fatalf("contracts=%v want 6000", got)
	}
}
