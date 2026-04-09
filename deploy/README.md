# Deploy — Fé Pública

This directory holds the production deployment recipe for the canonical
Fé Pública instance at `https://fepublica.gmowses.cloud`.

The target is a single VPS already running Traefik as a reverse proxy on a
shared Docker network called `proxy`. If your infrastructure differs, adjust
`compose/docker-compose.prod.yml` accordingly.

## Prerequisites on the VPS

- Docker Engine 24+
- Docker Compose v2
- A running Traefik instance attached to a Docker network named `proxy`
- Traefik entrypoint named `websecure` with a cert resolver named `letsencrypt`
- DNS `A` record pointing `fepublica.gmowses.cloud` to the VPS public IP
- Outbound HTTPS access to:
  - `api.portaldatransparencia.gov.br` (collector)
  - `*.btc.calendar.opentimestamps.org` (anchor)
  - `finney.calendar.eternitywall.com` (anchor)

## First-time setup

```bash
sudo mkdir -p /opt/fepublica
sudo chown $USER:$USER /opt/fepublica
cd /opt/fepublica

git clone https://github.com/gmowses/fepublica .

cp .env.example .env
# Edit .env and set:
#   POSTGRES_PASSWORD=<strong random value>
#   TRANSPARENCIA_API_KEY=<your Portal da Transparência token>
chmod 600 .env

docker compose \
  -f docker-compose.yml \
  -f deploy/compose/docker-compose.prod.yml \
  build

docker compose \
  -f docker-compose.yml \
  -f deploy/compose/docker-compose.prod.yml \
  up -d postgres

# Wait a few seconds for Postgres to initialize (schema is applied automatically
# from db/migrations by the entrypoint).
sleep 10

docker compose \
  -f docker-compose.yml \
  -f deploy/compose/docker-compose.prod.yml \
  up -d api anchor
```

The collector is deliberately **not** started on first boot — it runs a long
initial fetch and you should trigger the first run manually so you can watch
the logs:

```bash
docker compose \
  -f docker-compose.yml \
  -f deploy/compose/docker-compose.prod.yml \
  run --rm collector run --source cnep
```

Then enable the scheduled collector:

```bash
docker compose \
  -f docker-compose.yml \
  -f deploy/compose/docker-compose.prod.yml \
  up -d collector
```

## Verification

```bash
curl -s https://fepublica.gmowses.cloud/health | jq .
curl -s https://fepublica.gmowses.cloud/sources | jq .
curl -s https://fepublica.gmowses.cloud/snapshots | jq .
```

## Updating

```bash
cd /opt/fepublica
git pull
docker compose \
  -f docker-compose.yml \
  -f deploy/compose/docker-compose.prod.yml \
  build

docker compose \
  -f docker-compose.yml \
  -f deploy/compose/docker-compose.prod.yml \
  up -d
```

## Logs

```bash
docker compose logs -f api
docker compose logs -f collector
docker compose logs -f anchor
```

## Upgrading OTS receipts

Calendars take a few hours (up to ~24h) to commit their aggregation batch to
Bitcoin. Once that's happened, the anchor worker's internal upgrade routine
will fetch the upgraded receipts on its next run.

If you want to force an immediate upgrade pass:

```bash
docker compose \
  -f docker-compose.yml \
  -f deploy/compose/docker-compose.prod.yml \
  run --rm anchor --once
```

## Backups

The Postgres volume `fepublica_postgres_data` is the only stateful asset. Back
it up with your usual VPS backup tooling (rsnapshot, restic, borg, etc).
Losing the volume does not destroy the anchored proofs (they are in Bitcoin),
but it does destroy the index from external_id to proof, which is expensive
to rebuild (you'd need to re-crawl every snapshot).
