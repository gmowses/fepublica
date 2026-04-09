# Roadmap

Estado real por componente. Atualizado à medida que o código evolui.

Legenda: [x] done · [~] in progress · [ ] not started

## v0.1 — MVP

### Infraestrutura básica
- [x] Estrutura do repositório
- [x] Design doc (`docs/DESIGN.md`)
- [x] README público
- [x] CONTRIBUTING
- [x] LICENSE (AGPL-3.0)
- [x] Docker Compose
- [x] Dockerfile multi-stage
- [x] Makefile
- [x] `.env.example`
- [x] `.gitignore`

### Core
- [ ] Pacote `internal/config` (carrega env, validação)
- [ ] Pacote `internal/logging` (setup zerolog)
- [ ] Pacote `internal/store` (conexão pgx, repositories)
- [ ] Migrations iniciais (`db/migrations/001_initial.sql`)
- [ ] Trigger anti-UPDATE/DELETE em `events`
- [ ] Seed inicial de `sources` (CEIS, CNEP)

### Merkle
- [ ] Pacote `internal/merkle` — build tree, gerar prova de inclusão, verificar prova
- [ ] Testes unitários cobrindo: árvore vazia, 1 folha, 2 folhas, ímpar, grande, prova inválida

### Coletor
- [ ] Pacote `internal/transparencia/client` — HTTP client com rate limit e backoff
- [ ] Pacote `internal/transparencia/ceis` — fetch + parse CEIS
- [ ] Pacote `internal/transparencia/cnep` — fetch + parse CNEP
- [ ] Pacote `internal/collector` — orquestração de coleta, criação de snapshot + events
- [ ] Binário `cmd/collector` — CLI com `run --source <nome>` e scheduler via cron
- [ ] Testes de integração com servidor fake

### Canonical JSON
- [ ] Pacote `internal/canonjson` — serialização determinística RFC 8785-ish
- [ ] Testes contra vetores de teste conhecidos

### Anchor
- [ ] Pacote `internal/ots` — cliente HTTP para calendar servers OTS
- [ ] Pacote `internal/anchor` — seleciona snapshots não ancorados, constrói Merkle, submete, guarda receipt
- [ ] Binário `cmd/anchor` — roda em loop com `ANCHOR_BATCH_INTERVAL`
- [ ] Rotina de upgrade de receipts (uma vez por dia)

### API
- [ ] Pacote `internal/api` — handlers HTTP stdlib
- [ ] Endpoint `GET /health`
- [ ] Endpoint `GET /sources`
- [ ] Endpoint `GET /snapshots`
- [ ] Endpoint `GET /snapshots/{id}`
- [ ] Endpoint `GET /events/{snapshot_id}/{external_id}/proof`
- [ ] Endpoint `GET /anchors/{snapshot_id}`
- [ ] Endpoint `GET /anchors/{snapshot_id}/{calendar_id}/receipt.ots`
- [ ] Binário `cmd/api`
- [ ] Labels Traefik no compose de produção

### Verify CLI
- [ ] Binário `cmd/verify` usando cobra
- [ ] Subcomando `verify proof.json`
- [ ] Re-hash do canonical JSON
- [ ] Verificação de prova Merkle
- [ ] Verificação de receipt OTS (via shell-out para `ots verify` ou lib equivalente)
- [ ] Saída legível + exit codes corretos

### Docs
- [x] DESIGN.md
- [x] README.md
- [x] CONTRIBUTING.md
- [x] ROADMAP.md (este arquivo)
- [ ] `docs/sources/ceis.md`
- [ ] `docs/sources/cnep.md`
- [ ] `docs/api.md` (referência dos endpoints)
- [ ] `docs/verification.md` (como verificar uma prova)

### Deploy
- [ ] `deploy/compose/docker-compose.prod.yml` com labels Traefik
- [ ] Script `scripts/deploy.sh` para o VPS
- [ ] GitHub Actions workflow de build + push da imagem
- [ ] GitHub Actions workflow de test + lint em PR
- [ ] DNS de `fepublica.gmowses.cloud`

## v0.2

- [ ] Coletor PNCP
- [ ] Endpoint de diff histórico entre snapshots
- [ ] UI web mínima (search + verify) — Next.js ou HTMX puro
- [ ] Prometheus metrics exporter
- [ ] Dashboard Grafana inicial
- [ ] Feed RSS de mudanças anômalas detectadas

## v0.3

- [ ] Coletor DOU
- [ ] Helm chart
- [ ] Arquitetura de plugin (fonte via YAML + Go package)
- [ ] Observabilidade completa

## v1.0

- [ ] Calendar server OpenTimestamps brasileiro
- [ ] Webhook / API de inscrição em mudanças
- [ ] Documentação completa em PT e EN
- [ ] Caso de uso jornalístico publicado com parceiro (Abraji, Agência Pública, etc)
- [ ] Anúncio público formal
