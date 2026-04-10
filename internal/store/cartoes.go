package store

import (
	"context"
	"fmt"
	"time"

	"github.com/gmowses/fepublica/internal/cartoes"
)

// InsertCartao persists a projected CPGF row.
func (s *Store) InsertCartao(ctx context.Context, eventID, snapshotID int64, r *cartoes.Row, collectedAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO cartoes (
			event_id, snapshot_id, external_id,
			tipo_cartao, mes_extrato, data_transacao, valor_transacao,
			estab_cnpj, estab_nome, estab_tipo,
			portador_cpf, portador_nome,
			orgao_codigo, orgao_sigla, orgao_nome,
			orgao_max_codigo, orgao_max_sigla, orgao_max_nome,
			unidade_codigo, unidade_nome,
			collected_at
		) VALUES (
			$1, $2, $3,
			$4, $5, $6, $7,
			$8, $9, $10,
			$11, $12,
			$13, $14, $15,
			$16, $17, $18,
			$19, $20,
			$21
		)
		ON CONFLICT (snapshot_id, external_id) DO NOTHING
	`,
		eventID, snapshotID, r.ExternalID,
		nullIfEmpty(r.TipoCartao), nullIfEmpty(r.MesExtrato), r.DataTransacao, r.ValorTransacao,
		nullIfEmpty(r.EstabCNPJ), nullIfEmpty(r.EstabNome), nullIfEmpty(r.EstabTipo),
		nullIfEmpty(r.PortadorCPF), nullIfEmpty(r.PortadorNome),
		nullIfEmpty(r.OrgaoCodigo), nullIfEmpty(r.OrgaoSigla), nullIfEmpty(r.OrgaoNome),
		nullIfEmpty(r.OrgaoMaxCodigo), nullIfEmpty(r.OrgaoMaxSigla), nullIfEmpty(r.OrgaoMaxNome),
		nullIfEmpty(r.UnidadeCodigo), nullIfEmpty(r.UnidadeNome),
		collectedAt,
	)
	if err != nil {
		return fmt.Errorf("store: insert cartao: %w", err)
	}
	return nil
}

// ListUnindexedCartoesEvents returns CPGF events not yet projected.
func (s *Store) ListUnindexedCartoesEvents(ctx context.Context, limit int) ([]Event, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := s.pool.Query(ctx, `
		SELECT e.id, e.snapshot_id, e.source_id, e.external_id, e.content_hash, e.canonical_json, e.collected_at
		FROM events e
		LEFT JOIN cartoes c ON c.event_id = e.id
		WHERE e.source_id = 'cartoes-cpgf'
		  AND c.id IS NULL
		  AND e.canonical_json IS NOT NULL
		ORDER BY e.id ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("store: list unindexed cartoes: %w", err)
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

// CartoesStats summarizes the CPGF table.
type CartoesStats struct {
	TotalTransacoes int64   `json:"total_transacoes"`
	ValorTotal      float64 `json:"valor_total"`
	OrgaosUnicos    int64   `json:"orgaos_unicos"`
	PortadoresUnicos int64  `json:"portadores_unicos"`
}

// GetCartoesStats returns aggregate stats for the CPGF projection.
func (s *Store) GetCartoesStats(ctx context.Context) (*CartoesStats, error) {
	var st CartoesStats
	err := s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM(valor_transacao), 0),
			COUNT(DISTINCT orgao_max_codigo),
			COUNT(DISTINCT portador_cpf)
		FROM cartoes
	`).Scan(&st.TotalTransacoes, &st.ValorTotal, &st.OrgaosUnicos, &st.PortadoresUnicos)
	if err != nil {
		return nil, fmt.Errorf("store: cartoes stats: %w", err)
	}
	return &st, nil
}

// CartoesTopRow is a generic top-N row for portadores or órgãos.
type CartoesTopRow struct {
	Key        string  `json:"key"`
	Nome       string  `json:"nome"`
	Count      int64   `json:"count"`
	TotalValor float64 `json:"total_valor"`
}

// TopPortadores returns the top portadores by total CPGF spend.
func (s *Store) TopPortadores(ctx context.Context, limit int) ([]CartoesTopRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	rows, err := s.pool.Query(ctx, `
		SELECT
			COALESCE(portador_cpf, '') AS key,
			COALESCE(MIN(portador_nome), '') AS nome,
			COUNT(*) AS qtd,
			COALESCE(SUM(valor_transacao), 0) AS total
		FROM cartoes
		WHERE portador_cpf IS NOT NULL AND portador_cpf <> ''
		GROUP BY portador_cpf
		ORDER BY total DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("store: top portadores: %w", err)
	}
	defer rows.Close()
	var out []CartoesTopRow
	for rows.Next() {
		var r CartoesTopRow
		if err := rows.Scan(&r.Key, &r.Nome, &r.Count, &r.TotalValor); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// TopOrgaosCartoes returns top órgãos máximos by total CPGF spend.
func (s *Store) TopOrgaosCartoes(ctx context.Context, limit int) ([]CartoesTopRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	rows, err := s.pool.Query(ctx, `
		SELECT
			COALESCE(orgao_max_codigo, '') AS key,
			COALESCE(MIN(orgao_max_nome), '') AS nome,
			COUNT(*) AS qtd,
			COALESCE(SUM(valor_transacao), 0) AS total
		FROM cartoes
		WHERE orgao_max_codigo IS NOT NULL AND orgao_max_codigo <> ''
		GROUP BY orgao_max_codigo
		ORDER BY total DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("store: top orgaos cartoes: %w", err)
	}
	defer rows.Close()
	var out []CartoesTopRow
	for rows.Next() {
		var r CartoesTopRow
		if err := rows.Scan(&r.Key, &r.Nome, &r.Count, &r.TotalValor); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// CartaoSummary is the card shape for transaction listings.
type CartaoSummary struct {
	ID             int64
	ExternalID     string
	DataTransacao  *time.Time
	ValorTransacao *float64
	EstabNome      string
	EstabCNPJ      string
	PortadorNome   string
	PortadorCPF    string
	OrgaoMaxNome   string
	OrgaoMaxSigla  string
	UnidadeNome    string
}

// ListCartoes returns a paginated list of CPGF transactions, optionally
// filtered by free-text search on portador or estabelecimento.
func (s *Store) ListCartoes(ctx context.Context, search string, limit int) ([]CartaoSummary, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var (
		args  []interface{}
		where = "WHERE 1=1"
		i     = 1
	)
	if search != "" {
		where += fmt.Sprintf(" AND (portador_nome ILIKE $%d OR estab_nome ILIKE $%d)", i, i+1)
		args = append(args, "%"+search+"%", "%"+search+"%")
		i += 2
	}

	var total int64
	if err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM cartoes "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("store: count cartoes: %w", err)
	}

	args = append(args, limit)
	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, external_id, data_transacao, valor_transacao,
			COALESCE(estab_nome, ''), COALESCE(estab_cnpj, ''),
			COALESCE(portador_nome, ''), COALESCE(portador_cpf, ''),
			COALESCE(orgao_max_nome, ''), COALESCE(orgao_max_sigla, ''),
			COALESCE(unidade_nome, '')
		FROM cartoes
		%s
		ORDER BY valor_transacao DESC NULLS LAST
		LIMIT $%d
	`, where, i), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("store: list cartoes: %w", err)
	}
	defer rows.Close()
	var out []CartaoSummary
	for rows.Next() {
		var c CartaoSummary
		if err := rows.Scan(&c.ID, &c.ExternalID, &c.DataTransacao, &c.ValorTransacao,
			&c.EstabNome, &c.EstabCNPJ,
			&c.PortadorNome, &c.PortadorCPF,
			&c.OrgaoMaxNome, &c.OrgaoMaxSigla,
			&c.UnidadeNome); err != nil {
			return nil, 0, err
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}
