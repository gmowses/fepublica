# Observatório de Transparência — Design Spec

**Status**: approved
**Date**: 2026-04-09
**Author**: Gabriel Mowses
**License**: AGPL-3.0 (same as fepublica)
**Scope**: program (6 sub-projects, 7 milestones, v0.2.0 → v1.0.0)

## 1. Propósito

O Observatório é a camada de accountability ativa construída sobre o fepublica. Onde o fepublica arquiva e prova integridade, o Observatório detecta mudanças, rastreia conformidade, classifica gaps e distribui alertas. Mesmo stack, mesmo repo, mesmo domínio, mesma filosofia.

**Missão**: responder "o que mudou nos dados públicos brasileiros desde a última vez que você olhou, por que isso importa, e como provar".

## 2. Princípios

1. Incremental, sem big-bang. Cada sub-projeto ganha seu release próprio.
2. Mesmo repo, mesma stack. Go + Postgres + Docker Compose + Vite SPA.
3. Backend primeiro, UI segue.
4. Testes em todo componente novo.
5. Zero dado pessoal fora de inscrições de email (com LGPD compliance).
6. Operational hygiene: métricas, logs JSON, healthchecks, falhas barulhentas.

## 3. Decisões cross-cutting

| Decisão | Valor | Justificativa |
|---|---|---|
| Granularidade de ente | Completo Brasil: 1 União + poderes federais + 27 UFs + 5570 municípios + autarquias/empresas | Cobertura total; rollout faseado no crawler |
| Data model de drift | Híbrido: `diff_runs` (audit) + `change_events` (flat log) | Audit trail independente de volume, queries diretas por ente/tipo, retenção separada |
| Object storage | IDrive E2 S3-compatible, bucket `fepublica` | Cold archive + backup off-site + LAI HTMLs + exports; ~$2/mês pra 500GB |
| LAI crawler | Moderado: GET + parsing HTML + arquivamento, sem POST em e-SIC | ~40k req/mês, baixo risco de ToS/bloqueio, boa cobertura |
| Canais de notificação | RSS + JSON + Telegram + Mastodon + webhook + email | Máximo alcance; RSS/Telegram/Mastodon são zero custo |
| Email provider | Resend (default), SMTP configurável | Modern DX, decent free tier, LGPD compliant |
| Release strategy | Uma tag por sub-projeto (v0.2.0, v0.3.0, ...) | Deploys independentes, rollback cirúrgico, changelogs legíveis |
| UI | SPA única (mesmo app React), lazy-loaded routes | Sem fork, sem duplicação, code splitting por rota |

## 4. Arquitetura

### Novos binários (todos em `cmd/`)

- `driftd` — detector de drift (O1)
- `lai-crawler` — crawler de conformidade LAI (O3)
- `archive` — worker de cold archive pro bucket
- `notifier` — distribuidor de RSS/Telegram/Mastodon/webhook (O4)
- `emailer` — newsletter periódica (O4)
- `enteadm` — CLI one-shot para ingestão/update de entes

### Novos pacotes internos (todos em `internal/`)

- `archive` — wrapper minio-go, presigned URLs, lifecycle
- `entes` — modelo + ingestão IBGE + seed federal + queries
- `drift` — lógica de detecção + severity rules
- `lai` — HTTP client + HTML parser + score calculator
- `notify` — interface Channel + impls por canal
- `email` — Resend wrapper + templates + subscriber management
- `feed` — geração Atom + JSON Feed
- `severity` — regras de escalada info→warn→alert

### Serviços no docker-compose

Um container novo por binário (driftd, lai-crawler, archive, notifier, emailer). Todos herdam `common-env`. Resource limits: default 0.5 CPU / 256 MB; lai-crawler 1 CPU / 1 GB.

## 5. Modelo de dados

### Tabela `entes` (O2)

```sql
CREATE TABLE entes (
    id              TEXT PRIMARY KEY,  -- "fed:uniao", "uf:sp", "mun:3550308"
    nome            TEXT NOT NULL,
    nome_curto      TEXT,
    esfera          TEXT NOT NULL CHECK (esfera IN ('federal','estadual','municipal','distrital')),
    tipo            TEXT NOT NULL CHECK (tipo IN ('uniao','poder','ministerio','autarquia','empresa','uf','municipio','camara','tribunal','mp','outro')),
    poder           TEXT CHECK (poder IN ('executivo','legislativo','judiciario','mp','tc','defensoria')),
    uf              CHAR(2),
    ibge_code       TEXT,
    cnpj            TEXT,
    populacao       INT,
    domain_hint     TEXT,
    parent_id       TEXT REFERENCES entes(id),
    tier            INT NOT NULL DEFAULT 4,
    active          BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ON entes (esfera, uf);
CREATE INDEX ON entes (tier) WHERE active = TRUE;
CREATE INDEX ON entes (ibge_code);
```

Convenção de IDs legíveis (em vez de UUID) para facilitar debug, logs, queries manuais.

**Seeds**:
- Municípios: CSV do IBGE via `servicodados.ibge.gov.br`, ingestão one-shot + refresh mensal.
- UFs: hardcoded (27, nunca muda).
- Federal: YAML curado em `db/seeds/entes-federal.yaml`.

### Tabela `diff_runs` (O1) — auditoria

```sql
CREATE TABLE diff_runs (
    id               BIGSERIAL PRIMARY KEY,
    source_id        TEXT NOT NULL REFERENCES sources(id),
    snapshot_a_id    BIGINT NOT NULL REFERENCES snapshots(id),
    snapshot_b_id    BIGINT NOT NULL REFERENCES snapshots(id),
    added_count      INT NOT NULL DEFAULT 0,
    removed_count    INT NOT NULL DEFAULT 0,
    modified_count   INT NOT NULL DEFAULT 0,
    ran_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    duration_ms      INT NOT NULL,
    UNIQUE (snapshot_a_id, snapshot_b_id)
);
CREATE INDEX ON diff_runs (source_id, ran_at DESC);
CREATE INDEX ON diff_runs (snapshot_b_id);
```

Append-only (trigger). Sempre gravado, mesmo quando não há mudanças.

### Tabela `change_events` (O1) — flat log

```sql
CREATE TABLE change_events (
    id                 BIGSERIAL PRIMARY KEY,
    diff_run_id        BIGINT NOT NULL REFERENCES diff_runs(id),
    source_id          TEXT NOT NULL REFERENCES sources(id),
    ente_id            TEXT REFERENCES entes(id),
    external_id        TEXT NOT NULL,
    change_type        TEXT NOT NULL CHECK (change_type IN ('added','removed','modified')),
    content_hash_a     BYTEA,
    content_hash_b     BYTEA,
    detected_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    severity           TEXT NOT NULL DEFAULT 'info' CHECK (severity IN ('info','warn','alert')),
    published_rss      BOOLEAN NOT NULL DEFAULT FALSE,
    published_telegram BOOLEAN NOT NULL DEFAULT FALSE,
    published_mastodon BOOLEAN NOT NULL DEFAULT FALSE,
    published_webhook  BOOLEAN NOT NULL DEFAULT FALSE,
    published_email    BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE INDEX ON change_events (source_id, detected_at DESC);
CREATE INDEX ON change_events (ente_id, detected_at DESC) WHERE ente_id IS NOT NULL;
CREATE INDEX ON change_events (external_id);
CREATE INDEX ON change_events (severity, detected_at DESC) WHERE severity != 'info';
CREATE INDEX ON change_events (detected_at) WHERE published_rss = FALSE;
```

`ente_id` nullable para permitir O1 rodar antes de O2 estar completo. Backfill job popula retroativamente quando ente é mapeado.

Append-only, exceto para os campos `published_*` e `ente_id` (estes podem ser atualizados, o resto não).

### Tabelas `lai_checks` e `lai_scores` (O3)

```sql
CREATE TABLE lai_checks (
    id              BIGSERIAL PRIMARY KEY,
    ente_id         TEXT NOT NULL REFERENCES entes(id),
    checked_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    target_url      TEXT NOT NULL,
    http_status     INT,
    response_ms     INT,
    ssl_valid       BOOLEAN,
    ssl_expires_at  TIMESTAMPTZ,
    portal_loads    BOOLEAN,
    html_size_bytes INT,
    terms_found     JSONB NOT NULL DEFAULT '{}'::jsonb,
    required_links  JSONB NOT NULL DEFAULT '{}'::jsonb,
    html_archive_key TEXT,
    errors          TEXT[],
    tier_at_check   INT NOT NULL
);
CREATE INDEX ON lai_checks (ente_id, checked_at DESC);
CREATE INDEX ON lai_checks (checked_at DESC);

CREATE TABLE lai_scores (
    ente_id         TEXT PRIMARY KEY REFERENCES entes(id),
    score           NUMERIC(5,2) NOT NULL DEFAULT 0,
    last_check_id   BIGINT REFERENCES lai_checks(id),
    last_calculated TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    components      JSONB NOT NULL DEFAULT '{}'::jsonb
);
```

### Tabelas `gap_catalog` e `gap_evidence` (O6)

```sql
CREATE TABLE gap_catalog (
    id              BIGSERIAL PRIMARY KEY,
    ente_id         TEXT REFERENCES entes(id),
    source_id       TEXT REFERENCES sources(id),
    category        TEXT NOT NULL,
    title           TEXT NOT NULL,
    description     TEXT NOT NULL,
    severity        TEXT NOT NULL CHECK (severity IN ('info','warn','alert')),
    legal_reference TEXT,
    status          TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','acknowledged','resolved','wont_fix')),
    first_seen_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at     TIMESTAMPTZ,
    metadata        JSONB NOT NULL DEFAULT '{}'::jsonb
);
CREATE INDEX ON gap_catalog (ente_id, status);
CREATE INDEX ON gap_catalog (category, status);
CREATE INDEX ON gap_catalog (severity, status);

CREATE TABLE gap_evidence (
    id              BIGSERIAL PRIMARY KEY,
    gap_id          BIGINT NOT NULL REFERENCES gap_catalog(id),
    kind            TEXT NOT NULL CHECK (kind IN ('change_event','lai_check','snapshot','url','note')),
    change_event_id BIGINT REFERENCES change_events(id),
    lai_check_id    BIGINT REFERENCES lai_checks(id),
    snapshot_id     BIGINT REFERENCES snapshots(id),
    url             TEXT,
    note            TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

**Taxonomia fechada de categorias**: `lai.portal_missing`, `lai.portal_broken`, `lai.ssl_invalid`, `lai.mandatory_section_missing`, `lai.esic_broken`, `data.silent_removal`, `data.silent_edit`, `data.retroactive_amendment`, `data.delayed_publication`, `api.rate_limited`, `api.broken`.

### Tabelas `subscriptions_email` e `subscriptions_webhook` (O4)

```sql
CREATE TABLE subscriptions_email (
    id              BIGSERIAL PRIMARY KEY,
    email           TEXT NOT NULL UNIQUE,
    confirmed_at    TIMESTAMPTZ,
    confirm_token   TEXT,
    unsubscribe_token TEXT NOT NULL,
    filter_sources  TEXT[] DEFAULT '{}',
    filter_severity TEXT DEFAULT 'warn',
    cadence         TEXT NOT NULL DEFAULT 'daily' CHECK (cadence IN ('realtime','daily','weekly')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_sent_at    TIMESTAMPTZ
);

CREATE TABLE subscriptions_webhook (
    id              BIGSERIAL PRIMARY KEY,
    url             TEXT NOT NULL,
    secret          TEXT NOT NULL,
    filter_sources  TEXT[] DEFAULT '{}',
    filter_severity TEXT DEFAULT 'warn',
    active          BOOLEAN NOT NULL DEFAULT TRUE,
    owner_contact   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_success_at TIMESTAMPTZ,
    last_failure_at TIMESTAMPTZ,
    failure_count   INT NOT NULL DEFAULT 0
);
```

LGPD compliance:
- Email como único dado pessoal
- Double opt-in obrigatório (confirm_token, expiry 24h)
- Unsubscribe one-click via `unsubscribe_token`
- Retention: emails desativados deletados após 30 dias

## 6. Novos endpoints da API

Todos JSON, stateless, rate-limited via reverse proxy, padrão REST existente.

```
# Drift (O1)
GET  /api/diff-runs
GET  /api/diff-runs/{id}
GET  /api/change-events
GET  /api/change-events/{id}

# Entes (O2)
GET  /api/entes
GET  /api/entes/{id}
GET  /api/entes/{id}/change-events
GET  /api/entes/{id}/lai-checks

# LAI (O3)
GET  /api/lai/scores
GET  /api/lai/scores?uf=sp
GET  /api/lai/checks/{id}

# Feeds (O4)
GET  /api/feeds/all.atom
GET  /api/feeds/all.json
GET  /api/feeds/sources/{id}.atom
GET  /api/feeds/sources/{id}.json
GET  /api/feeds/entes/{code}.atom
GET  /api/feeds/entes/{code}.json

# Subscriptions (O4)
POST /api/subscriptions/email
GET  /api/subscriptions/confirm?t={token}
GET  /api/subscriptions/unsubscribe?t={token}
POST /api/subscriptions/webhook
DELETE /api/subscriptions/webhook/{id}

# Gaps (O6)
GET  /api/gaps
GET  /api/gaps/{id}
POST /api/gaps            # public create, moderated
PATCH /api/gaps/{id}      # status update
GET  /api/gaps/{id}/evidence

# Exports (bucket)
GET  /api/exports/ceis/{year}/{month}.parquet
GET  /api/exports/cnep/{year}/{month}.parquet
GET  /api/exports/pncp/{year}/{month}.parquet
```

## 7. UI — novas rotas

Lazy-loaded via `React.lazy()` + `Suspense` para code splitting por rota.

```
/observatorio                       dashboard com mapa, stats, timeline
/entes                              índice paginado com busca e filtros
/entes/{code}                       perfil do ente
/entes/{code}/lai                   histórico LAI do ente
/entes/{code}/gaps                  gaps do ente
/lai                                ranking LAI global
/gaps                               catálogo público
/gaps/{id}                          detalhe de gap
/feeds                              página "como assinar"
/feeds/confirm/{token}              confirmação double opt-in
/feeds/unsubscribe/{token}          unsubscribe one-click
/changes                            timeline visual de change_events
```

Componentes reutilizáveis novos: `EnteBadge`, `SeverityBadge`, `ChangeTypeBadge`, `LaiScoreBar`, `GapCategoryBadge`, `ChangeEventList` (virtualizado), `Sparkline`, `MapHeatmap` (upgrade do `BrazilMap`).

Landing (`/`) ganha seção "Observatório" com 4 stat cards + timeline compacta + link pro dashboard. `/recent` é refatorado para ler `change_events` em vez de calcular diffs no cliente.

## 8. Fluxo de dados end-to-end

```
collector (cron 04:00) → snapshot novo
anchor (loop 6h)       → merkle root + OTS
driftd (cron 04:30)    → diff_runs + change_events (severity=info)
severity classifier    → re-avalia eventos recentes, upgrade warn/alert
notifier (loop 30s)    → RSS/Telegram/Mastodon/webhook marca published_*
emailer (cron 08:00)   → daily digest para subscribers daily
archive (loop 24h)     → events > 90d → bucket, canonical_json=NULL
api                    → serve tudo via JSON + SPA
```

Cada worker é reinicializável sem corromper estado. Nenhum depende do outro estar online.

## 9. Testes

- Unit tests: `drift`, `severity`, `canonjson`, `merkle`, `feed` generators, `lai` parser
- Integration tests com Postgres real (Docker Compose test override)
- Smoke test end-to-end: collector → anchor → driftd → notifier → feed API → valida payload
- Mocks dos clients externos (Telegram, Mastodon, Resend, S3) em testes
- CI runs `go test -race ./...` em todo push (já configurado)

## 10. Segurança

- Webhook SSRF guard: rejeita `localhost`, `127.*`, `10.*`, `172.16-31.*`, `192.168.*`, `169.254.*`, `::1`, cloud metadata (169.254.169.254)
- Webhook HMAC signing: `X-Fepublica-Signature: sha256=<hex>`
- Email double opt-in com expiry de 24h no confirm token
- Unsubscribe one-click sem login
- Secrets em env vars, nunca commitados
- Rate limit nos endpoints de subscriptions via Traefik middleware
- robots.txt respeitado no LAI crawler
- User-Agent identificável em toda req externa

## 11. Operações

- Métricas Prometheus por worker em `/metrics`
- Backup diário Postgres → bucket via `scripts/backup.sh` atualizado
- Retention: `change_events` 1 ano; `events.canonical_json` 90 dias; `lai_checks` 6 meses; resto cold
- Healthcheck endpoint por worker
- Log JSON estruturado (zerolog)
- Alerting mínimo: cron externo chama `/api/health`, avisa em falha via Telegram/email
- Disk monitoring: cron checa `df /`, alerta em >90%

## 12. Roadmap de milestones

| Milestone | Release | Sub-projeto | Dependências |
|---|---|---|---|
| M1 | v0.2.0 | O1 Drift detector | nenhuma |
| M2 | v0.3.0 | O4 Feeds básicos (RSS+JSON+Telegram+Mastodon) | O1 |
| M3 | v0.4.0 | O2 Entes + bucket archive + backup off-site | O1 |
| M4 | v0.5.0 | O5 Dashboards geo + landing integration | O1, O2 |
| M5 | v0.6.0 | O3 LAI crawler (tiered rollout) | O2 |
| M6 | v0.7.0 | O4 Feeds completos (webhook + email) | M2 |
| M7 | v0.8.0 | O6 Gaps catalog | O1, O2, O3 |
| M8 | v1.0.0 | Polish + portfolio (English docs, screenshots, case study) | todos |

Cada release é independentemente útil. Git tag dispara `release.yml`, publica binários + imagens ghcr.io, deploy no VPS via `git pull`.

## 13. Portfólio

Cada milestone gera conteúdo estruturado:

- Post LinkedIn (PT) + post dev.to (EN) com narrativa do milestone
- Métricas demonstráveis (mudanças detectadas, gaps abertos, inscritos, etc)
- Screenshots de cada nova página UI (captura via Playwright headless)
- Entradas no CHANGELOG

Artefatos finais em v1.0.0:
- Repositório público completo no GitHub
- Instância pública ativa em `fepublica.gmowses.cloud`
- ~16 posts publicados (8 LinkedIn, 8 dev.to)
- `docs/CASE-STUDY.md` (PT) + `docs/CASE-STUDY.en.md` (EN)
- `docs/screenshots/` com captures por página
- README atualizado posicionando o Observatório como camada principal

## 14. Fora de escopo (permanente)

- Scraping ativo de e-SIC
- Análise de sentimento / NLP
- ML para detecção de anomalias (v2.0+)
- Integração com ferramentas de petição/ação cidadã
- Multi-tenant / SaaS
- Tokens, NFTs, DeFi de qualquer forma

## 15. Fora de escopo por enquanto

- Distrito e bairro (granularidade fica no município)
- Cobertura do Judiciário (DataJud é sensível)
- Cobertura do Legislativo (Câmara/Senado APIs abertas → v0.4+ como source regular)

## 16. Riscos e mitigações

| Risco | Mitigação |
|---|---|
| LAI crawler bloqueado por ente | Rate limit agressivo, robots.txt, UA identificável, circuit breaker por ente |
| Volume de `change_events` em PNCP | Cold archive após 90 dias, partitioning futuro |
| Bucket IDrive E2 indisponível | Fallback para Postgres local, retry async |
| VPS disk pressure | Archive worker + retention + monitoring |
| LGPD em subscriptions_email | Double opt-in, one-click unsub, retention policy, documentação clara |
| Webhook subscribers maliciosos (SSRF) | Validação no cadastro, disable após 10 falhas |
| Scraping detectado como abuso | User-Agent identificável, página `/about` explicando quem somos, contato público |

## 17. Questões abertas (resolver durante implementação)

- Email provider: Resend como default, SMTP configurável, validar custo em produção
- Taxonomia de gaps: começar com a lista fechada, revisar após M7 se houver lacunas
- Webhook retry: começar com fire-and-forget, migrar para queue se volume pedir
- Partitioning de `change_events`: decidir baseado em volume real após 6 meses
- i18n: PT-BR only por enquanto; decidir EN strings após M8

---

**Appendix A — Decisões rejeitadas**

- Kubernetes: descartado para manter simplicidade do Docker Compose + VPS única
- Custom blockchain: Bitcoin via OpenTimestamps é suficiente
- Mobile app: web responsive cobre o caso de uso
- GraphQL: REST cobre, menos complexidade
- Monorepo com turborepo/pnpm: web/ e Go coexistem com `make web` como ponte
- Next.js: Vite SPA é mais leve e estático
- Mastodon privado: usa instância pública padrão

**Appendix B — Referências**

- fepublica design doc: `docs/DESIGN.md`
- fepublica roadmap: `docs/ROADMAP.md`
- Lei 12.527/2011 (LAI): https://www.planalto.gov.br/ccivil_03/_ato2011-2014/2011/lei/l12527.htm
- Lei 14.133/2021 (Nova Lei de Licitações): https://www.planalto.gov.br/ccivil_03/_ato2019-2022/2021/lei/l14133.htm
- OpenTimestamps: https://opentimestamps.org
- IBGE Localidades API: https://servicodados.ibge.gov.br/api/docs/localidades
- Resend: https://resend.com
- IDrive E2: https://www.idrive.com/e2
