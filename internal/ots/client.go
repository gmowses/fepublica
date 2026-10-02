// Package ots implements a bounded OpenTimestamps calendar client.
package ots

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/ripemd160" //nolint:staticcheck // required to parse the published OTS operation set.
	"golang.org/x/crypto/sha3"
)

const (
	maxReceiptBytes = 1 << 20
	maxDepth        = 256
	maxNodes        = 10_000
	maxMessageBytes = 4096
	maxPayloadBytes = 8192
	sha256OpTag     = 0x08
)

var (
	detachedMagic         = []byte{0x00, 'O', 'p', 'e', 'n', 'T', 'i', 'm', 'e', 's', 't', 'a', 'm', 'p', 's', 0x00, 0x00, 'P', 'r', 'o', 'o', 'f', 0x00, 0xbf, 0x89, 0xe2, 0xe8, 0x84, 0xe8, 0x92, 0x94}
	bitcoinAttestationTag = [8]byte{0x05, 0x88, 0x96, 0x0d, 0x73, 0xd7, 0x19, 0x01}
	pendingAttestationTag = [8]byte{0x83, 0xdf, 0xe3, 0x0d, 0x2e, 0xf9, 0x0c, 0x8e}
)

// Client submits digests to and fetches upgrades from an OTS calendar server.
type Client struct {
	httpClient *http.Client
	userAgent  string
}

// NewClient builds a Client with sensible defaults.
func NewClient() *Client {
	return &Client{httpClient: &http.Client{Timeout: 30 * time.Second}, userAgent: "fepublica/0.1 (+https://github.com/gmowses/fepublica)"}
}

// Submit posts a SHA-256 digest and returns the raw Timestamp emitted by the
// calendar. Calendar responses are not DetachedTimestampFile blobs.
func (c *Client) Submit(ctx context.Context, calendarURL string, digest []byte) ([]byte, error) {
	if len(digest) != 32 {
		return nil, fmt.Errorf("ots: digest must be 32 bytes, got %d", len(digest))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(calendarURL, "/")+"/digest", bytes.NewReader(digest))
	if err != nil {
		return nil, fmt.Errorf("ots: build submit request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("User-Agent", c.userAgent)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ots: submit %s: %w", calendarURL, err)
	}
	defer resp.Body.Close()
	body, err := readReceipt(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ots: read submit response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ots: submit %s: status %d, body: %s", calendarURL, resp.StatusCode, truncate(string(body), 200))
	}
	if _, err := ValidateTimestamp(digest, body); err != nil {
		return nil, fmt.Errorf("ots: invalid submit receipt: %w", err)
	}
	return body, nil
}

// Upgrade returns ErrNotReady until a Bitcoin block-header attestation exists.
// Presence of that attestation still needs reference CLI verification against Bitcoin.
func (c *Client) Upgrade(ctx context.Context, calendarURL string, digest []byte) ([]byte, *int, error) {
	if len(digest) != 32 {
		return nil, nil, fmt.Errorf("ots: digest must be 32 bytes, got %d", len(digest))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(calendarURL, "/")+"/timestamp/"+hex.EncodeToString(digest), nil)
	if err != nil {
		return nil, nil, fmt.Errorf("ots: build upgrade request: %w", err)
	}
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("User-Agent", c.userAgent)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("ots: upgrade %s: %w", calendarURL, err)
	}
	defer resp.Body.Close()
	body, err := readReceipt(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("ots: read upgrade response: %w", err)
	}
	switch resp.StatusCode {
	case http.StatusOK:
		info, err := ValidateTimestamp(digest, body)
		if err != nil {
			return nil, nil, fmt.Errorf("ots: invalid upgrade receipt: %w", err)
		}
		if info.BitcoinBlockHeight == nil {
			return nil, nil, ErrNotReady
		}
		return body, info.BitcoinBlockHeight, nil
	case http.StatusNotFound:
		return nil, nil, ErrNotReady
	default:
		return nil, nil, fmt.Errorf("ots: upgrade %s: status %d, body: %s", calendarURL, resp.StatusCode, truncate(string(body), 200))
	}
}

// TimestampInfo describes the locally parsed, unverified receipt.
type TimestampInfo struct{ BitcoinBlockHeight *int }

// ValidateTimestamp validates a raw OTS Timestamp. It deliberately does not
// verify attestations against Bitcoin; use the reference ots CLI for that.
func ValidateTimestamp(digest, raw []byte) (TimestampInfo, error) {
	if len(digest) != 32 {
		return TimestampInfo{}, fmt.Errorf("digest must be 32 bytes, got %d", len(digest))
	}
	if len(raw) == 0 {
		return TimestampInfo{}, errors.New("empty receipt")
	}
	if len(raw) > maxReceiptBytes {
		return TimestampInfo{}, fmt.Errorf("receipt exceeds %d bytes", maxReceiptBytes)
	}
	p := timestampParser{data: raw}
	if err := p.timestamp(digest, 0); err != nil {
		return TimestampInfo{}, err
	}
	if p.pos != len(raw) {
		return TimestampInfo{}, errors.New("trailing receipt data")
	}
	return TimestampInfo{BitcoinBlockHeight: p.bitcoinHeight}, nil
}

// WrapDetachedTimestamp produces an interoperable OTS v1 DetachedTimestampFile.
func WrapDetachedTimestamp(digest, raw []byte) ([]byte, error) {
	if _, err := ValidateTimestamp(digest, raw); err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(detachedMagic)+2+len(digest)+len(raw))
	out = append(out, detachedMagic...)
	out = append(out, 1, sha256OpTag)
	out = append(out, digest...)
	out = append(out, raw...)
	return out, nil
}

type timestampParser struct {
	data          []byte
	pos           int
	nodes         int
	bitcoinHeight *int
}

func (p *timestampParser) timestamp(message []byte, depth int) error {
	if depth >= maxDepth {
		return fmt.Errorf("receipt exceeds timestamp depth %d", maxDepth)
	}
	if len(message) == 0 || len(message) > maxMessageBytes {
		return fmt.Errorf("invalid operation message length %d", len(message))
	}
	for {
		tag, err := p.byte()
		if err != nil {
			return err
		}
		more := tag == 0xff
		if more {
			tag, err = p.byte()
			if err != nil {
				return err
			}
		}
		p.nodes++
		if p.nodes > maxNodes {
			return fmt.Errorf("receipt exceeds timestamp node limit %d", maxNodes)
		}
		if tag == 0x00 {
			err = p.attestation(message)
		} else {
			var result []byte
			result, err = p.operation(tag, message)
			if err == nil {
				err = p.timestamp(result, depth+1)
			}
		}
		if err != nil {
			return err
		}
		if !more {
			return nil
		}
	}
}

func (p *timestampParser) operation(tag byte, message []byte) ([]byte, error) {
	switch tag {
	case 0x02:
		h := sha1.Sum(message)
		return h[:], nil
	case 0x03:
		h := ripemd160.New()
		_, _ = h.Write(message)
		return h.Sum(nil), nil
	case 0x08:
		h := sha256.Sum256(message)
		return h[:], nil
	case 0x67:
		h := sha3.NewLegacyKeccak256()
		_, _ = h.Write(message)
		return h.Sum(nil), nil
	case 0xf2:
		out := append([]byte(nil), message...)
		for i := 0; i < len(out)/2; i++ {
			out[i], out[len(out)-1-i] = out[len(out)-1-i], out[i]
		}
		return out, nil
	case 0xf3:
		if len(message) > maxMessageBytes/2 {
			return nil, errors.New("hexlify result exceeds limit")
		}
		return []byte(hex.EncodeToString(message)), nil
	case 0xf0, 0xf1:
		arg, err := p.varbytes(maxMessageBytes)
		if err != nil {
			return nil, err
		}
		if len(arg) == 0 {
			return nil, errors.New("empty append/prepend argument")
		}
		if len(message)+len(arg) > maxMessageBytes {
			return nil, errors.New("append/prepend result exceeds limit")
		}
		out := make([]byte, 0, len(message)+len(arg))
		if tag == 0xf0 {
			out = append(out, message...)
			out = append(out, arg...)
		} else {
			out = append(out, arg...)
			out = append(out, message...)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported operation tag 0x%02x", tag)
	}
}

func (p *timestampParser) attestation(message []byte) error {
	if len(p.data)-p.pos < 8 {
		return errors.New("truncated attestation tag")
	}
	var tag [8]byte
	copy(tag[:], p.data[p.pos:p.pos+8])
	p.pos += 8
	payload, err := p.varbytes(maxPayloadBytes)
	if err != nil {
		return err
	}
	switch tag {
	case bitcoinAttestationTag:
		if len(message) != sha256.Size {
			return errors.New("bitcoin attestation has non-SHA256 commitment")
		}
		height, n, err := readVaruint(payload)
		if err != nil || n != len(payload) {
			return errors.New("invalid Bitcoin block height")
		}
		if height > uint64(^uint(0)>>1) {
			return errors.New("bitcoin block height overflows int")
		}
		h := int(height)
		p.bitcoinHeight = &h
	case pendingAttestationTag:
		uri, n, err := readVaruint(payload)
		if err != nil || uri > 1000 || n+int(uri) != len(payload) {
			return errors.New("invalid pending attestation URI")
		}
		for _, b := range payload[n:] {
			if !isURIByte(b) {
				return errors.New("invalid pending attestation URI")
			}
		}
	}
	return nil
}

func isURIByte(b byte) bool {
	return b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b >= '0' && b <= '9' || strings.ContainsRune("-._/:", rune(b))
}

func (p *timestampParser) byte() (byte, error) {
	if p.pos == len(p.data) {
		return 0, errors.New("truncated receipt")
	}
	b := p.data[p.pos]
	p.pos++
	return b, nil
}
func (p *timestampParser) varbytes(max int) ([]byte, error) {
	n, used, err := readVaruint(p.data[p.pos:])
	if err != nil {
		return nil, err
	}
	if n > uint64(max) || n > uint64(len(p.data)-p.pos-used) {
		return nil, errors.New("invalid variable-length payload")
	}
	p.pos += used
	out := p.data[p.pos : p.pos+int(n)]
	p.pos += int(n)
	return out, nil
}
func readVaruint(data []byte) (uint64, int, error) {
	var v uint64
	for i, b := range data {
		if i == 10 || (i == 9 && b > 1) {
			return 0, 0, errors.New("variable-length integer overflow")
		}
		v |= uint64(b&0x7f) << (7 * i)
		if b&0x80 == 0 {
			return v, i + 1, nil
		}
	}
	return 0, 0, errors.New("truncated variable-length integer")
}
func readReceipt(r io.Reader) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, maxReceiptBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxReceiptBytes {
		return nil, fmt.Errorf("receipt exceeds %d bytes", maxReceiptBytes)
	}
	return body, nil
}

// ErrNotReady indicates that a Bitcoin attestation has not been returned yet.
var ErrNotReady = errors.New("ots: upgrade not ready yet")

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
