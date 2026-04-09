package merkle

import (
	"crypto/sha256"
	"fmt"
	"testing"
)

func leafFrom(s string) [HashSize]byte {
	return sha256.Sum256([]byte(s))
}

func TestBuildRejectsEmpty(t *testing.T) {
	if _, err := Build(nil); err == nil {
		t.Fatal("expected error on empty input")
	}
}

func TestSingleLeaf(t *testing.T) {
	leaves := [][HashSize]byte{leafFrom("a")}
	tree, err := Build(leaves)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Leaves() != 1 {
		t.Fatalf("expected 1 leaf, got %d", tree.Leaves())
	}
	if tree.Root() != leaves[0] {
		t.Fatal("root of single leaf must equal the leaf")
	}
	proof, err := tree.Proof(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(proof) != 0 {
		t.Fatalf("single-leaf proof must be empty, got %d steps", len(proof))
	}
	if !Verify(leaves[0], proof, tree.Root()) {
		t.Fatal("verify of single-leaf proof failed")
	}
}

func TestTwoLeaves(t *testing.T) {
	leaves := [][HashSize]byte{leafFrom("a"), leafFrom("b")}
	tree, err := Build(leaves)
	if err != nil {
		t.Fatal(err)
	}
	expected := hashPair(leaves[0], leaves[1])
	if tree.Root() != expected {
		t.Fatal("root mismatch for two leaves")
	}
	for i := 0; i < 2; i++ {
		p, err := tree.Proof(i)
		if err != nil {
			t.Fatal(err)
		}
		if !Verify(leaves[i], p, tree.Root()) {
			t.Fatalf("proof verify failed for index %d", i)
		}
	}
}

func TestOddLeaves(t *testing.T) {
	leaves := [][HashSize]byte{
		leafFrom("a"), leafFrom("b"), leafFrom("c"),
	}
	tree, err := Build(leaves)
	if err != nil {
		t.Fatal(err)
	}
	for i := range leaves {
		p, err := tree.Proof(i)
		if err != nil {
			t.Fatal(err)
		}
		if !Verify(leaves[i], p, tree.Root()) {
			t.Fatalf("verify failed for index %d", i)
		}
	}
}

func TestLargeTree(t *testing.T) {
	const N = 1000
	leaves := make([][HashSize]byte, N)
	for i := 0; i < N; i++ {
		leaves[i] = leafFrom(fmt.Sprintf("leaf-%d", i))
	}
	tree, err := Build(leaves)
	if err != nil {
		t.Fatal(err)
	}
	root := tree.Root()
	// Spot-check several indices.
	for _, i := range []int{0, 1, 42, 500, 999} {
		p, err := tree.Proof(i)
		if err != nil {
			t.Fatalf("proof index %d: %v", i, err)
		}
		if !Verify(leaves[i], p, root) {
			t.Fatalf("verify failed at index %d", i)
		}
	}
}

func TestProofRejectsWrongLeaf(t *testing.T) {
	leaves := [][HashSize]byte{leafFrom("a"), leafFrom("b"), leafFrom("c"), leafFrom("d")}
	tree, err := Build(leaves)
	if err != nil {
		t.Fatal(err)
	}
	p, err := tree.Proof(0)
	if err != nil {
		t.Fatal(err)
	}
	wrong := leafFrom("not-a")
	if Verify(wrong, p, tree.Root()) {
		t.Fatal("expected verify to reject wrong leaf")
	}
}

func TestProofRejectsTamperedProof(t *testing.T) {
	leaves := [][HashSize]byte{leafFrom("a"), leafFrom("b"), leafFrom("c"), leafFrom("d")}
	tree, err := Build(leaves)
	if err != nil {
		t.Fatal(err)
	}
	p, err := tree.Proof(1)
	if err != nil {
		t.Fatal(err)
	}
	// Flip a bit in the first sibling.
	p[0].Sibling[0] ^= 0xff
	if Verify(leaves[1], p, tree.Root()) {
		t.Fatal("expected verify to reject tampered proof")
	}
}

func TestProofOutOfRange(t *testing.T) {
	leaves := [][HashSize]byte{leafFrom("a"), leafFrom("b")}
	tree, err := Build(leaves)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tree.Proof(2); err == nil {
		t.Fatal("expected out-of-range error")
	}
	if _, err := tree.Proof(-1); err == nil {
		t.Fatal("expected out-of-range error for negative index")
	}
}
