// Package transparencia provides a typed HTTP client for the Brazilian Portal da
// Transparência public API.
//
// Endpoints supported in the MVP:
//
//   - /api-de-dados/ceis  (CEIS sub-package)
//   - /api-de-dados/cnep  (CNEP sub-package)
//
// The client is conservative about rate limiting and retries, following the
// official policy of 90 req/min during the day and 300 req/min between
// 00:00–05:59 BRT. See docs/DESIGN.md for details.
package transparencia

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	// DefaultBaseURL is the public root of the API.
	DefaultBaseURL = "https://api.portaldatransparencia.gov.br"

	// APIKeyHeader is the header name expected by the API.
	APIKeyHeader = "chave-api-dados"
)

// Client is a low-level HTTP client for the Portal da Transparência API.
// It handles authentication, rate limiting, retries, and basic JSON decoding.
// Specific resources (CEIS, CNEP, etc.) live in sub-packages that use this client.
type Client struct {
	baseURL    string
	apiKey     string
	userAgent  string
	httpClient *http.Client
	limiter    Limiter
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithHTTPClient overrides the default http.Client.
func WithHTTPClient(h *http.Client) ClientOption {
	return func(c *Client) { c.httpClient = h }
}

// WithBaseURL overrides the default base URL (useful for tests).
func WithBaseURL(u string) ClientOption {
	return func(c *Client) { c.baseURL = u }
}

// WithUserAgent sets a custom User-Agent header.
func WithUserAgent(ua string) ClientOption {
	return func(c *Client) { c.userAgent = ua }
}

// WithLimiter replaces the internal rate limiter. Use this in tests to remove
// waiting, or in production to enforce a stricter policy.
func WithLimiter(l Limiter) ClientOption {
	return func(c *Client) { c.limiter = l }
}

// New creates a Client with the given API key.
func New(apiKey string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL:   DefaultBaseURL,
		apiKey:    apiKey,
		userAgent: "fepublica/0.1 (+https://github.com/gmowses/fepublica)",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		limiter: NewSleepLimiter(1 * time.Second), // 60 req/min, well under diurnal 90/min
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Get issues a GET request to path (without base URL), with the given query params,
// and decodes the JSON response into out.
//
// Errors returned:
//   - ErrRateLimited when the server responds 429.
//   - ErrUnauthorized when 401/403.
//   - a generic wrapped error for other statuses.
func (c *Client) Get(ctx context.Context, path string, query url.Values, out any) error {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return fmt.Errorf("transparencia: parse url: %w", err)
	}
	if query != nil {
		u.RawQuery = query.Encode()
	}

	if err := c.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("transparencia: rate limit wait: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("transparencia: build request: %w", err)
	}
	req.Header.Set(APIKeyHeader, c.apiKey)
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("transparencia: do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 50<<20)) // 50 MiB ceiling
	if err != nil {
		return fmt.Errorf("transparencia: read body: %w", err)
	}

	switch {
	case resp.StatusCode == http.StatusOK:
		if len(body) == 0 || out == nil {
			return nil
		}
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("transparencia: decode body: %w", err)
		}
		return nil

	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
		return fmt.Errorf("%w: status %d, body: %s",
			ErrUnauthorized, resp.StatusCode, truncate(string(body), 200))

	case resp.StatusCode == http.StatusTooManyRequests:
		return fmt.Errorf("%w: status %d", ErrRateLimited, resp.StatusCode)

	default:
		return fmt.Errorf("transparencia: unexpected status %d, body: %s",
			resp.StatusCode, truncate(string(body), 200))
	}
}

// Errors returned by the client.
var (
	ErrUnauthorized = errors.New("transparencia: unauthorized")
	ErrRateLimited  = errors.New("transparencia: rate limited")
)

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
