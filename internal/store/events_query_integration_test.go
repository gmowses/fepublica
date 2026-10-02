package store

import (
	"context"
	"testing"
)

func diffEvent(snapshotID int64, externalID string, marker byte) InsertEventParams {
	h := make([]byte, 32)
	h[0] = marker
	return InsertEventParams{SnapshotID: snapshotID, SourceID: "ceis", ExternalID: externalID, ContentHash: h, CanonicalJSON: []byte(`{"id":"` + externalID + `"}`)}
}

func insertDiffEvents(t *testing.T, s *Store, snapshotID int64, events ...InsertEventParams) {
	t.Helper()
	if _, err := s.InsertEventsBatch(context.Background(), events); err != nil {
		t.Fatal(err)
	}
}

func TestCompareSnapshotsFiltersBothSidesAndPaginates(t *testing.T) {
	s := lifecycleTestStore(t)
	ctx := context.Background()
	a, b, other := lifecycleSnapshot(t, s, 3), lifecycleSnapshot(t, s, 3), lifecycleSnapshot(t, s, 2)
	insertDiffEvents(t, s, a, diffEvent(a, "changed", 1), diffEvent(a, "removed-a", 2), diffEvent(a, "removed-b", 3))
	insertDiffEvents(t, s, b, diffEvent(b, "changed", 9), diffEvent(b, "added-a", 4), diffEvent(b, "added-b", 5))
	// A third snapshot deliberately overlaps and adds IDs. It must not affect either side.
	insertDiffEvents(t, s, other, diffEvent(other, "changed", 7), diffEvent(other, "third-only", 8))

	page, err := s.CompareSnapshots(ctx, a, b, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if page.AddedTotal != 2 || page.RemovedTotal != 2 || page.ChangedTotal != 1 {
		t.Fatalf("totals = %+v", page)
	}
	if len(page.Added) != 1 || page.Added[0].ExternalID != "added-a" || len(page.Removed) != 1 || page.Removed[0].ExternalID != "removed-a" || len(page.Changed) != 1 || page.Changed[0].ExternalID != "changed" {
		t.Fatalf("first page = %+v", page)
	}
	page, err = s.CompareSnapshots(ctx, a, b, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Added) != 1 || page.Added[0].ExternalID != "added-b" || len(page.Removed) != 1 || page.Removed[0].ExternalID != "removed-b" || len(page.Changed) != 0 {
		t.Fatalf("second page = %+v", page)
	}
	page, err = s.CompareSnapshots(ctx, a, b, 1, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Added) != 0 || len(page.Removed) != 0 || len(page.Changed) != 0 {
		t.Fatalf("past-end page = %+v", page)
	}
	same, err := s.CompareSnapshots(ctx, a, a, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if same.AddedTotal != 0 || same.RemovedTotal != 0 || same.ChangedTotal != 0 {
		t.Fatalf("same snapshot = %+v", same)
	}
}

func TestListSnapshotHashLeavesNeverTruncates(t *testing.T) {
	s := lifecycleTestStore(t)
	id := lifecycleSnapshot(t, s, 2)
	insertDiffEvents(t, s, id, diffEvent(id, "one", 1), diffEvent(id, "two", 2))
	if _, err := s.ListSnapshotHashLeaves(context.Background(), id, 1); err == nil {
		t.Fatal("expected cap error")
	}
	leaves, err := s.ListSnapshotHashLeaves(context.Background(), id, 2)
	if err != nil || len(leaves) != 2 || leaves[0].ContentHash[0] != 1 {
		t.Fatalf("leaves=%v err=%v", leaves, err)
	}
}
