// Package ots implements a minimal HTTP client for OpenTimestamps calendar servers.
//
// We only use two operations:
//
//  1. Submit: POST <calendar>/digest with the raw 32-byte digest, receive a
//     timestamp proof blob.
//  2. Upgrade: GET <calendar>/timestamp/<hex-digest> to get the fully-upgraded
//     proof once the calendar has anchored the corresponding commitment into
//     Bitcoin.
//
// The proof format is opaque to fepublica — we store the bytes as-is and
// delegate full cryptographic verification to the reference `ots` CLI or an
// equivalent library during the verify step.
package ots

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client submits digests to and fetches upgrades from an OTS calendar server.
type Client struct {
	httpClient *http.Client
	userAgent  string
}

// NewClient builds a Client with sensible defaults.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		userAgent:  "fepublica/0.1 (+https://github.com/gmowses/fepublica)",
	}
}

// Submit posts the given 32-byte digest to the calendar and returns the proof
// blob returned by the calendar (the "pending" receipt).
func (c *Client) Submit(ctx context.Context, calendarURL string, digest []byte) ([]byte, error) {
	if len(digest) != 32 {
		return nil, fmt.Errorf("ots: digest must be 32 bytes, got %d", len(digest))
	}

	base := strings.TrimRight(calendarURL, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/digest", bytes.NewReader(digest))
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

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MiB ceiling
	if err != nil {
		return nil, fmt.Errorf("ots: read submit response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ots: submit %s: status %d, body: %s",
			calendarURL, resp.StatusCode, truncate(string(body), 200))
	}
	if len(body) == 0 {
		return nil, errors.New("ots: empty receipt from calendar")
	}
	return body, nil
}

// Upgrade asks the calendar for the fully-upgraded proof of the given digest.
// Returns the new receipt bytes if available. Returns ErrNotReady if the
// calendar has not yet committed the digest to Bitcoin.
func (c *Client) Upgrade(ctx context.Context, calendarURL string, digest []byte) ([]byte, error) {
	if len(digest) != 32 {
		return nil, fmt.Errorf("ots: digest must be 32 bytes, got %d", len(digest))
	}
	base := strings.TrimRight(calendarURL, "/")
	u := base + "/timestamp/" + hex.EncodeToString(digest)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("ots: build upgrade request: %w", err)
	}
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ots: upgrade %s: %w", calendarURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("ots: read upgrade response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		if len(body) == 0 {
			return nil, errors.New("ots: empty upgrade receipt")
		}
		return body, nil
	case http.StatusNotFound:
		return nil, ErrNotReady
	default:
		return nil, fmt.Errorf("ots: upgrade %s: status %d, body: %s",
			calendarURL, resp.StatusCode, truncate(string(body), 200))
	}
}

// ErrNotReady indicates that the calendar server has not yet anchored the
// submitted digest in Bitcoin.
var ErrNotReady = errors.New("ots: upgrade not ready yet")

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
