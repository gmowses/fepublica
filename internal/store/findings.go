package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// PersistFindings inserts or updates findings by dedup_key. New findings get
// first_seen_at = NOW(); already-seen findings have last_seen_at bumped.
//
// Returns the IDs of findings that were newly inserted in this call (i.e.
// first time we ever saw them) — useful to drive notifications.
func (s *Store) PersistFindings(ctx context.Context, findings []Finding) ([]int64, error) {
	if len(findings) == 0 {
		return nil, nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("findings: begin: %w", err)
	}
	defer tx.Rollback(ctx)

	var newIDs []int64
	for _, f := range findings {
		if f.DedupKey == "" {
			continue
		}
		evidenceJSON, err := json.Marshal(f.Evidence)
		if err != nil {
			return nil, fmt.Errorf("findings: marshal evidence for %s: %w", f.DedupKey, err)
		}
		var (
			id      int64
			created bool
		)
		err = tx.QueryRow(ctx, `
			INSERT INTO findings (
				finding_type, dedup_key, severity, title, subject, valor, evidence, link
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (dedup_key) DO UPDATE
				SET last_seen_at = NOW(),
				    severity    = EXCLUDED.severity,
				    valor       = EXCLUDED.valor,
				    evidence    = EXCLUDED.evidence
			RETURNING id, (xmax = 0) AS created
		`,
			string(f.Type), f.DedupKey, string(f.Severity),
			f.Title, f.Subject, f.Valor, string(evidenceJSON), f.Link,
		).Scan(&id, &created)
		if err != nil {
			return nil, fmt.Errorf("findings: upsert %s: %w", f.DedupKey, err)
		}
		if created {
			newIDs = append(newIDs, id)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("findings: commit: %w", err)
	}
	return newIDs, nil
}

// ListPersistedFindings returns persisted findings, optionally filtered by
// type and minimum severity, ordered by first_seen_at desc. Used by the
// atom feed and the active findings list.
func (s *Store) ListPersistedFindings(ctx context.Context, fType string, includeDismissed bool, limit int) ([]PersistedFinding, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	args := []interface{}{}
	where := "WHERE 1=1"
	i := 1
	if !includeDismissed {
		where += " AND dismissed_at IS NULL"
	}
	if fType != "" {
		where += fmt.Sprintf(" AND finding_type = $%d", i)
		args = append(args, fType)
		i++
	}
	args = append(args, limit)

	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, finding_type, dedup_key, severity, title, subject, valor, evidence, link,
		       first_seen_at, last_seen_at, dismissed_at, dismissed_by, dismissed_note,
		       confirmed_at, confirmed_by, notified_at
		FROM findings
		%s
		ORDER BY first_seen_at DESC
		LIMIT $%d
	`, where, i), args...)
	if err != nil {
		return nil, fmt.Errorf("findings: list: %w", err)
	}
	defer rows.Close()

	var out []PersistedFinding
	for rows.Next() {
		var (
			pf      PersistedFinding
			fType   string
			sev     string
			ev      []byte
		)
		if err := rows.Scan(&pf.ID, &fType, &pf.Finding.DedupKey, &sev,
			&pf.Finding.Title, &pf.Finding.Subject, &pf.Finding.Valor, &ev, &pf.Finding.Link,
			&pf.FirstSeenAt, &pf.LastSeenAt, &pf.DismissedAt, &pf.DismissedBy, &pf.DismissedNote,
			&pf.ConfirmedAt, &pf.ConfirmedBy, &pf.NotifiedAt); err != nil {
			return nil, err
		}
		pf.Finding.Type = FindingType(fType)
		pf.Finding.Severity = Severity(sev)
		if len(ev) > 0 {
			_ = json.Unmarshal(ev, &pf.Finding.Evidence)
		}
		out = append(out, pf)
	}
	return out, rows.Err()
}

// ListPendingNotificationFindings returns high-severity findings whose
// notified_at is NULL. Used by the notifier loop.
func (s *Store) ListPendingNotificationFindings(ctx context.Context, limit int) ([]PersistedFinding, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, finding_type, dedup_key, severity, title, subject, valor, evidence, link,
		       first_seen_at, last_seen_at
		FROM findings
		WHERE notified_at IS NULL AND severity = 'high' AND dismissed_at IS NULL
		ORDER BY first_seen_at ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PersistedFinding
	for rows.Next() {
		var (
			pf    PersistedFinding
			fType string
			sev   string
			ev    []byte
		)
		if err := rows.Scan(&pf.ID, &fType, &pf.Finding.DedupKey, &sev,
			&pf.Finding.Title, &pf.Finding.Subject, &pf.Finding.Valor, &ev, &pf.Finding.Link,
			&pf.FirstSeenAt, &pf.LastSeenAt); err != nil {
			return nil, err
		}
		pf.Finding.Type = FindingType(fType)
		pf.Finding.Severity = Severity(sev)
		if len(ev) > 0 {
			_ = json.Unmarshal(ev, &pf.Finding.Evidence)
		}
		out = append(out, pf)
	}
	return out, rows.Err()
}

// MarkFindingNotified flips notified_at = NOW(). Idempotent.
func (s *Store) MarkFindingNotified(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `UPDATE findings SET notified_at = NOW() WHERE id = $1`, id)
	return err
}

// DismissFinding marks a finding as dismissed (false positive or accepted risk).
func (s *Store) DismissFinding(ctx context.Context, id int64, by, note string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE findings
		SET dismissed_at = NOW(), dismissed_by = $2, dismissed_note = $3
		WHERE id = $1
	`, id, by, note)
	return err
}

// ConfirmFinding marks a finding as confirmed (a real case worth investigation).
func (s *Store) ConfirmFinding(ctx context.Context, id int64, by string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE findings SET confirmed_at = NOW(), confirmed_by = $2 WHERE id = $1
	`, id, by)
	return err
}

// FindingsCounts is the dashboard summary for the persisted findings table.
type FindingsCounts struct {
	Total       int64                  `json:"total"`
	BySeverity  map[string]int64       `json:"by_severity"`
	ByType      map[string]int64       `json:"by_type"`
	Active      int64                  `json:"active"`
	Dismissed   int64                  `json:"dismissed"`
	Confirmed   int64                  `json:"confirmed"`
	NewLast24h  int64                  `json:"new_last_24h"`
}

// GetFindingsCounts is the persisted equivalent of GetForensesSummary.
func (s *Store) GetFindingsCounts(ctx context.Context) (*FindingsCounts, error) {
	c := &FindingsCounts{
		BySeverity: make(map[string]int64),
		ByType:     make(map[string]int64),
	}
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM findings`).Scan(&c.Total); err != nil {
		return nil, err
	}
	if err := s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE dismissed_at IS NULL),
			COUNT(*) FILTER (WHERE dismissed_at IS NOT NULL),
			COUNT(*) FILTER (WHERE confirmed_at IS NOT NULL),
			COUNT(*) FILTER (WHERE first_seen_at >= NOW() - INTERVAL '24 hours')
		FROM findings
	`).Scan(&c.Active, &c.Dismissed, &c.Confirmed, &c.NewLast24h); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT severity, COUNT(*) FROM findings GROUP BY severity`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var sev string
		var n int64
		if err := rows.Scan(&sev, &n); err != nil {
			rows.Close()
			return nil, err
		}
		c.BySeverity[sev] = n
	}
	rows.Close()

	rows2, err := s.pool.Query(ctx, `SELECT finding_type, COUNT(*) FROM findings GROUP BY finding_type`)
	if err != nil {
		return nil, err
	}
	defer rows2.Close()
	for rows2.Next() {
		var t string
		var n int64
		if err := rows2.Scan(&t, &n); err != nil {
			return nil, err
		}
		c.ByType[t] = n
	}
	return c, nil
}

// _unused suppresses unused-import warnings if Time isn't referenced elsewhere.
var _ = time.Time{}
