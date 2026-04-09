package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Ente is a public body (União, state, municipality, órgão, etc.).
type Ente struct {
	ID         string
	Nome       string
	NomeCurto  string
	Esfera     string
	Tipo       string
	Poder      string
	UF         string
	IBGECode   string
	CNPJ       string
	Populacao  int
	DomainHint string
	ParentID   string
	Tier       int
	Active     bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// UpsertEnteParams are the fields allowed to be inserted/updated.
type UpsertEnteParams struct {
	ID         string
	Nome       string
	NomeCurto  string
	Esfera     string
	Tipo       string
	Poder      string
	UF         string
	IBGECode   string
	CNPJ       string
	Populacao  int
	DomainHint string
	ParentID   string
	Tier       int
}

// UpsertEnte inserts or updates a single ente by id.
func (s *Store) UpsertEnte(ctx context.Context, p UpsertEnteParams) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO entes (
			id, nome, nome_curto, esfera, tipo, poder, uf,
			ibge_code, cnpj, populacao, domain_hint, parent_id, tier
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (id) DO UPDATE SET
			nome = EXCLUDED.nome,
			nome_curto = EXCLUDED.nome_curto,
			esfera = EXCLUDED.esfera,
			tipo = EXCLUDED.tipo,
			poder = EXCLUDED.poder,
			uf = EXCLUDED.uf,
			ibge_code = EXCLUDED.ibge_code,
			cnpj = EXCLUDED.cnpj,
			populacao = EXCLUDED.populacao,
			domain_hint = EXCLUDED.domain_hint,
			parent_id = EXCLUDED.parent_id,
			tier = EXCLUDED.tier,
			updated_at = NOW()
	`,
		p.ID, p.Nome, nullIfEmpty(p.NomeCurto), p.Esfera, p.Tipo,
		nullIfEmpty(p.Poder), nullIfEmpty(p.UF),
		nullIfEmpty(p.IBGECode), nullIfEmpty(p.CNPJ),
		nullIfZero(p.Populacao), nullIfEmpty(p.DomainHint), nullIfEmpty(p.ParentID),
		p.Tier,
	)
	if err != nil {
		return fmt.Errorf("store: upsert ente %s: %w", p.ID, err)
	}
	return nil
}

func nullIfZero(n int) any {
	if n == 0 {
		return nil
	}
	return n
}

// ListEntesParams filters the entes listing.
type ListEntesParams struct {
	Esfera string
	UF     string
	Tier   int
	Search string
	Limit  int
	Offset int
}

// ListEntes returns a paginated list of entes with filters.
func (s *Store) ListEntes(ctx context.Context, p ListEntesParams) ([]Ente, int, error) {
	if p.Limit <= 0 || p.Limit > 1000 {
		p.Limit = 100
	}
	conds := []string{"1=1"}
	args := []any{}
	if p.Esfera != "" {
		args = append(args, p.Esfera)
		conds = append(conds, fmt.Sprintf("esfera = $%d", len(args)))
	}
	if p.UF != "" {
		args = append(args, strings.ToUpper(p.UF))
		conds = append(conds, fmt.Sprintf("uf = $%d", len(args)))
	}
	if p.Tier > 0 {
		args = append(args, p.Tier)
		conds = append(conds, fmt.Sprintf("tier = $%d", len(args)))
	}
	if p.Search != "" {
		args = append(args, "%"+p.Search+"%")
		conds = append(conds, fmt.Sprintf("(nome ILIKE $%d OR nome_curto ILIKE $%d)", len(args), len(args)))
	}
	where := strings.Join(conds, " AND ")

	var total int
	if err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM entes WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("store: count entes: %w", err)
	}

	args = append(args, p.Limit, p.Offset)
	q := fmt.Sprintf(`
		SELECT id, nome, COALESCE(nome_curto, ''), esfera, tipo,
		       COALESCE(poder, ''), COALESCE(uf, ''),
		       COALESCE(ibge_code, ''), COALESCE(cnpj, ''),
		       COALESCE(populacao, 0), COALESCE(domain_hint, ''),
		       COALESCE(parent_id, ''), tier, active, created_at, updated_at
		FROM entes
		WHERE %s
		ORDER BY esfera, uf, nome
		LIMIT $%d OFFSET $%d
	`, where, len(args)-1, len(args))

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("store: list entes: %w", err)
	}
	defer rows.Close()
	var out []Ente
	for rows.Next() {
		var e Ente
		if err := rows.Scan(
			&e.ID, &e.Nome, &e.NomeCurto, &e.Esfera, &e.Tipo,
			&e.Poder, &e.UF, &e.IBGECode, &e.CNPJ,
			&e.Populacao, &e.DomainHint, &e.ParentID,
			&e.Tier, &e.Active, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("store: scan ente: %w", err)
		}
		out = append(out, e)
	}
	return out, total, rows.Err()
}

// GetEnte fetches a single ente by id.
func (s *Store) GetEnte(ctx context.Context, id string) (*Ente, error) {
	var e Ente
	err := s.pool.QueryRow(ctx, `
		SELECT id, nome, COALESCE(nome_curto, ''), esfera, tipo,
		       COALESCE(poder, ''), COALESCE(uf, ''),
		       COALESCE(ibge_code, ''), COALESCE(cnpj, ''),
		       COALESCE(populacao, 0), COALESCE(domain_hint, ''),
		       COALESCE(parent_id, ''), tier, active, created_at, updated_at
		FROM entes WHERE id = $1
	`, id).Scan(
		&e.ID, &e.Nome, &e.NomeCurto, &e.Esfera, &e.Tipo,
		&e.Poder, &e.UF, &e.IBGECode, &e.CNPJ,
		&e.Populacao, &e.DomainHint, &e.ParentID,
		&e.Tier, &e.Active, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("store: ente %s not found", id)
		}
		return nil, fmt.Errorf("store: get ente: %w", err)
	}
	return &e, nil
}

// CountEntes returns the total row count.
func (s *Store) CountEntes(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM entes").Scan(&n)
	return n, err
}
