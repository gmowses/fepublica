package store

import (
	"context"
	"fmt"
	"time"

	"github.com/gmowses/fepublica/internal/ceap"
)

func (s *Store) InsertCEAP(ctx context.Context, eventID, snapshotID int64, r *ceap.Row, collectedAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO ceap (
			event_id, snapshot_id, external_id,
			deputado_id, deputado_nome, partido, uf,
			ano, mes, data_documento,
			tipo_despesa, fornecedor_cnpj, fornecedor_nome,
			valor_documento, valor_liquido, valor_glosa, url_documento,
			collected_at
		) VALUES (
			$1, $2, $3,
			$4, $5, $6, $7,
			$8, $9, $10,
			$11, $12, $13,
			$14, $15, $16, $17,
			$18
		)
		ON CONFLICT (snapshot_id, external_id) DO NOTHING
	`,
		eventID, snapshotID, r.ExternalID,
		nullIfZero(r.DeputadoID), nullIfEmpty(r.DeputadoNome), nullIfEmpty(r.Partido), nullIfEmpty(r.UF),
		nullIfZero(r.Ano), nullIfZero(r.Mes), r.DataDocumento,
		nullIfEmpty(r.TipoDespesa), nullIfEmpty(r.FornecedorCNPJ), nullIfEmpty(r.FornecedorNome),
		r.ValorDocumento, r.ValorLiquido, r.ValorGlosa, nullIfEmpty(r.URLDocumento),
		collectedAt,
	)
	if err != nil {
		return fmt.Errorf("store: insert ceap: %w", err)
	}
	return nil
}

func (s *Store) ListUnindexedCEAPEvents(ctx context.Context, limit int) ([]Event, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := s.pool.Query(ctx, `
		SELECT e.id, e.snapshot_id, e.source_id, e.external_id, e.content_hash, e.canonical_json, e.collected_at
		FROM events e
		LEFT JOIN ceap c ON c.event_id = e.id
		WHERE e.source_id = 'camara-ceap'
		  AND c.id IS NULL
		  AND e.canonical_json IS NOT NULL
		ORDER BY e.id ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("store: list unindexed ceap: %w", err)
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var ev Event
		if err := rows.Scan(&ev.ID, &ev.SnapshotID, &ev.SourceID, &ev.ExternalID,
			&ev.ContentHash, &ev.CanonicalJSON, &ev.CollectedAt); err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

// CEAPStats summarizes the CEAP table.
type CEAPStats struct {
	TotalDespesas    int64   `json:"total_despesas"`
	ValorTotal       float64 `json:"valor_total"`
	DeputadosUnicos  int64   `json:"deputados_unicos"`
	FornecedoresUnicos int64 `json:"fornecedores_unicos"`
}

func (s *Store) GetCEAPStats(ctx context.Context) (*CEAPStats, error) {
	var st CEAPStats
	err := s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM(valor_liquido), 0),
			COUNT(DISTINCT deputado_id),
			COUNT(DISTINCT fornecedor_cnpj)
		FROM ceap
	`).Scan(&st.TotalDespesas, &st.ValorTotal, &st.DeputadosUnicos, &st.FornecedoresUnicos)
	return &st, err
}

