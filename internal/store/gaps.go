package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Gap mirrors gap_catalog rows.
type Gap struct {
	ID             int64
	EnteID         *string
	SourceID       *string
	Category       string
	Title          string
	Description    string
	Severity       string
	LegalReference *string
	Status         string
	FirstSeenAt    time.Time
	LastSeenAt     time.Time
	ResolvedAt     *time.Time
}

// InsertGapParams are required fields for creating a gap.
type InsertGapParams struct {
	EnteID         string
	SourceID       string
	Category       string
	Title          string
	Description    string
	Severity       string
	LegalReference string
}

// InsertGap creates a new gap_catalog row.
func (s *Store) InsertGap(ctx context.Context, p InsertGapParams) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO gap_catalog (ente_id, source_id, category, title, description, severity, legal_reference)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`, nullIfEmpty(p.EnteID), nullIfEmpty(p.SourceID), p.Category, p.Title, p.Description,
		p.Severity, nullIfEmpty(p.LegalReference)).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("store: insert gap: %w", err)
	}
	return id, nil
}

// ListGapsParams filters the gap catalog.
type ListGapsParams struct {
	Status   string
	Category string
	EnteID   string
	Limit    int
	Offset   int
}

// ListGaps returns a paginated slice of gaps.
func (s *Store) ListGaps(ctx context.Context, p ListGapsParams) ([]Gap, int, error) {
	if p.Limit <= 0 {
		p.Limit = 50
	}
	conds := []string{"1=1"}
	args := []any{}
	if p.Status != "" {
		args = append(args, p.Status)
		conds = append(conds, fmt.Sprintf("status = $%d", len(args)))
	}
	if p.Category != "" {
		args = append(args, p.Category)
		conds = append(conds, fmt.Sprintf("category = $%d", len(args)))
	}
	if p.EnteID != "" {
		args = append(args, p.EnteID)
		conds = append(conds, fmt.Sprintf("ente_id = $%d", len(args)))
	}
	where := strings.Join(conds, " AND ")

	var total int
	if err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM gap_catalog WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, p.Limit, p.Offset)
	q := fmt.Sprintf(`
		SELECT id, ente_id, source_id, category, title, description,
		       severity, legal_reference, status, first_seen_at, last_seen_at, resolved_at
		FROM gap_catalog
		WHERE %s
		ORDER BY last_seen_at DESC
		LIMIT $%d OFFSET $%d
	`, where, len(args)-1, len(args))

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Gap
	for rows.Next() {
		var g Gap
		if err := rows.Scan(&g.ID, &g.EnteID, &g.SourceID, &g.Category, &g.Title,
			&g.Description, &g.Severity, &g.LegalReference, &g.Status,
			&g.FirstSeenAt, &g.LastSeenAt, &g.ResolvedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, g)
	}
	return out, total, rows.Err()
}
