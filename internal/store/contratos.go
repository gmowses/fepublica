package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gmowses/fepublica/internal/contratos"
)

// InsertContrato persists a projected PNCP contract row.
func (s *Store) InsertContrato(ctx context.Context, eventID, snapshotID int64, r *contratos.Row, collectedAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO contratos (
			event_id, snapshot_id, external_id, numero_controle_pncp,
			orgao_cnpj, orgao_razao_social, orgao_poder_id, orgao_esfera_id, uf,
			fornecedor_ni, fornecedor_nome, fornecedor_tipo,
			valor_inicial, valor_global, valor_acumulado,
			data_assinatura, data_vigencia_inicio, data_vigencia_fim, data_publicacao_pncp,
			objeto_contrato, tipo_contrato, categoria_processo,
			collected_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8, $9,
			$10, $11, $12,
			$13, $14, $15,
			$16, $17, $18, $19,
			$20, $21, $22,
			$23
		)
		ON CONFLICT (snapshot_id, external_id) DO NOTHING
	`,
		eventID, snapshotID, r.ExternalID, nullIfEmpty(r.NumeroControlePNCP),
		nullIfEmpty(r.OrgaoCNPJ), nullIfEmpty(r.OrgaoRazaoSocial),
		nullIfEmpty(r.OrgaoPoderID), nullIfEmpty(r.OrgaoEsferaID),
		nullIfEmpty(r.UF),
		nullIfEmpty(r.FornecedorNI), nullIfEmpty(r.FornecedorNome),
		nullIfEmpty(r.FornecedorTipo),
		r.ValorInicial, r.ValorGlobal, r.ValorAcumulado,
		r.DataAssinatura, r.DataVigenciaInicio, r.DataVigenciaFim, r.DataPublicacaoPNCP,
		nullIfEmpty(r.ObjetoContrato), nullIfEmpty(r.TipoContrato), nullIfEmpty(r.CategoriaProcesso),
		collectedAt,
	)
	if err != nil {
		return fmt.Errorf("store: insert contrato: %w", err)
	}
	return nil
}

// ListUnindexedPNCPEvents returns PNCP events that haven't been projected
// into contratos yet, up to limit, oldest first.
func (s *Store) ListUnindexedPNCPEvents(ctx context.Context, limit int) ([]Event, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := s.pool.Query(ctx, `
		SELECT e.id, e.snapshot_id, e.source_id, e.external_id, e.content_hash, e.canonical_json, e.collected_at
		FROM events e
		LEFT JOIN contratos c ON c.event_id = e.id
		WHERE e.source_id = 'pncp-contratos'
		  AND c.id IS NULL
		  AND e.canonical_json IS NOT NULL
		ORDER BY e.id ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("store: list unindexed pncp: %w", err)
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

// ContratoSummary is the card shape for list views.
type ContratoSummary struct {
	ID                int64
	ExternalID        string
	OrgaoCNPJ         string
	OrgaoRazaoSocial  string
	UF                string
	FornecedorNI      string
	FornecedorNome    string
	ValorGlobal       *float64
	DataAssinatura    *time.Time
	ObjetoContrato    string
	// FornecedorSancionado is true if the fornecedor_ni matches any CEIS or
	// CNEP record by content_hash presence. Cheap LEFT JOIN indicator.
	FornecedorSancionado bool
}

// ListContratosParams filters the contratos listing.
type ListContratosParams struct {
	Search    string // ILIKE across fornecedor_nome, orgao_razao_social, objeto_contrato
	UF        string
	MinValor  float64
	MaxValor  float64
	Limit     int
	Offset    int
	OrderBy   string // "valor_desc", "data_desc" (default)
}

// ListContratos returns a page of contratos with basic filters.
func (s *Store) ListContratos(ctx context.Context, p ListContratosParams) ([]ContratoSummary, int, error) {
	if p.Limit <= 0 || p.Limit > 500 {
		p.Limit = 50
	}
	conds := []string{"1=1"}
	args := []any{}
	if p.Search != "" {
		args = append(args, "%"+p.Search+"%")
		conds = append(conds, fmt.Sprintf(
			"(fornecedor_nome ILIKE $%d OR orgao_razao_social ILIKE $%d OR objeto_contrato ILIKE $%d OR fornecedor_ni LIKE $%d OR orgao_cnpj LIKE $%d)",
			len(args), len(args), len(args), len(args), len(args)))
	}
	if p.UF != "" {
		args = append(args, strings.ToUpper(p.UF))
		conds = append(conds, fmt.Sprintf("uf = $%d", len(args)))
	}
	if p.MinValor > 0 {
		args = append(args, p.MinValor)
		conds = append(conds, fmt.Sprintf("valor_global >= $%d", len(args)))
	}
	if p.MaxValor > 0 {
		args = append(args, p.MaxValor)
		conds = append(conds, fmt.Sprintf("valor_global <= $%d", len(args)))
	}
	where := strings.Join(conds, " AND ")

	var total int
	if err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM contratos WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	order := "data_assinatura DESC NULLS LAST, id DESC"
	if p.OrderBy == "valor_desc" {
		order = "valor_global DESC NULLS LAST, id DESC"
	}

	args = append(args, p.Limit, p.Offset)
	q := fmt.Sprintf(`
		SELECT c.id, c.external_id,
		       COALESCE(c.orgao_cnpj, ''), COALESCE(c.orgao_razao_social, ''),
		       COALESCE(c.uf, ''),
		       COALESCE(c.fornecedor_ni, ''), COALESCE(c.fornecedor_nome, ''),
		       c.valor_global, c.data_assinatura,
		       COALESCE(c.objeto_contrato, ''),
		       EXISTS (
		          SELECT 1 FROM events ev
		          WHERE ev.source_id IN ('ceis', 'cnep')
		            AND ev.canonical_json::text ILIKE '%%' || c.fornecedor_ni || '%%'
		            AND c.fornecedor_ni IS NOT NULL
		          LIMIT 1
		       ) AS fornecedor_sancionado
		FROM contratos c
		WHERE %s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, where, order, len(args)-1, len(args))

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []ContratoSummary
	for rows.Next() {
		var c ContratoSummary
		if err := rows.Scan(&c.ID, &c.ExternalID, &c.OrgaoCNPJ, &c.OrgaoRazaoSocial,
			&c.UF, &c.FornecedorNI, &c.FornecedorNome, &c.ValorGlobal,
			&c.DataAssinatura, &c.ObjetoContrato, &c.FornecedorSancionado); err != nil {
			return nil, 0, err
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}

// GastosStats are the hero numbers for the /gastos landing.
type GastosStats struct {
	TotalContratos   int     `json:"total_contratos"`
	ValorTotalGlobal float64 `json:"valor_total_global"`
	OrgaosUnicos     int     `json:"orgaos_unicos"`
	FornecedoresUnicos int   `json:"fornecedores_unicos"`
}

// GetGastosStats aggregates top-level numbers.
func (s *Store) GetGastosStats(ctx context.Context) (*GastosStats, error) {
	var st GastosStats
	err := s.pool.QueryRow(ctx, `
		SELECT
		  COUNT(*),
		  COALESCE(SUM(valor_global), 0),
		  COUNT(DISTINCT orgao_cnpj),
		  COUNT(DISTINCT fornecedor_ni)
		FROM contratos
	`).Scan(&st.TotalContratos, &st.ValorTotalGlobal, &st.OrgaosUnicos, &st.FornecedoresUnicos)
	if err != nil {
		return nil, err
	}
	return &st, nil
}

// TopRow is a single entry in a "top N" list.
type TopRow struct {
	Key   string  `json:"key"`
	Nome  string  `json:"nome"`
	Count int     `json:"count"`
	Total float64 `json:"total_valor"`
}

// TopFornecedores returns the top N suppliers by sum of valor_global.
func (s *Store) TopFornecedores(ctx context.Context, limit int) ([]TopRow, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.pool.Query(ctx, `
		SELECT COALESCE(fornecedor_ni, '') AS ni,
		       COALESCE(MIN(fornecedor_nome), '') AS nome,
		       COUNT(*) AS n,
		       COALESCE(SUM(valor_global), 0) AS total
		FROM contratos
		WHERE fornecedor_ni IS NOT NULL
		GROUP BY fornecedor_ni
		ORDER BY total DESC NULLS LAST
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TopRow
	for rows.Next() {
		var t TopRow
		if err := rows.Scan(&t.Key, &t.Nome, &t.Count, &t.Total); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// TopOrgaos returns the top N buyer agencies by sum of valor_global.
func (s *Store) TopOrgaos(ctx context.Context, limit int) ([]TopRow, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.pool.Query(ctx, `
		SELECT COALESCE(orgao_cnpj, '') AS cnpj,
		       COALESCE(MIN(orgao_razao_social), '') AS nome,
		       COUNT(*) AS n,
		       COALESCE(SUM(valor_global), 0) AS total
		FROM contratos
		WHERE orgao_cnpj IS NOT NULL
		GROUP BY orgao_cnpj
		ORDER BY total DESC NULLS LAST
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TopRow
	for rows.Next() {
		var t TopRow
		if err := rows.Scan(&t.Key, &t.Nome, &t.Count, &t.Total); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// FornecedorDetail is the aggregated view for a supplier page.
type FornecedorDetail struct {
	NI             string  `json:"ni"`
	Nome           string  `json:"nome"`
	TotalContratos int     `json:"total_contratos"`
	ValorTotal     float64 `json:"valor_total"`
	Sancionado     bool    `json:"sancionado"`
}

// GetFornecedor returns the aggregated view for one supplier.
func (s *Store) GetFornecedor(ctx context.Context, ni string) (*FornecedorDetail, error) {
	var f FornecedorDetail
	f.NI = ni
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(MIN(fornecedor_nome), ''),
		       COUNT(*),
		       COALESCE(SUM(valor_global), 0),
		       EXISTS (
		         SELECT 1 FROM events ev
		         WHERE ev.source_id IN ('ceis', 'cnep')
		           AND ev.canonical_json::text ILIKE '%' || $1 || '%'
		         LIMIT 1
		       )
		FROM contratos
		WHERE fornecedor_ni = $1
	`, ni).Scan(&f.Nome, &f.TotalContratos, &f.ValorTotal, &f.Sancionado)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// TimeseriesPoint is one point in the gastos timeline.
type TimeseriesPoint struct {
	Date  string  `json:"date"`
	Count int     `json:"count"`
	Total float64 `json:"total"`
}

// GastosTimeseries returns daily sums of valor_global for the last 90 days.
func (s *Store) GastosTimeseries(ctx context.Context) ([]TimeseriesPoint, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT to_char(data_assinatura, 'YYYY-MM-DD') AS d,
		       COUNT(*) AS n,
		       COALESCE(SUM(valor_global), 0) AS total
		FROM contratos
		WHERE data_assinatura IS NOT NULL
		  AND data_assinatura >= CURRENT_DATE - INTERVAL '90 days'
		GROUP BY d
		ORDER BY d ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TimeseriesPoint
	for rows.Next() {
		var p TimeseriesPoint
		if err := rows.Scan(&p.Date, &p.Count, &p.Total); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListContratosByFornecedor returns all contracts of one supplier.
func (s *Store) ListContratosByFornecedor(ctx context.Context, ni string, limit int) ([]ContratoSummary, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, external_id,
		       COALESCE(orgao_cnpj, ''), COALESCE(orgao_razao_social, ''),
		       COALESCE(uf, ''),
		       COALESCE(fornecedor_ni, ''), COALESCE(fornecedor_nome, ''),
		       valor_global, data_assinatura,
		       COALESCE(objeto_contrato, ''),
		       FALSE
		FROM contratos
		WHERE fornecedor_ni = $1
		ORDER BY data_assinatura DESC NULLS LAST
		LIMIT $2
	`, ni, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ContratoSummary
	for rows.Next() {
		var c ContratoSummary
		if err := rows.Scan(&c.ID, &c.ExternalID, &c.OrgaoCNPJ, &c.OrgaoRazaoSocial,
			&c.UF, &c.FornecedorNI, &c.FornecedorNome, &c.ValorGlobal,
			&c.DataAssinatura, &c.ObjetoContrato, &c.FornecedorSancionado); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
