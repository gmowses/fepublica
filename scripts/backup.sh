#!/usr/bin/env bash
# Create an atomic Postgres custom-format backup from the Fé Pública stack.
set -euo pipefail
umask 077

backup_dir="${FEPUBLICA_BACKUP_DIR:-/var/backups/fepublica}"
retention_days="${FEPUBLICA_BACKUP_RETENTION_DAYS:-14}"
database="${POSTGRES_DB:-fepublica}"
database_user="${POSTGRES_USER:-fepublica}"

if ! [[ "$retention_days" =~ ^[0-9]+$ ]]; then
  echo "FEPUBLICA_BACKUP_RETENTION_DAYS must be a non-negative integer" >&2
  exit 2
fi

mkdir -p "$backup_dir"
timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
final_path="$backup_dir/fepublica-postgres-$timestamp.dump"
temporary_path="$(mktemp "$backup_dir/.fepublica-postgres-$timestamp.XXXXXX.dump")"
cleanup() { rm -f "$temporary_path"; }
trap cleanup EXIT

# In Swarm the service label is exact and selects only the current task. The
# Compose fallback keeps the script useful for a local recovery environment.
container_id="$(docker ps --filter 'label=com.docker.swarm.service.name=fepublica_postgres' --format '{{.ID}}' | head -n 1)"
if [[ -z "$container_id" ]]; then
  compose_file="${COMPOSE_FILE:-docker-compose.yml}"
  container_id="$(docker compose -f "$compose_file" ps -q postgres 2>/dev/null | head -n 1 || true)"
fi
if [[ -z "$container_id" ]]; then
  echo "Postgres container not found for fepublica_postgres or Compose service postgres" >&2
  exit 1
fi

docker exec "$container_id" pg_dump -U "$database_user" -d "$database" -Fc > "$temporary_path"
if [[ ! -s "$temporary_path" ]]; then
  echo "pg_dump produced an empty backup" >&2
  exit 1
fi

# Validate before publishing it. The image is deliberately pinned to the
# major version used by the stack so pg_restore understands its dump format.
docker run --rm \
  --mount "type=bind,src=$backup_dir,dst=/backups,readonly" \
  postgres:16-alpine \
  pg_restore --list "/backups/$(basename "$temporary_path")" >/dev/null

mv "$temporary_path" "$final_path"
trap - EXIT
echo "backup created: $final_path"

# Do not touch files outside this script's own backup naming pattern.
find "$backup_dir" -maxdepth 1 -type f -name 'fepublica-postgres-*.dump' -mtime "+$retention_days" -delete
