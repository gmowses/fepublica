package store

import (
	"context"
	"fmt"
)

// EntesByUF returns the count of entes grouped by UF.
// Used by the Observatório landing map heat.
func (s *Store) EntesByUF(ctx context.Context) (map[string]int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT uf, COUNT(*) FROM entes
		WHERE uf IS NOT NULL AND active = TRUE
		GROUP BY uf
	`)
	if err != nil {
		return nil, fmt.Errorf("store: entes by uf: %w", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var uf string
		var n int
		if err := rows.Scan(&uf, &n); err != nil {
			return nil, err
		}
		out[uf] = n
	}
	return out, rows.Err()
}

// ChangeEventsByUF returns change_event counts grouped by the UF of their
// associated ente (via JOIN). Events without ente_id are counted under "".
func (s *Store) ChangeEventsByUF(ctx context.Context) (map[string]int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT COALESCE(e.uf, '') AS uf, COUNT(*)
		FROM change_events ce
		LEFT JOIN entes e ON e.id = ce.ente_id
		GROUP BY e.uf
	`)
	if err != nil {
		return nil, fmt.Errorf("store: change_events by uf: %w", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var uf string
		var n int
		if err := rows.Scan(&uf, &n); err != nil {
			return nil, err
		}
		out[uf] = n
	}
	return out, rows.Err()
}

// ObservatorioStats are the aggregate counters shown on the dashboard hero.
type ObservatorioStats struct {
	TotalEntes        int `json:"total_entes"`
	ActiveSources     int `json:"active_sources"`
	TotalSnapshots    int `json:"total_snapshots"`
	TotalEvents       int `json:"total_events"`
	TotalDiffRuns     int `json:"total_diff_runs"`
	TotalChangeEvents int `json:"total_change_events"`
	AlertCount        int `json:"alert_count"`
	WarnCount         int `json:"warn_count"`
}

// GetObservatorioStats returns the aggregate numbers for the dashboard hero.
func (s *Store) GetObservatorioStats(ctx context.Context) (*ObservatorioStats, error) {
	var st ObservatorioStats
	queries := []struct {
		dst *int
		sql string
	}{
		{&st.TotalEntes, "SELECT COUNT(*) FROM entes WHERE active = TRUE"},
		{&st.ActiveSources, "SELECT COUNT(*) FROM sources"},
		{&st.TotalSnapshots, "SELECT COUNT(*) FROM snapshots"},
		{&st.TotalEvents, "SELECT COALESCE(SUM(record_count), 0) FROM snapshots"},
		{&st.TotalDiffRuns, "SELECT COUNT(*) FROM diff_runs"},
		{&st.TotalChangeEvents, "SELECT COUNT(*) FROM change_events"},
		{&st.AlertCount, "SELECT COUNT(*) FROM change_events WHERE severity = 'alert'"},
		{&st.WarnCount, "SELECT COUNT(*) FROM change_events WHERE severity = 'warn'"},
	}
	for _, q := range queries {
		if err := s.pool.QueryRow(ctx, q.sql).Scan(q.dst); err != nil {
			return nil, fmt.Errorf("store: observatorio stats %q: %w", q.sql, err)
		}
	}
	return &st, nil
}
