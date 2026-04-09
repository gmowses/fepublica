// Command fepublica-verify validates a proof file offline, without requiring
// access to the Fé Pública server that emitted it.
//
// Usage:
//
//	fepublica-verify proof.json
//
// The command performs three checks:
//
//  1. Re-hashes the canonical JSON of the event and compares it to the
//     declared content_hash.
//  2. Applies the Merkle inclusion proof to reconstruct the root and
//     compares it to the declared merkle.root.
//  3. Prints the OTS receipt metadata. Full cryptographic verification of
//     the Bitcoin anchor is delegated to the reference `ots verify` CLI
//     (install via `pip install opentimestamps-client`), which the user
//     can run against the extracted .ots file printed here.
//
// Exit codes:
//
//	0 — all local checks passed
//	1 — verification failed or file unreadable
package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/gmowses/fepublica/internal/canonjson"
	"github.com/gmowses/fepublica/internal/merkle"
)

var version = "dev"

// proofDTO must stay in sync with internal/api ProofDTO.
type proofDTO struct {
	Version             int             `json:"version"`
	SourceID            string          `json:"source_id"`
	SnapshotID          int64           `json:"snapshot_id"`
	SnapshotCollectedAt string          `json:"snapshot_collected_at"`
	Event               proofEvent      `json:"event"`
	Merkle              proofMerkle     `json:"merkle"`
	Anchors             []proofAnchor   `json:"anchors"`
	GeneratedAt         string          `json:"generated_at"`
}

type proofEvent struct {
	ExternalID    string          `json:"external_id"`
	ContentHash   string          `json:"content_hash"`
	CanonicalJSON json.RawMessage `json:"canonical_json"`
}

type proofMerkle struct {
	Root     string      `json:"root"`
	Index    int         `json:"index"`
	Siblings []proofStep `json:"siblings"`
}

type proofStep struct {
	Sibling string `json:"sibling"`
	Side    string `json:"side"`
}

type proofAnchor struct {
	CalendarURL   string `json:"calendar_url"`
	ReceiptBase64 string `json:"receipt_base64"`
	Upgraded      bool   `json:"upgraded"`
	BlockHeight   *int   `json:"block_height,omitempty"`
	SubmittedAt   string `json:"submitted_at"`
}

func main() {
	root := &cobra.Command{
		Use:     "fepublica-verify <proof.json>",
		Short:   "Verify a Fé Pública proof offline",
		Version: version,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return verify(args[0])
		},
	}
	var extractDir string
	extractCmd := &cobra.Command{
		Use:   "extract <proof.json>",
		Short: "Extract .ots receipt files to a directory for use with the reference ots CLI",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return extract(args[0], extractDir)
		},
	}
	extractCmd.Flags().StringVarP(&extractDir, "out", "o", ".", "output directory for .ots files")
	root.AddCommand(extractCmd)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func verify(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read proof: %w", err)
	}
	var p proofDTO
	if err := json.Unmarshal(data, &p); err != nil {
		return fmt.Errorf("parse proof: %w", err)
	}
	if p.Version != 1 {
		return fmt.Errorf("unsupported proof version %d (only 1 supported)", p.Version)
	}

	// 1. Re-hash canonical JSON.
	canon, err := canonjson.Marshal(json.RawMessage(p.Event.CanonicalJSON))
	if err != nil {
		return fmt.Errorf("canonicalize event: %w", err)
	}
	actual := sha256.Sum256(canon)
	expected, err := hex.DecodeString(p.Event.ContentHash)
	if err != nil {
		return fmt.Errorf("decode content_hash hex: %w", err)
	}
	if len(expected) != sha256.Size {
		return fmt.Errorf("content_hash wrong length (%d)", len(expected))
	}
	if !equalBytes(actual[:], expected) {
		return fmt.Errorf("content hash mismatch:\n  computed: %x\n  declared: %x", actual[:], expected)
	}
	fmt.Printf("[1/3] content hash matches: sha256:%s\n", p.Event.ContentHash)

	// 2. Merkle proof.
	var leaf [merkle.HashSize]byte
	copy(leaf[:], actual[:])

	steps := make([]merkle.ProofStep, 0, len(p.Merkle.Siblings))
	for i, step := range p.Merkle.Siblings {
		sib, err := hex.DecodeString(step.Sibling)
		if err != nil || len(sib) != merkle.HashSize {
			return fmt.Errorf("merkle sibling %d invalid", i)
		}
		var s merkle.ProofStep
		copy(s.Sibling[:], sib)
		switch step.Side {
		case "left":
			s.Side = merkle.SideLeft
		case "right":
			s.Side = merkle.SideRight
		default:
			return fmt.Errorf("merkle sibling %d: unknown side %q", i, step.Side)
		}
		steps = append(steps, s)
	}

	declaredRootBytes, err := hex.DecodeString(p.Merkle.Root)
	if err != nil || len(declaredRootBytes) != merkle.HashSize {
		return fmt.Errorf("decode merkle root: %w", err)
	}
	var declaredRoot [merkle.HashSize]byte
	copy(declaredRoot[:], declaredRootBytes)

	if !merkle.Verify(leaf, steps, declaredRoot) {
		return fmt.Errorf("merkle proof does not reconstruct declared root %s", p.Merkle.Root)
	}
	fmt.Printf("[2/3] merkle proof valid (root sha256:%s)\n", p.Merkle.Root)

	// 3. OTS metadata.
	fmt.Printf("[3/3] OTS anchors attached:\n")
	if len(p.Anchors) == 0 {
		fmt.Printf("  (no anchors — this snapshot is not yet anchored)\n")
		return fmt.Errorf("proof contains no anchors")
	}
	for i, a := range p.Anchors {
		status := "pending"
		if a.Upgraded {
			status = "upgraded (confirmed in Bitcoin)"
		}
		block := ""
		if a.BlockHeight != nil {
			block = fmt.Sprintf(" block=%d", *a.BlockHeight)
		}
		rcpt, derr := base64.StdEncoding.DecodeString(a.ReceiptBase64)
		if derr != nil {
			fmt.Printf("  #%d %s — invalid base64 receipt\n", i+1, a.CalendarURL)
			continue
		}
		fmt.Printf("  #%d %s\n      status=%s%s receipt=%d bytes submitted=%s\n",
			i+1, a.CalendarURL, status, block, len(rcpt), a.SubmittedAt)
	}
	fmt.Printf("\nLocal verification passed.\n")
	fmt.Printf("To verify the Bitcoin anchor end-to-end, run:\n")
	fmt.Printf("  fepublica-verify extract %s --out ./receipts/\n", path)
	fmt.Printf("  ots verify ./receipts/<file>.ots\n")
	fmt.Printf("(install the reference CLI with: pip install opentimestamps-client)\n")
	return nil
}

func extract(path, outDir string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read proof: %w", err)
	}
	var p proofDTO
	if err := json.Unmarshal(data, &p); err != nil {
		return fmt.Errorf("parse proof: %w", err)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	for i, a := range p.Anchors {
		rcpt, err := base64.StdEncoding.DecodeString(a.ReceiptBase64)
		if err != nil {
			return fmt.Errorf("decode anchor %d: %w", i, err)
		}
		name := fmt.Sprintf("%s/snapshot-%d-anchor-%d.ots", outDir, p.SnapshotID, i+1)
		if err := os.WriteFile(name, rcpt, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
		fmt.Printf("wrote %s (%d bytes)\n", name, len(rcpt))
	}
	return nil
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
