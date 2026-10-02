package main

import (
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractBuildsOfficialDetachedTimestampFile(t *testing.T) {
	full, err := os.ReadFile(filepath.Join("..", "..", "internal", "ots", "testdata", "pending.ots"))
	if err != nil {
		t.Fatal(err)
	}
	root, raw := full[33:65], full[65:]
	proof := `{"version":1,"snapshot_id":7,"merkle":{"root":"` + hex.EncodeToString(root) + `"},"anchors":[{"receipt_base64":"` + base64.StdEncoding.EncodeToString(raw) + `"}]}`
	path := filepath.Join(t.TempDir(), "proof.json")
	if err := os.WriteFile(path, []byte(proof), 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "out")
	if err := extract(path, out); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(out, "snapshot-7-anchor-1.ots"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(full) {
		t.Fatal("extracted .ots differs from official fixture")
	}
}

func TestExtractRejectsInvalidInput(t *testing.T) {
	cases := []struct{ name, proof, want string }{
		{name: "json", proof: `{`, want: "parse proof"},
		{name: "root", proof: `{"version":1,"merkle":{"root":"not-hex"}}`, want: "decode merkle root"},
		{name: "base64", proof: `{"version":1,"merkle":{"root":"0000000000000000000000000000000000000000000000000000000000000000"},"anchors":[{"receipt_base64":"!"}]}`, want: "decode anchor 0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "proof.json")
			if err := os.WriteFile(path, []byte(tc.proof), 0o600); err != nil {
				t.Fatal(err)
			}
			err := extract(path, t.TempDir())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}
