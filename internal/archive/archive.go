// Package archive is a thin wrapper around minio-go for S3-compatible storage.
//
// It knows only how to:
//
//   - PUT arbitrary bytes at a key
//   - GET bytes back from a key
//   - generate a presigned GET URL for a key
//   - delete a key
//
// No business logic lives here. Callers (collector cold-archive worker,
// backup script, LAI HTML stash) decide when and what to store.
package archive

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Config holds S3-compatible connection parameters.
type Config struct {
	Endpoint  string // e.g. "s3.us-west-1.idrivee2.com"
	Region    string // e.g. "us-west-1"
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool // default true
}

// Client is a ready-to-use wrapper over minio.Client.
type Client struct {
	cfg Config
	mc  *minio.Client
}

// New builds a Client from Config. Errors are returned only for misconfig;
// network errors are deferred to the first operation.
func New(cfg Config) (*Client, error) {
	if cfg.Endpoint == "" || cfg.Bucket == "" || cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, errors.New("archive: missing required config (endpoint, bucket, access key, secret key)")
	}
	// Strip scheme if caller passed a URL-style endpoint.
	endpoint := strings.TrimPrefix(strings.TrimPrefix(cfg.Endpoint, "https://"), "http://")
	useSSL := cfg.UseSSL || !strings.HasPrefix(cfg.Endpoint, "http://")

	mc, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: useSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("archive: minio client: %w", err)
	}
	cfg.UseSSL = useSSL
	return &Client{cfg: cfg, mc: mc}, nil
}

// Put uploads raw bytes to the bucket at the given key.
func (c *Client) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	_, err := c.mc.PutObject(ctx, c.cfg.Bucket, key, body, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("archive: put %s: %w", key, err)
	}
	return nil
}

// PutBytes is a convenience wrapper for small payloads.
func (c *Client) PutBytes(ctx context.Context, key string, body []byte, contentType string) error {
	return c.Put(ctx, key, strings.NewReader(string(body)), int64(len(body)), contentType)
}

// Get returns a reader for the object. Caller must close it.
func (c *Client) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := c.mc.GetObject(ctx, c.cfg.Bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("archive: get %s: %w", key, err)
	}
	return obj, nil
}

// Delete removes an object.
func (c *Client) Delete(ctx context.Context, key string) error {
	return c.mc.RemoveObject(ctx, c.cfg.Bucket, key, minio.RemoveObjectOptions{})
}

// PresignedGet returns a temporary URL for direct download.
func (c *Client) PresignedGet(ctx context.Context, key string, expiry time.Duration) (string, error) {
	u, err := c.mc.PresignedGetObject(ctx, c.cfg.Bucket, key, expiry, url.Values{})
	if err != nil {
		return "", fmt.Errorf("archive: presigned %s: %w", key, err)
	}
	return u.String(), nil
}

// Bucket returns the configured bucket name.
func (c *Client) Bucket() string { return c.cfg.Bucket }
