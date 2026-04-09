#!/usr/bin/env bash
#
# Daily backup of the fepublica Postgres volume.
# Intended to run via cron on the VPS hosting the instance.
#
# Usage:
#   ./scripts/backup.sh [out_dir]
#
# Defaults to /var/backups/fepublica. Creates out_dir if missing. Rotates
# backups keeping the last 14 daily dumps.

set -euo pipefail

OUT_DIR="${1:-/var/backups/fepublica}"
RETAIN_DAYS="${RETAIN_DAYS:-14}"
COMPOSE_DIR="${COMPOSE_DIR:-/opt/fepublica}"

mkdir -p "$OUT_DIR"

cd "$COMPOSE_DIR"

TS="$(date -u +%Y%m%dT%H%M%SZ)"
OUT="${OUT_DIR}/fepublica-${TS}.dump"

echo "[$(date -u +%FT%TZ)] dumping Postgres to ${OUT}"
docker compose \
  -f docker-compose.yml \
  -f deploy/compose/docker-compose.prod.yml \
  exec -T postgres \
  pg_dump -U fepublica -d fepublica -Fc \
  > "$OUT"

SIZE_BYTES="$(stat -c '%s' "$OUT" 2>/dev/null || wc -c < "$OUT")"
echo "[$(date -u +%FT%TZ)] dump complete: ${SIZE_BYTES} bytes"

echo "[$(date -u +%FT%TZ)] pruning dumps older than ${RETAIN_DAYS} days"
find "$OUT_DIR" -name 'fepublica-*.dump' -type f -mtime +"$RETAIN_DAYS" -print -delete

echo "[$(date -u +%FT%TZ)] done"
