# Roadmap

Estado real por componente. Atualizado a cada release.

Legenda: [x] done · [~] in progress · [ ] not started

## v0.1.0 — Fé Pública MVP ✅

- [x] Collector CEIS/CNEP
- [x] Anchor worker com OpenTimestamps
- [x] API HTTP + JSON
- [x] fepublica-verify CLI standalone
- [x] SPA React inicial
- [x] Docker Compose + VPS deploy
- [x] Instância pública em fepublica.gmowses.cloud
- [x] GitHub Actions CI + goreleaser
- [x] Design doc

## v0.2.0 — Observatório M1: drift detector ✅

- [x] Migration 003: diff_runs + change_events
- [x] Triggers append-only
- [x] Store repos (diff_runs, change_events)
- [x] internal/severity com rules configuráveis
- [x] internal/drift detector + Loop
- [x] cmd/driftd worker
- [x] API: /api/diff-runs, /api/change-events
- [x] /recent page refatorada

## v0.3.0 — Observatório M2: feeds básicos ✅

- [x] internal/feed (Atom 1.0 + JSON Feed 1.1)
- [x] internal/notify Channel interface
- [x] RSSChannel (pull-based)
- [x] TelegramChannel (Bot API)
- [x] MastodonChannel (/api/v1/statuses)
- [x] cmd/notifier worker
- [x] API feed endpoints
- [x] Optional env-var-driven enable

## v0.4.0 — Observatório M3: entes + bucket archive ✅

- [x] Migration 004: entes table
- [x] db/seeds/entes-federal.yaml
- [x] IBGE API fetch com suporte a ambas hierarquias
- [x] Hardcoded 27 UFs + DF
- [x] cmd/enteadm CLI de seed
- [x] internal/archive com minio-go
- [x] cmd/archive worker
- [x] S3Config no config
- [x] Bucket IDrive E2 configurado

## v0.5.0 — Observatório M4: dashboards geo ✅

- [x] /observatorio SPA page
- [x] /entes listing page
- [x] /entes/{id} detail page
- [x] Aggregates API (stats, entes-by-uf, changes-by-uf)
- [x] BrazilMap com heatByUF real
- [x] Layout nav expandido

## v0.6.0 — Observatório M5: LAI crawler ✅

- [x] Migration 005: lai_checks + lai_scores
- [x] internal/lai Checker com GET/SSL/terms/links
- [x] Score calculator com breakdown por componente
- [x] cmd/lai-crawler worker com tier selection
- [x] Store ListEntesForCrawl + LAI repos
- [x] API endpoints /api/lai/scores, /api/entes/{id}/lai-checks
- [ ] Tier-2 (capitais) — gated em domain_hint curation
- [ ] Tier-3 e tier-4 — work futuro

## v0.7.0 — Observatório M6+M7: subscriptions + gaps schema ✅

- [x] Migration 006: subscriptions_webhook + subscriptions_email + gap_catalog + gap_evidence
- [x] Store repos para gaps
- [x] API endpoints GET/POST /api/gaps
- [ ] Webhook delivery worker (cmd/notifier extension)
- [ ] Emailer worker com Resend
- [ ] Email double opt-in flow
- [ ] Gap SPA pages

## v1.0.0 — Polish + portfolio ✅

- [x] docs/CASE-STUDY.md
- [x] docs/ROADMAP.md atualizado
- [x] Migrations 003-006 aplicadas em produção
- [x] Todos os milestones M1-M7 deployed
- [ ] Screenshots automatizados (v1.1)
- [ ] English translations (v1.1)
- [ ] Anúncio público formal em LinkedIn e dev.to

## Pós-v1.0 (ideias)

- Webhook delivery worker + email newsletter (completar M6 operacional)
- LAI crawler expansion tier-2 e tier-3
- Detector de drift semântico (valores de contratos, etc)
- Calendar OTS BR próprio
- Parceria com OKBR/Querido Diário
- Casos de uso jornalísticos publicados
- Multi-idioma (EN) na SPA
