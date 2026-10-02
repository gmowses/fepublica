package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// These tests require an empty, explicitly provisioned Postgres database. They
// create a private schema, so they never use the developer's normal schema.
func lifecycleTestStore(t *testing.T) *Store {
	return integrationTestStore(t, "001_initial.sql", "011_snapshot_lifecycle.sql")
}

func integrationTestStore(t *testing.T, migrations ...string) *Store {
	t.Helper()
	dsn := os.Getenv("FE_PUBLICA_TEST_DSN")
	if dsn == "" {
		t.Skip("set FE_PUBLICA_TEST_DSN to run Postgres lifecycle integration tests")
	}

	schema := fmt.Sprintf("snapshot_lifecycle_test_%d", time.Now().UnixNano())
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse test dsn: %v", err)
	}
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET search_path TO "+schema+", public")
		return err
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(context.Background(), "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	ensurePGTrgm(t, pool)

	for _, name := range migrations {
		migration, err := os.ReadFile(filepath.Join("..", "..", "db", "migrations", name))
		if err != nil {
			t.Fatalf("read migration %s: %v", name, err)
		}
		if _, err := pool.Exec(context.Background(), string(migration)); err != nil {
			t.Fatalf("apply migration %s: %v", name, err)
		}
	}
	return &Store{pool: pool}
}

func ensurePGTrgm(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire pg_trgm setup connection: %v", err)
	}
	defer conn.Release()
	const lockID int64 = 6_351_541_191
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", lockID); err != nil {
		t.Fatalf("lock pg_trgm setup: %v", err)
	}
	defer func() {
		if _, err := conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", lockID); err != nil {
			t.Errorf("unlock pg_trgm setup: %v", err)
		}
	}()
	if _, err := conn.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA public"); err != nil {
		t.Fatalf("create pg_trgm extension: %v", err)
	}
}

func lifecycleEvent(snapshotID int64, externalID string) InsertEventParams {
	return InsertEventParams{
		SnapshotID:    snapshotID,
		SourceID:      "ceis",
		ExternalID:    externalID,
		ContentHash:   make([]byte, 32),
		CanonicalJSON: []byte(`{"id":"` + externalID + `"}`),
	}
}

func lifecycleSnapshot(t *testing.T, s *Store, count int) int64 {
	t.Helper()
	id, err := s.CreateSnapshotWithCounts(context.Background(), CreateSnapshotParams{
		SourceID: "ceis", CollectedAt: time.Now().UTC(), CollectorVersion: "test",
	}, count, 0, "")
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	return id
}

func TestSnapshotLifecycleRejectsPartialAndLateEvents(t *testing.T) {
	s := lifecycleTestStore(t)
	ctx := context.Background()
	id := lifecycleSnapshot(t, s, 501)
	events := make([]InsertEventParams, 0, 501)
	for i := 0; i < 500; i++ {
		events = append(events, lifecycleEvent(id, fmt.Sprintf("event-%d", i)))
	}
	// This duplicate fails only in the second chunk, after the first 500 rows
	// have committed. It reproduces the former premature-anchor window.
	events = append(events, lifecycleEvent(id, "event-0"))
	inserted, err := s.InsertEventsBatch(ctx, events)
	if err == nil || inserted != 500 {
		t.Fatalf("second chunk failure = inserted %d, err %v; want 500 and error", inserted, err)
	}

	pending, err := s.ListSnapshotsPendingMerkle(ctx, 10, 200_000)
	if err != nil {
		t.Fatalf("list pending roots: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("partial snapshot became eligible for root: %#v", pending)
	}
	if _, err := s.pool.Exec(ctx, `UPDATE snapshots SET lifecycle_status = 'complete' WHERE id = $1`, id); err == nil {
		t.Fatal("database allowed a partial snapshot to become complete")
	}
	if err := s.CompleteSnapshot(ctx, id); err == nil || !strings.Contains(err.Error(), "expected 501") {
		t.Fatalf("complete partial snapshot error = %v, want count mismatch", err)
	}
	if err := s.FailSnapshot(ctx, id); err != nil {
		t.Fatalf("mark partial snapshot failed: %v", err)
	}
	if _, err := s.InsertEventsBatch(ctx, []InsertEventParams{lifecycleEvent(id, "two")}); err == nil {
		t.Fatal("late event insert into failed snapshot succeeded")
	}
}

func TestSnapshotLifecycleSealsCompleteSnapshot(t *testing.T) {
	s := lifecycleTestStore(t)
	ctx := context.Background()
	id := lifecycleSnapshot(t, s, 1)
	if _, err := s.InsertEventsBatch(ctx, []InsertEventParams{lifecycleEvent(id, "one")}); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	if err := s.CompleteSnapshot(ctx, id); err != nil {
		t.Fatalf("complete snapshot: %v", err)
	}
	if err := s.SetSnapshotMerkleRoot(ctx, id, make([]byte, 32)); err != nil {
		t.Fatalf("set root: %v", err)
	}
	if _, err := s.InsertEventsBatch(ctx, []InsertEventParams{lifecycleEvent(id, "late")}); err == nil {
		t.Fatal("late event insert into complete snapshot succeeded")
	}
	if err := s.SetSnapshotMerkleRoot(ctx, id, make([]byte, 32)); err == nil {
		t.Fatal("second root write succeeded")
	}
}

func TestSnapshotLifecycleSerializesDirectSealWithEventInsert(t *testing.T) {
	s := lifecycleTestStore(t)
	ctx := context.Background()
	id := lifecycleSnapshot(t, s, 1)
	if _, err := s.InsertEventsBatch(ctx, []InsertEventParams{lifecycleEvent(id, "one")}); err != nil {
		t.Fatalf("insert expected event: %v", err)
	}

	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire event transaction connection: %v", err)
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin event transaction: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	late := lifecycleEvent(id, "uncommitted")
	if _, err := tx.Exec(ctx, `
		INSERT INTO events (snapshot_id, source_id, external_id, content_hash, canonical_json, collected_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
	`, late.SnapshotID, late.SourceID, late.ExternalID, late.ContentHash, late.CanonicalJSON); err != nil {
		t.Fatalf("insert uncommitted event: %v", err)
	}

	sealed := make(chan error, 1)
	go func() {
		_, err := s.pool.Exec(ctx, `UPDATE snapshots SET lifecycle_status = 'complete' WHERE id = $1`, id)
		sealed <- err
	}()
	select {
	case err := <-sealed:
		t.Fatalf("direct seal finished before concurrent insert resolved: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback uncommitted event: %v", err)
	}
	if err := <-sealed; err != nil {
		t.Fatalf("seal after rollback: %v", err)
	}
}

func TestInsertEventsBatchCountsOnlyCommittedRows(t *testing.T) {
	s := lifecycleTestStore(t)
	ctx := context.Background()
	id := lifecycleSnapshot(t, s, 2)
	inserted, err := s.InsertEventsBatch(ctx, []InsertEventParams{
		lifecycleEvent(id, "duplicate"),
		lifecycleEvent(id, "duplicate"),
	})
	if err == nil || inserted != 0 {
		t.Fatalf("rolled back chunk reported inserted=%d, err=%v; want 0 and error", inserted, err)
	}
	var count int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM events WHERE snapshot_id = $1`, id).Scan(&count); err != nil {
		t.Fatalf("count rolled back events: %v", err)
	}
	if count != 0 {
		t.Fatalf("rolled back chunk persisted %d events", count)
	}
}
