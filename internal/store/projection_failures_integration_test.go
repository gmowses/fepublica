package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gmowses/fepublica/internal/cartoes"
)

func projectionSnapshot(t *testing.T, s *Store, source string, count int) int64 {
	t.Helper()
	id, err := s.CreateSnapshotWithCounts(context.Background(), CreateSnapshotParams{
		SourceID: source, CollectedAt: time.Now().UTC(), CollectorVersion: "test",
	}, count, 0, "")
	if err != nil {
		t.Fatalf("create projection snapshot: %v", err)
	}
	return id
}

func TestProjectionFailuresDoNotStarveLaterEvents(t *testing.T) {
	s := integrationTestStore(t,
		"001_initial.sql", "002_add_pncp.sql", "007_add_contratos.sql",
		"008_add_cartoes.sql", "010_add_ceap.sql", "011_snapshot_lifecycle.sql",
		"012_projection_failures.sql",
	)
	ctx := context.Background()
	id := projectionSnapshot(t, s, "cartoes-cpgf", 2)
	events := []InsertEventParams{
		{SnapshotID: id, SourceID: "cartoes-cpgf", ExternalID: "invalid", ContentHash: make([]byte, 32), CanonicalJSON: []byte(`[]`)},
		{SnapshotID: id, SourceID: "cartoes-cpgf", ExternalID: "valid", ContentHash: make([]byte, 32), CanonicalJSON: []byte(`{"id":1}`)},
	}
	if _, err := s.InsertEventsBatch(ctx, events); err != nil {
		t.Fatalf("insert projection events: %v", err)
	}
	pending, err := s.ListUnindexedCartoesEvents(ctx, 10)
	if err != nil || len(pending) != 0 {
		t.Fatalf("collecting snapshot was eligible for projection: %#v, %v", pending, err)
	}
	if err := s.CompleteSnapshot(ctx, id); err != nil {
		t.Fatalf("complete projection snapshot: %v", err)
	}

	pending, err = s.ListUnindexedCartoesEvents(ctx, 10)
	if err != nil || len(pending) != 2 {
		t.Fatalf("initial pending = %d, %v; want two events", len(pending), err)
	}
	if _, err := s.RecordProjectionFailure(ctx, ProjectionCartoes, pending[0].ID, errors.New("invalid shape")); err != nil {
		t.Fatalf("record first failure: %v", err)
	}
	pending, err = s.ListUnindexedCartoesEvents(ctx, 10)
	if err != nil || len(pending) != 1 || pending[0].ExternalID != "valid" {
		t.Fatalf("pending after failure = %#v, %v; want valid event only", pending, err)
	}
	if err := s.InsertCartao(ctx, pending[0].ID, pending[0].SnapshotID, &cartoes.Row{ExternalID: pending[0].ExternalID}, pending[0].CollectedAt); err != nil {
		t.Fatalf("insert valid projection: %v", err)
	}
	if err := s.ResolveProjectionFailure(ctx, ProjectionCartoes, pending[0].ID); err != nil {
		t.Fatalf("resolve valid projection failure: %v", err)
	}

	// The failure row is keyed by event_id; retrieve it from the original batch
	// and retry it until it becomes terminal.
	invalidEventID := int64(0)
	if err := s.pool.QueryRow(ctx, `SELECT id FROM events WHERE snapshot_id = $1 AND external_id = 'invalid'`, id).Scan(&invalidEventID); err != nil {
		t.Fatalf("find invalid event: %v", err)
	}
	var failure ProjectionFailure
	for i := 0; i < 2; i++ {
		failure, err = s.RecordProjectionFailure(ctx, ProjectionCartoes, invalidEventID, errors.New("invalid shape"))
		if err != nil {
			t.Fatalf("record retry %d: %v", i+2, err)
		}
	}
	if !failure.Terminal || failure.Attempts != 3 {
		t.Fatalf("failure = %#v, want terminal attempt 3", failure)
	}
	pending, err = s.ListUnindexedCartoesEvents(ctx, 10)
	if err != nil || len(pending) != 0 {
		t.Fatalf("terminal failure remained pending: %#v, %v", pending, err)
	}
	if err := s.RequeueProjectionFailure(ctx, ProjectionCartoes, invalidEventID); err != nil {
		t.Fatalf("requeue terminal failure: %v", err)
	}
	pending, err = s.ListUnindexedCartoesEvents(ctx, 10)
	if err != nil || len(pending) != 1 || pending[0].ExternalID != "invalid" {
		t.Fatalf("requeued failure was not pending: %#v, %v", pending, err)
	}
}
