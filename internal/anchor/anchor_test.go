package anchor

import (
	"strings"
	"testing"

	"github.com/gmowses/fepublica/internal/merkle"
	"github.com/gmowses/fepublica/internal/store"
)

func TestMerkleLeavesPreservesRoot(t *testing.T) {
	expectedLeaves := [][merkle.HashSize]byte{
		merkle.HashLeaf([]byte("first")),
		merkle.HashLeaf([]byte("second")),
		merkle.HashLeaf([]byte("third")),
	}
	storedLeaves := make([]store.SnapshotHashLeaf, len(expectedLeaves))
	for i, leaf := range expectedLeaves {
		storedLeaves[i] = store.SnapshotHashLeaf{ContentHash: leaf[:]}
	}

	gotLeaves, err := merkleLeaves(42, storedLeaves)
	if err != nil {
		t.Fatal(err)
	}
	expectedTree, err := merkle.Build(expectedLeaves)
	if err != nil {
		t.Fatal(err)
	}
	gotTree, err := merkle.Build(gotLeaves)
	if err != nil {
		t.Fatal(err)
	}
	if gotTree.Root() != expectedTree.Root() {
		t.Fatalf("root = %x, want %x", gotTree.Root(), expectedTree.Root())
	}
}

func TestValidateMerkleLeafCountRejectsBoundsAndMismatch(t *testing.T) {
	for _, count := range []int{0, -1} {
		if err := validateMerkleLeafCount(1, count, count); err == nil {
			t.Fatalf("expected non-positive record count %d error", count)
		}
	}
	if err := validateMerkleLeafCount(1, maxMerkleLeaves+1, maxMerkleLeaves+1); err == nil {
		t.Fatal("expected oversized snapshot error")
	}
	if err := validateMerkleLeafCount(1, 2, 1); err == nil {
		t.Fatal("expected record count mismatch error")
	}
	if err := validateMerkleLeafCount(1, 2, 2); err != nil {
		t.Fatalf("matching count returned error: %v", err)
	}
}

func TestMerkleLeavesRejectsInvalidHashLength(t *testing.T) {
	_, err := merkleLeaves(42, []store.SnapshotHashLeaf{{ContentHash: []byte("short")}})
	if err == nil || !strings.Contains(err.Error(), "hash leaf 0") {
		t.Fatalf("error = %v, want invalid hash leaf", err)
	}
}
