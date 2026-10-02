package api

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/gmowses/fepublica/internal/merkle"
	"github.com/gmowses/fepublica/internal/store"
)

func proofIntegrationServer(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	dsn := os.Getenv("FE_PUBLICA_TEST_DSN")
	if dsn == "" {
		t.Skip("set FE_PUBLICA_TEST_DSN")
	}
	admin, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	t.Cleanup(admin.Close)
	schema := fmt.Sprintf("proof_api_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(context.Background(), "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("drop test schema: %v", err)
		}
	})

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse test dsn: %v", err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatalf("connect test schema: %v", err)
	}
	for _, migration := range []string{"001_initial.sql", "011_snapshot_lifecycle.sql"} {
		contents, err := os.ReadFile(filepath.Join("..", "..", "db", "migrations", migration))
		if err != nil {
			pool.Close()
			t.Fatalf("read migration %s: %v", migration, err)
		}
		if _, err := pool.Exec(context.Background(), string(contents)); err != nil {
			pool.Close()
			t.Fatalf("apply migration %s: %v", migration, err)
		}
	}
	pool.Close()

	// The private search_path keeps Store's pool in this test schema as well.
	s, err := store.Open(context.Background(), testSchemaDSN(dsn, schema))
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	t.Cleanup(s.Close)
	return New(s, zerolog.Nop(), "test", ""), s
}

func testSchemaDSN(dsn, schema string) string {
	u, err := url.Parse(dsn)
	if err == nil && (u.Scheme == "postgres" || u.Scheme == "postgresql") {
		query := u.Query()
		query.Set("search_path", schema+",public")
		u.RawQuery = query.Encode()
		return u.String()
	}
	return dsn + " search_path=" + schema + ",public"
}

func proofSnapshot(t *testing.T, s *store.Store, events []store.InsertEventParams, root [merkle.HashSize]byte) int64 {
	t.Helper()
	id, err := s.CreateSnapshotWithCounts(context.Background(), store.CreateSnapshotParams{SourceID: "ceis", CollectedAt: time.Now().UTC(), CollectorVersion: "proof-test"}, len(events), 20, "")
	if err != nil {
		t.Fatal(err)
	}
	for i := range events {
		events[i].SnapshotID, events[i].SourceID = id, "ceis"
	}
	if _, err := s.InsertEventsBatch(context.Background(), events); err != nil {
		t.Fatal(err)
	}
	if err := s.CompleteSnapshot(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSnapshotMerkleRoot(context.Background(), id, root[:]); err != nil {
		t.Fatal(err)
	}
	return id
}

func proofRoot(t *testing.T, events []store.InsertEventParams) [merkle.HashSize]byte {
	t.Helper()
	leaves := make([][merkle.HashSize]byte, len(events))
	for i, event := range events {
		copy(leaves[i][:], event.ContentHash)
	}
	tree, err := merkle.Build(leaves)
	if err != nil {
		t.Fatal(err)
	}
	return tree.Root()
}

func proofEvent(externalID string, hashByte byte, payload string) store.InsertEventParams {
	hash := make([]byte, merkle.HashSize)
	hash[0] = hashByte
	return store.InsertEventParams{ExternalID: externalID, ContentHash: hash, CanonicalJSON: []byte(payload)}
}

func proofRequest(s *Server, snapshotID int64, externalID string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/snapshots/%d/events/%s/proof", snapshotID, externalID), nil)
	s.Routes().ServeHTTP(w, r)
	return w
}

func TestProofEndpointProducesVerifiableProof(t *testing.T) {
	srv, s := proofIntegrationServer(t)
	events := []store.InsertEventParams{proofEvent("one", 1, `{"id":"one"}`), proofEvent("two", 2, `{"id":"two"}`)}
	id := proofSnapshot(t, s, events, proofRoot(t, events))
	w := proofRequest(srv, id, "one")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var dto ProofDTO
	if err := json.Unmarshal(w.Body.Bytes(), &dto); err != nil {
		t.Fatal(err)
	}
	if dto.Version != 1 || dto.Event.ExternalID != "one" || dto.Merkle.Index != 0 || len(dto.Merkle.Siblings) != 1 {
		t.Fatalf("unexpected proof DTO: %+v", dto)
	}
	leaf, err := proofHash(dto.Event.ContentHash)
	if err != nil {
		t.Fatal(err)
	}
	root, err := proofHash(dto.Merkle.Root)
	if err != nil {
		t.Fatal(err)
	}
	steps := make([]merkle.ProofStep, len(dto.Merkle.Siblings))
	for i, sibling := range dto.Merkle.Siblings {
		hash, err := proofHash(sibling.Sibling)
		if err != nil {
			t.Fatal(err)
		}
		steps[i].Sibling = hash
		switch sibling.Side {
		case "left":
			steps[i].Side = merkle.SideLeft
		case "right":
			steps[i].Side = merkle.SideRight
		default:
			t.Fatalf("unexpected proof side %q", sibling.Side)
		}
	}
	if !merkle.Verify(leaf, steps, root) {
		t.Fatal("returned DTO does not verify against its merkle root")
	}
}

func proofHash(value string) ([merkle.HashSize]byte, error) {
	var hash [merkle.HashSize]byte
	decoded, err := hex.DecodeString(value)
	if err != nil {
		return hash, err
	}
	if len(decoded) != merkle.HashSize {
		return hash, fmt.Errorf("hash length = %d, want %d", len(decoded), merkle.HashSize)
	}
	copy(hash[:], decoded)
	return hash, nil
}

func TestProofEndpointRejectsHistoricalRootMismatch(t *testing.T) {
	srv, s := proofIntegrationServer(t)
	events := []store.InsertEventParams{proofEvent("one", 1, `{"id":"one"}`)}
	var historicalRoot [merkle.HashSize]byte
	historicalRoot[0] = 99
	id := proofSnapshot(t, s, events, historicalRoot)
	if w := proofRequest(srv, id, "one"); w.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestProofEndpointRejectsCollectingSnapshot(t *testing.T) {
	srv, s := proofIntegrationServer(t)
	id, err := s.CreateSnapshotWithCounts(context.Background(), store.CreateSnapshotParams{SourceID: "ceis", CollectedAt: time.Now().UTC(), CollectorVersion: "proof-test"}, 1, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if w := proofRequest(srv, id, "missing"); w.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestProofEndpointRejectsOversizeTargetPayload(t *testing.T) {
	srv, s := proofIntegrationServer(t)
	payload := `{"payload":"` + strings.Repeat("x", 2<<20) + `"}`
	events := []store.InsertEventParams{proofEvent("large", 1, payload)}
	id := proofSnapshot(t, s, events, proofRoot(t, events))
	if w := proofRequest(srv, id, "large"); w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestProofEndpointReturnsNotFoundForUnknownEvent(t *testing.T) {
	srv, s := proofIntegrationServer(t)
	events := []store.InsertEventParams{proofEvent("one", 1, `{"id":"one"}`)}
	id := proofSnapshot(t, s, events, proofRoot(t, events))
	if w := proofRequest(srv, id, "absent"); w.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestProofEndpointRejectsSnapshotOverLeafCapBeforeReadingEvents(t *testing.T) {
	srv, s := proofIntegrationServer(t)
	var id int64
	err := s.Pool().QueryRow(context.Background(), `
		INSERT INTO snapshots (source_id, collected_at, collector_version, record_count, bytes_size, merkle_root, merkle_computed_at, lifecycle_status)
		VALUES ('ceis', NOW(), 'proof-test', 200001, 0, $1, NOW(), 'complete') RETURNING id
	`, make([]byte, merkle.HashSize)).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	if w := proofRequest(srv, id, "irrelevant"); w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestDiffEndpointRejectsCollectingSnapshot(t *testing.T) {
	srv, s := proofIntegrationServer(t)
	collecting, err := s.CreateSnapshotWithCounts(context.Background(), store.CreateSnapshotParams{SourceID: "ceis", CollectedAt: time.Now().UTC(), CollectorVersion: "proof-test"}, 0, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	complete, err := s.CreateSnapshotWithCounts(context.Background(), store.CreateSnapshotParams{SourceID: "ceis", CollectedAt: time.Now().UTC().Add(time.Nanosecond), CollectorVersion: "proof-test"}, 0, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CompleteSnapshot(context.Background(), complete); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/snapshots/%d/diff/%d", collecting, complete), nil)
	srv.Routes().ServeHTTP(w, r)
	if w.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}
