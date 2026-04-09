// Package merkle implements a binary SHA-256 Merkle tree with inclusion proofs.
//
// Design choices:
//
//   - Leaves are arbitrary 32-byte digests (typically SHA-256 of canonical JSON).
//   - Internal nodes are SHA-256(left || right).
//   - If the number of leaves on a level is odd, the last node is duplicated.
//   - The leaf order is preserved as provided; the caller owns ordering semantics.
//   - Proofs are represented as a sequence of (siblingHash, side) pairs, ordered
//     from leaf to root. "side" indicates whether the sibling is on the left
//     (0) or the right (1) of the current node when hashing upward.
//
// This implementation deliberately avoids external dependencies beyond stdlib.
package merkle

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
)

// HashSize is the length in bytes of a SHA-256 digest.
const HashSize = 32

// Side identifies the position of a sibling relative to the node being hashed upward.
type Side uint8

const (
	// SideLeft means the sibling is the left child; the current hash is on the right.
	SideLeft Side = 0
	// SideRight means the sibling is the right child; the current hash is on the left.
	SideRight Side = 1
)

// ProofStep is a single level of a Merkle inclusion proof.
type ProofStep struct {
	Sibling [HashSize]byte
	Side    Side
}

// Tree is an in-memory Merkle tree.
type Tree struct {
	leaves [][HashSize]byte
	levels [][][HashSize]byte // levels[0] == leaves (possibly padded), levels[len-1] == [root]
}

// Build constructs a Merkle tree from leaf hashes. An empty input returns an error.
func Build(leaves [][HashSize]byte) (*Tree, error) {
	if len(leaves) == 0 {
		return nil, errors.New("merkle: cannot build tree with zero leaves")
	}
	// Copy input to avoid aliasing caller slice.
	initial := make([][HashSize]byte, len(leaves))
	copy(initial, leaves)

	levels := [][][HashSize]byte{initial}
	current := initial
	for len(current) > 1 {
		// Duplicate the last element if current level has odd count.
		if len(current)%2 == 1 {
			current = append(current, current[len(current)-1])
		}
		next := make([][HashSize]byte, len(current)/2)
		for i := 0; i < len(current); i += 2 {
			next[i/2] = hashPair(current[i], current[i+1])
		}
		levels = append(levels, next)
		current = next
	}

	return &Tree{leaves: leaves, levels: levels}, nil
}

// Root returns the Merkle root.
func (t *Tree) Root() [HashSize]byte {
	last := t.levels[len(t.levels)-1]
	return last[0]
}

// Leaves returns the number of original leaves provided to Build (not including padding).
func (t *Tree) Leaves() int {
	return len(t.leaves)
}

// Proof returns an inclusion proof for the leaf at the given index.
func (t *Tree) Proof(index int) ([]ProofStep, error) {
	if index < 0 || index >= len(t.leaves) {
		return nil, fmt.Errorf("merkle: index out of range: %d (leaves=%d)", index, len(t.leaves))
	}
	var proof []ProofStep
	cursor := index
	for lvl := 0; lvl < len(t.levels)-1; lvl++ {
		level := t.levels[lvl]
		// Apply the same padding rule Build uses.
		if len(level)%2 == 1 {
			level = append(level, level[len(level)-1])
		}
		var sibling [HashSize]byte
		var side Side
		if cursor%2 == 0 {
			sibling = level[cursor+1]
			side = SideRight
		} else {
			sibling = level[cursor-1]
			side = SideLeft
		}
		proof = append(proof, ProofStep{Sibling: sibling, Side: side})
		cursor /= 2
	}
	return proof, nil
}

// Verify checks that a leaf hash combined with a proof reconstructs the given root.
func Verify(leaf [HashSize]byte, proof []ProofStep, root [HashSize]byte) bool {
	current := leaf
	for _, step := range proof {
		switch step.Side {
		case SideLeft:
			current = hashPair(step.Sibling, current)
		case SideRight:
			current = hashPair(current, step.Sibling)
		default:
			return false
		}
	}
	return bytes.Equal(current[:], root[:])
}

// HashLeaf is a convenience helper: SHA-256 of arbitrary bytes.
func HashLeaf(b []byte) [HashSize]byte {
	return sha256.Sum256(b)
}

func hashPair(left, right [HashSize]byte) [HashSize]byte {
	h := sha256.New()
	h.Write(left[:])
	h.Write(right[:])
	var out [HashSize]byte
	copy(out[:], h.Sum(nil))
	return out
}
