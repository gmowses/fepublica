// Package store provides Postgres-backed repositories for Fé Pública.
//
// The store layer owns all SQL. Business logic in internal/collector and
// internal/anchor must not write SQL directly. This keeps the
// append-only discipline in a single place.
package store

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store is the top-level handle for Postgres access.
type Store struct {
	pool *pgxpool.Pool
}

// Open dials Postgres and returns a Store. Caller is responsible for calling Close.
func Open(ctx context.Context, dsn string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("store: parse dsn: %w", err)
	}
	cfg.MaxConns = 10
	cfg.MinConns = 1
	cfg.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("store: connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Close releases all pooled connections.
func (s *Store) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

// Pool exposes the underlying pool for repositories in this package.
func (s *Store) Pool() *pgxpool.Pool {
	return s.pool
}

// Source represents a configured public data source.
type Source struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	BaseURL     string    `json:"base_url"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Snapshot represents one execution of a collector.
type Snapshot struct {
	ID               int64
	SourceID         string
	CollectedAt      time.Time
	APIVersion       string
	RecordCount      int
	BytesSize        int64
	MerkleRoot       []byte
	MerkleComputedAt *time.Time
	CollectorVersion string
	Notes            string
}

// Event is a single record captured in a snapshot.
type Event struct {
	ID            int64
	SnapshotID    int64
	SourceID      string
	ExternalID    string
	ContentHash   []byte
	CanonicalJSON []byte
	CollectedAt   time.Time
}

// Anchor is the result of submitting a snapshot's Merkle root to an OTS calendar.
type Anchor struct {
	ID          int64
	SnapshotID  int64
	CalendarURL string
	SubmittedAt time.Time
	Receipt     []byte
	Upgraded    bool
	UpgradedAt  *time.Time
	BlockHeight *int
}

// HexHash formats a byte slice as lowercase hex (useful for logs/API responses).
func HexHash(b []byte) string {
	return hex.EncodeToString(b)
}

// ListSources returns all configured sources.
func (s *Store) ListSources(ctx context.Context) ([]Source, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, base_url, COALESCE(description, ''), created_at
		FROM sources
		ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("store: list sources: %w", err)
	}
	defer rows.Close()

	var out []Source
	for rows.Next() {
		var src Source
		if err := rows.Scan(&src.ID, &src.Name, &src.BaseURL, &src.Description, &src.CreatedAt); err != nil {
			return nil, fmt.Errorf("store: scan source: %w", err)
		}
		out = append(out, src)
	}
	return out, rows.Err()
}

// GetSource returns a single source by ID or an error if not found.
func (s *Store) GetSource(ctx context.Context, id string) (*Source, error) {
	var src Source
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, base_url, COALESCE(description, ''), created_at
		FROM sources WHERE id = $1
	`, id).Scan(&src.ID, &src.Name, &src.BaseURL, &src.Description, &src.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("store: get source %q: %w", id, err)
	}
	return &src, nil
}
