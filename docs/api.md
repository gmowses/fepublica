# Fé Pública — API HTTP

Todos os endpoints são públicos, sem autenticação, sem estado no cliente, e retornam JSON (ou HTML na raiz para browsers).

Base URL da instância oficial: `https://fepublica.gmowses.cloud`

## Convenções

- Respostas JSON usam `Content-Type: application/json; charset=utf-8`.
- Todos os timestamps são RFC 3339 em UTC.
- IDs de snapshots são inteiros positivos (`BIGSERIAL` no Postgres).
- IDs externos (`external_id`) são strings opacas cujo formato depende da fonte.
- Hashes são SHA-256 representados em hex minúsculo sem prefixo (o CLI adiciona `sha256:` ao exibir).

## Endpoints

### `GET /`

Página inicial. Se o cliente envia `Accept: text/html`, retorna a landing page HTML embutida. Caso contrário, retorna um JSON de índice com metadados do serviço, mapa de endpoints e estatísticas agregadas.

### `GET /health`

Health check simples. Usado pelo healthcheck do container.

```json
{
  "status": "ok",
  "version": "0.1.0",
  "uptime": "1h23m45s",
  "now": "2026-04-09T16:22:27Z"
}
```

### `GET /sources`

Lista de fontes configuradas.

```json
{
  "sources": [
    {
      "id": "ceis",
      "name": "Cadastro de Empresas Inidôneas e Suspensas",
      "base_url": "https://api.portaldatransparencia.gov.br/api-de-dados/ceis",
      "description": "...",
      "created_at": "2026-04-09T00:00:00Z"
    }
  ]
}
```

### `GET /snapshots`

Lista de snapshots, mais recentes primeiro.

Parâmetros de query:

- `source` (opcional) — filtra por `source_id`.
- `limit` (opcional, default 50, max 500) — limita o número de resultados.

```json
{
  "snapshots": [
    {
      "id": 1,
      "source_id": "cnep",
      "collected_at": "2026-04-09T15:50:54Z",
      "record_count": 1623,
      "bytes_size": 3814098,
      "merkle_root": "a8c24fa6...",
      "merkle_computed_at": "2026-04-09T15:50:56Z",
      "collector_version": "0.1.0"
    }
  ]
}
```

### `GET /snapshots/{id}`

Detalhes de um snapshot específico. 404 se não existir.

### `GET /snapshots/{id}/anchors`

Lista de âncoras OpenTimestamps associadas ao snapshot.

```json
{
  "anchors": [
    {
      "id": 1,
      "snapshot_id": 1,
      "calendar_url": "https://alice.btc.calendar.opentimestamps.org",
      "submitted_at": "2026-04-09T15:51:58Z",
      "upgraded": false,
      "receipt_bytes": 172
    }
  ]
}
```

Quando `upgraded` é `true`, os campos `upgraded_at` e `block_height` estão presentes indicando em que bloco Bitcoin o receipt foi confirmado.

### `GET /snapshots/{id}/diff/{other_id}`

Diferença semântica entre dois snapshots da mesma fonte. Erra com 400 se as fontes forem diferentes.

```json
{
  "source_id": "ceis",
  "snapshot_a": { "id": 1, "collected_at": "2026-04-01T04:00:00Z", "record_count": 25301 },
  "snapshot_b": { "id": 2, "collected_at": "2026-04-02T04:00:00Z", "record_count": 25298 },
  "summary": { "added": 3, "removed": 6, "changed": 12 },
  "added":   [ { "external_id": "...", "hash_b": "..." } ],
  "removed": [ { "external_id": "...", "hash_a": "..." } ],
  "changed": [ { "external_id": "...", "hash_a": "...", "hash_b": "..." } ]
}
```

- **added**: presente em `b`, ausente em `a`.
- **removed**: presente em `a`, ausente em `b`.
- **changed**: presente em ambos, com `content_hash` diferente — indica edição silenciosa do registro entre as duas coletas.

### `GET /snapshots/{id}/events/{external_id}/proof`

Retorna a prova completa de um evento individual dentro de um snapshot, no formato consumido pelo `fepublica-verify` CLI.

```json
{
  "version": 1,
  "source_id": "cnep",
  "snapshot_id": 1,
  "snapshot_collected_at": "2026-04-09T15:50:54Z",
  "event": {
    "external_id": "277765",
    "content_hash": "febff87b...",
    "canonical_json": { "...": "..." }
  },
  "merkle": {
    "root": "a8c24fa6...",
    "index": 0,
    "siblings": [
      { "sibling": "...", "side": "right" }
    ]
  },
  "anchors": [
    {
      "calendar_url": "https://alice.btc.calendar.opentimestamps.org",
      "receipt_base64": "...",
      "upgraded": false,
      "submitted_at": "2026-04-09T15:51:58Z"
    }
  ],
  "generated_at": "2026-04-09T16:00:00Z"
}
```

Esta é a unidade de verificação — leia [`verification.md`](./verification.md) para detalhes sobre como consumi-la.

### `GET /metrics`

Endpoint Prometheus no formato texto padrão. Métricas expostas:

- `fepublica_collector_runs_total{source,status}` — total de execuções por fonte e resultado.
- `fepublica_collector_records_total{source}` — total de registros persistidos.
- `fepublica_collector_run_duration_seconds{source}` — histograma de duração.
- `fepublica_anchor_merkle_roots_total` — total de raízes computadas.
- `fepublica_anchor_submit_total{calendar,status}` — submissões a calendars.
- `fepublica_anchor_upgrade_total{calendar,status}` — upgrades de receipts.
- `fepublica_anchor_pending_upgrade` — gauge de anchors aguardando Bitcoin.
- `fepublica_anchor_snapshots_without_root` — gauge de snapshots sem raiz.
- `fepublica_api_requests_total{route,status}` — contador HTTP por rota.
- `fepublica_api_request_duration_seconds{route}` — histograma de latência HTTP.

Rotas no label são normalizadas (ids numéricos viram `{id}`, ids opacos longos viram `{ext}`) para manter cardinalidade controlada.

## Códigos de erro

- **400 Bad Request** — parâmetro inválido.
- **404 Not Found** — recurso inexistente.
- **409 Conflict** — snapshot ainda sem Merkle root quando prova é solicitada.
- **500 Internal Server Error** — erro inesperado (logs do servidor trazem detalhes).

## Política de uso

A API é pública e não rate-limitada pelo servidor fepublica (o reverse proxy Traefik na frente impõe limites gerais se necessário). Se você pretende consumir em escala:

1. Seja um bom cidadão — não mande mais do que ~10 req/s sustentados.
2. Cache localmente os resultados.
3. Respeite os `Last-Modified` e `Cache-Control` quando presentes (v0.2 os adicionará).
4. Identifique-se no `User-Agent` com um email ou URL de contato.

Contribuições de melhorias nessa API são bem-vindas — veja [`CONTRIBUTING.md`](../CONTRIBUTING.md).
