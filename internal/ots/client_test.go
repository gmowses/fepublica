package ots

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestUpgradeRequiresBitcoinAttestation(t *testing.T) {
	digest := make([]byte, 32)
	cases := []struct {
		name string
		body []byte
		want int
		err  error
	}{
		{name: "malformed", body: []byte{0x00}, err: errAny},
		{name: "pending", body: pendingReceipt("https://calendar.example"), err: ErrNotReady},
		{name: "bitcoin", body: bitcoinReceipt(840_000), want: 840_000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(tc.body) }))
			defer srv.Close()
			got, height, err := NewClient().Upgrade(context.Background(), srv.URL, digest)
			if tc.err == ErrNotReady {
				if !errors.Is(err, ErrNotReady) {
					t.Fatalf("error = %v, want ErrNotReady", err)
				}
				return
			}
			if tc.err != nil {
				if err == nil {
					t.Fatal("malformed receipt returned nil error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if height == nil || *height != tc.want {
				t.Fatalf("height = %v, want %d", height, tc.want)
			}
			if string(got) != string(tc.body) {
				t.Fatal("upgrade changed receipt")
			}
		})
	}
}

func TestValidateTimestampRejectsBoundsAndUnknownOperations(t *testing.T) {
	digest := make([]byte, 32)
	if _, err := ValidateTimestamp(digest, append([]byte{0x99}, bitcoinReceipt(1)...)); err == nil {
		t.Fatal("unknown operation was accepted")
	}
	if _, err := ValidateTimestamp(digest, append([]byte{0x02}, bitcoinReceipt(1)...)); err == nil {
		t.Fatal("Bitcoin attestation after SHA-1 was accepted")
	}
	deep := bitcoinReceipt(1)
	for range maxDepth {
		deep = append([]byte{0xf2}, deep...)
	}
	if _, err := ValidateTimestamp(digest, deep); err == nil {
		t.Fatal("over-deep receipt was accepted")
	}
	oversized := make([]byte, maxReceiptBytes+1)
	if _, err := ValidateTimestamp(digest, oversized); err == nil {
		t.Fatal("oversized receipt was accepted")
	}
}

func TestWrapDetachedTimestampV1(t *testing.T) {
	digest := make([]byte, 32)
	raw := pendingReceipt("https://calendar.example")
	got, err := WrapDetachedTimestamp(digest, raw)
	if err != nil {
		t.Fatal(err)
	}
	wantPrefix := append(append(append([]byte{}, detachedMagic...), 1, sha256OpTag), digest...)
	if string(got[:len(wantPrefix)]) != string(wantPrefix) {
		t.Fatal("wrong DetachedTimestampFile prefix")
	}
	if string(got[len(wantPrefix):]) != string(raw) {
		t.Fatal("raw timestamp was not preserved")
	}
}

func TestGoldenDetachedTimestampFiles(t *testing.T) {
	// Fixtures are byte-for-byte copies of opentimestamps-client examples
	// (MIT License): incomplete.txt.ots and hello-world.txt.ots at
	// https://github.com/opentimestamps/opentimestamps-client/tree/master/examples
	cases := []struct {
		name        string
		file        string
		bitcoin     bool
		blockHeight int
	}{
		{name: "pending", file: "pending.ots"},
		{name: "bitcoin", file: "bitcoin.ots", bitcoin: true, blockHeight: 358391},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			full, err := os.ReadFile(filepath.Join("testdata", tc.file))
			if err != nil {
				t.Fatal(err)
			}
			root, raw := full[33:65], full[65:]
			info, err := ValidateTimestamp(root, raw)
			if err != nil {
				t.Fatal(err)
			}
			if (info.BitcoinBlockHeight != nil) != tc.bitcoin {
				t.Fatalf("Bitcoin attestation = %v, want %v", info.BitcoinBlockHeight != nil, tc.bitcoin)
			}
			if tc.bitcoin && *info.BitcoinBlockHeight != tc.blockHeight {
				t.Fatalf("Bitcoin block height = %d, want %d", *info.BitcoinBlockHeight, tc.blockHeight)
			}
			wrapped, err := WrapDetachedTimestamp(root, raw)
			if err != nil {
				t.Fatal(err)
			}
			if string(wrapped) != string(full) {
				t.Fatal("DetachedTimestampFile did not round trip")
			}
		})
	}
}

var errAny = errors.New("any error")

func pendingReceipt(uri string) []byte {
	inner := append(varuint(uint64(len(uri))), []byte(uri)...)
	return attestation(pendingAttestationTag, inner)
}

func bitcoinReceipt(height uint64) []byte { return attestation(bitcoinAttestationTag, varuint(height)) }
func attestation(tag [8]byte, payload []byte) []byte {
	out := []byte{0x00}
	out = append(out, tag[:]...)
	out = append(out, varuint(uint64(len(payload)))...)
	return append(out, payload...)
}
func varuint(v uint64) []byte {
	out := []byte{}
	for {
		b := byte(v & 0x7f)
		v >>= 7
		if v != 0 {
			b |= 0x80
		}
		out = append(out, b)
		if v == 0 {
			return out
		}
	}
}
