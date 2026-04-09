package store

import (
	"context"
	"fmt"
)

// ListUnpublishedChangeEvents returns change events where the given column
// (published_rss, published_telegram, published_mastodon, etc.) is FALSE,
// oldest first, up to limit. The caller passes the literal column name; it
// is validated against a whitelist to prevent SQL injection.
func (s *Store) ListUnpublishedChangeEvents(ctx context.Context, column string, limit int) ([]ChangeEvent, error) {
	if !isValidPublishedColumn(column) {
		return nil, fmt.Errorf("store: invalid published column %q", column)
	}
	if limit <= 0 {
		limit = 50
	}
	q := fmt.Sprintf(`
		SELECT id, diff_run_id, source_id, ente_id, external_id, change_type,
		       content_hash_a, content_hash_b, detected_at, severity,
		       published_rss, published_telegram, published_mastodon,
		       published_webhook, published_email
		FROM change_events
		WHERE %s = FALSE
		ORDER BY detected_at ASC
		LIMIT $1
	`, column)
	rows, err := s.pool.Query(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("store: list unpublished: %w", err)
	}
	defer rows.Close()

	var out []ChangeEvent
	for rows.Next() {
		var ce ChangeEvent
		if err := rows.Scan(
			&ce.ID, &ce.DiffRunID, &ce.SourceID, &ce.EnteID, &ce.ExternalID, &ce.ChangeType,
			&ce.ContentHashA, &ce.ContentHashB, &ce.DetectedAt, &ce.Severity,
			&ce.PublishedRSS, &ce.PublishedTelegram, &ce.PublishedMastodon,
			&ce.PublishedWebhook, &ce.PublishedEmail,
		); err != nil {
			return nil, fmt.Errorf("store: scan unpublished: %w", err)
		}
		out = append(out, ce)
	}
	return out, rows.Err()
}

// MarkChangeEventPublished sets the given published_* column to TRUE for one event.
func (s *Store) MarkChangeEventPublished(ctx context.Context, eventID int64, column string) error {
	if !isValidPublishedColumn(column) {
		return fmt.Errorf("store: invalid published column %q", column)
	}
	q := fmt.Sprintf(`UPDATE change_events SET %s = TRUE WHERE id = $1`, column)
	_, err := s.pool.Exec(ctx, q, eventID)
	if err != nil {
		return fmt.Errorf("store: mark published: %w", err)
	}
	return nil
}

func isValidPublishedColumn(c string) bool {
	switch c {
	case "published_rss", "published_telegram", "published_mastodon",
		"published_webhook", "published_email":
		return true
	}
	return false
}
