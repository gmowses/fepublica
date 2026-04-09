package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// LaiCheck mirrors the lai_checks row.
type LaiCheck struct {
	ID             int64
	EnteID         string
	CheckedAt      time.Time
	TargetURL      string
	HTTPStatus     int
	ResponseMS     int
	SSLValid       bool
	SSLExpiresAt   *time.Time
	PortalLoads    bool
	HTMLSizeBytes  int
	TermsFound     map[string]bool
	RequiredLinks  map[string]bool
	HTMLArchiveKey string
	Errors         []string
	TierAtCheck    int
}

// InsertLaiCheck persists a new lai_check row and returns its id.
func (s *Store) InsertLaiCheck(ctx context.Context, c *LaiCheck) (int64, error) {
	terms, _ := json.Marshal(c.TermsFound)
	links, _ := json.Marshal(c.RequiredLinks)
	var id int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO lai_checks (
			ente_id, checked_at, target_url, http_status, response_ms,
			ssl_valid, ssl_expires_at, portal_loads, html_size_bytes,
			terms_found, required_links, html_archive_key, errors, tier_at_check
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12, $13, $14
		) RETURNING id
	`, c.EnteID, c.CheckedAt, c.TargetURL, nullIfZero(c.HTTPStatus), c.ResponseMS,
		c.SSLValid, c.SSLExpiresAt, c.PortalLoads, c.HTMLSizeBytes,
		string(terms), string(links), nullIfEmpty(c.HTMLArchiveKey), c.Errors, c.TierAtCheck,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("store: insert lai_check: %w", err)
	}
	return id, nil
}

// UpsertLaiScore updates the rolling score for an ente.
func (s *Store) UpsertLaiScore(ctx context.Context, enteID string, score float64, components map[string]float64, checkID int64) error {
	comp, _ := json.Marshal(components)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO lai_scores (ente_id, score, last_check_id, last_calculated, components)
		VALUES ($1, $2, $3, NOW(), $4)
		ON CONFLICT (ente_id) DO UPDATE SET
			score = EXCLUDED.score,
			last_check_id = EXCLUDED.last_check_id,
			last_calculated = NOW(),
			components = EXCLUDED.components
	`, enteID, score, checkID, string(comp))
	return err
}

// ListLaiChecksByEnte returns check history for one ente, newest first.
func (s *Store) ListLaiChecksByEnte(ctx context.Context, enteID string, limit int) ([]LaiCheck, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, ente_id, checked_at, target_url, COALESCE(http_status, 0),
		       response_ms, ssl_valid, ssl_expires_at, portal_loads,
		       html_size_bytes, terms_found, required_links,
		       COALESCE(html_archive_key, ''), errors, tier_at_check
		FROM lai_checks
		WHERE ente_id = $1
		ORDER BY checked_at DESC
		LIMIT $2
	`, enteID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LaiCheck
	for rows.Next() {
		var c LaiCheck
		var terms, links []byte
		if err := rows.Scan(
			&c.ID, &c.EnteID, &c.CheckedAt, &c.TargetURL, &c.HTTPStatus,
			&c.ResponseMS, &c.SSLValid, &c.SSLExpiresAt, &c.PortalLoads,
			&c.HTMLSizeBytes, &terms, &links,
			&c.HTMLArchiveKey, &c.Errors, &c.TierAtCheck,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(terms, &c.TermsFound)
		_ = json.Unmarshal(links, &c.RequiredLinks)
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListLaiScores returns the current lai_scores leaderboard, sorted by score desc.
func (s *Store) ListLaiScores(ctx context.Context, limit int) ([]LaiScoreRow, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		SELECT ls.ente_id, e.nome, COALESCE(e.uf, ''), e.esfera, ls.score, ls.last_calculated
		FROM lai_scores ls
		JOIN entes e ON e.id = ls.ente_id
		ORDER BY ls.score DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LaiScoreRow
	for rows.Next() {
		var r LaiScoreRow
		if err := rows.Scan(&r.EnteID, &r.Nome, &r.UF, &r.Esfera, &r.Score, &r.LastCalculated); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// LaiScoreRow is the joined projection used by the scores endpoint.
type LaiScoreRow struct {
	EnteID         string
	Nome           string
	UF             string
	Esfera         string
	Score          float64
	LastCalculated time.Time
}

// ListEntesForCrawl returns active entes with a non-empty domain_hint ready to
// be checked, up to limit. Filters by tier so scheduling can pick the right
// cohort (tier 1 every day, tier 2 weekly, etc).
func (s *Store) ListEntesForCrawl(ctx context.Context, tier int, limit int) ([]Ente, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, nome, COALESCE(nome_curto, ''), esfera, tipo,
		       COALESCE(poder, ''), COALESCE(uf, ''),
		       COALESCE(ibge_code, ''), COALESCE(cnpj, ''),
		       COALESCE(populacao, 0), COALESCE(domain_hint, ''),
		       COALESCE(parent_id, ''), tier, active, created_at, updated_at
		FROM entes
		WHERE active = TRUE
		  AND domain_hint IS NOT NULL AND domain_hint != ''
		  AND ($1 = 0 OR tier = $1)
		ORDER BY tier ASC, id ASC
		LIMIT $2
	`, tier, limit)
	if err != nil {
		return nil, err
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
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
