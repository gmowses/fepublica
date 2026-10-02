package store

import (
	"context"
	"fmt"
)

const (
	ProjectionContratos = "contratos"
	ProjectionCartoes   = "cartoes"
	ProjectionCEAP      = "ceap"

	projectionMaxAttempts = 3
)

// ProjectionFailure records the retry state of a raw event that could not be
// materialized. Terminal failures are retained for audit and can be requeued
// after an operational correction.
type ProjectionFailure struct {
	Attempts int
	Terminal bool
}

// RecordProjectionFailure schedules a bounded retry. The retry delay grows
// from five to ten minutes before the third failure becomes terminal.
func (s *Store) RecordProjectionFailure(
	ctx context.Context,
	projection string,
	eventID int64,
	cause error,
) (ProjectionFailure, error) {
	if cause == nil {
		return ProjectionFailure{}, fmt.Errorf("store: projection failure cause is required")
	}
	var failure ProjectionFailure
	err := s.pool.QueryRow(ctx, `
		INSERT INTO projection_failures (
			projection, event_id, attempts, first_failed_at, last_failed_at,
			next_retry_at, error_message
		)
		VALUES ($1, $2, 1, NOW(), NOW(), NOW() + INTERVAL '5 minutes', $3)
		ON CONFLICT (projection, event_id) DO UPDATE
		SET attempts = projection_failures.attempts + 1,
			last_failed_at = NOW(),
			next_retry_at = CASE
				WHEN projection_failures.attempts + 1 >= $4 THEN NULL
				ELSE NOW() + (LEAST(2, projection_failures.attempts) * INTERVAL '5 minutes')
			END,
			terminal_at = CASE
				WHEN projection_failures.attempts + 1 >= $4 THEN NOW()
				ELSE NULL
			END,
			resolved_at = NULL,
			error_message = EXCLUDED.error_message
		RETURNING attempts, terminal_at IS NOT NULL
	`, projection, eventID, cause.Error(), projectionMaxAttempts).Scan(&failure.Attempts, &failure.Terminal)
	if err != nil {
		return ProjectionFailure{}, fmt.Errorf("store: record %s projection failure for event %d: %w", projection, eventID, err)
	}
	return failure, nil
}

// ResolveProjectionFailure keeps the failure audit trail while removing an
// event from retry scheduling after a successful projection.
func (s *Store) ResolveProjectionFailure(ctx context.Context, projection string, eventID int64) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE projection_failures
		SET resolved_at = NOW(), next_retry_at = NULL
		WHERE projection = $1 AND event_id = $2 AND resolved_at IS NULL
	`, projection, eventID)
	if err != nil {
		return fmt.Errorf("store: resolve %s projection failure for event %d: %w", projection, eventID, err)
	}
	return nil
}

// RequeueProjectionFailure starts a new bounded retry cycle for a terminal
// failure after an operator has corrected the projection code or source data.
func (s *Store) RequeueProjectionFailure(ctx context.Context, projection string, eventID int64) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE projection_failures
		SET attempts = 0, terminal_at = NULL, resolved_at = NULL, next_retry_at = NOW()
		WHERE projection = $1 AND event_id = $2
	`, projection, eventID)
	if err != nil {
		return fmt.Errorf("store: requeue %s projection failure for event %d: %w", projection, eventID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("store: projection failure for %s event %d not found", projection, eventID)
	}
	return nil
}
