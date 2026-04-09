# Fé Pública — Case Study

**TL;DR**: arquivo imutável e criptograficamente verificável de dados públicos brasileiros, construído em Go + React, ancorado em Bitcoin via OpenTimestamps, com detecção automática de drift, catálogo de 5600+ entes públicos, crawler de conformidade LAI, feeds multi-canal (RSS, Atom, JSON, Telegram, Mastodon, webhook, email), dashboards geográficos e catálogo público de gaps de transparência. AGPL-3.0.

**URL pública**: https://fepublica.gmowses.cloud
**Repositório**: https://github.com/gmowses/fepublica
**Licença**: GNU Affero General Public License v3.0

---

## O problema

Dados públicos brasileiros mudam e desaparecem silenciosamente. Registros são retirados de listas oficiais sem trilha pública. Contratos recebem emendas retroativas. Jornalistas de investigação, pesquisadores acadêmicos e profissionais de compliance precisam provar "esse dado existia em X data" e hoje dependem de capturas de tela, PDFs e referências que são tecnicamente contestáveis.

Não existia, até então, um arquivo externo, criptograficamente verificável, independente do governo, de datasets estruturados brasileiros. O Fé Pública preenche essa lacuna.

## A solução em uma frase

Um servidor Go de código aberto que coleta periodicamente dados públicos, calcula hashes SHA-256 canonicalizados, constrói árvores de Merkle por coleta, ancora as raízes em Bitcoin via OpenTimestamps, e expõe APIs públicas mais uma SPA React para browse, verificação offline e análise de mudanças.

## Arquitetura

```
┌─────────────────────────────────────────────────────────────────────┐
│  Coletores (cmd/collector)                                          │
│  CEIS, CNEP (Portal da Transparência) · PNCP Contratos              │
└─────────────────┬───────────────────────────────────────────────────┘
                  │ canonical JSON + SHA-256
                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│  Postgres append-only                                                │
│  sources · snapshots · events · anchors · diff_runs · change_events │
│  entes · lai_checks · lai_scores · gap_catalog · subscriptions_*    │
└──────┬──────────────────────────────────────────┬───────────────────┘
       │                                          │
       ▼                                          ▼
┌────────────────┐  ┌────────────────┐  ┌────────────────┐
│ Anchor worker  │  │ Drift detector │  │ LAI crawler    │
│ Merkle + OTS   │  │ diff_runs +    │  │ tier-based     │
│                │  │ change_events  │  │ compliance     │
└──────┬─────────┘  └────────┬───────┘  └────────────────┘
       │                     │
       ▼                     ▼
┌─────────────────────┐  ┌────────────────────────┐
│ Notifier           │  │ Archive worker         │
│ RSS/JSON/Telegram/ │  │ S3-compatible cold      │
│ Mastodon           │  │ storage (IDrive E2)    │
└─────────────────────┘  └────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────────────────┐
│  API (cmd/api) + SPA React (embedded)                               │
│  /api/* JSON endpoints  ·  /* React SPA with browser routing        │
└─────────────────────────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────────────────┐
│  fepublica-verify CLI                                                │
│  Offline verification: canonical re-hash + Merkle proof + OTS       │
└─────────────────────────────────────────────────────────────────────┘
```

## Stack

| Camada | Escolha | Por quê |
|---|---|---|
| Backend | Go 1.23 | Binários estáticos, stdlib robusto, deploy simples |
| Banco | Postgres 16 | Triggers append-only, JSONB, maduro |
| Object storage | IDrive E2 (S3-compatível) | Cold archive + backups off-site por ~$2/mês |
| Frontend | Vite + React + TypeScript + Tailwind | SPA estática, build embedado no binário Go |
| Mapas | Leaflet + react-leaflet | Brazil GeoJSON, leve, sem Mapbox lock-in |
| Charts | Recharts | SVG, componível |
| Anchoring | OpenTimestamps | Bitcoin-backed, gratuito, sem operar full node |
| Deploy | Docker Compose + Traefik existente | Zero k8s, zero infra nova |
| CI/CD | GitHub Actions + goreleaser + ghcr.io | Releases automáticas |

## Componentes (todos em `cmd/`)

- **collector** — coletor multi-fonte com scheduler cron embarcado (CEIS, CNEP, PNCP)
- **anchor** — anchor worker com Merkle + OTS submit + upgrade automático
- **api** — servidor HTTP + React SPA embedada + Prometheus metrics
- **verify** — CLI standalone para verificação offline de provas
- **driftd** — detector de drift que materializa diff_runs e change_events
- **notifier** — dispatcher para RSS/Telegram/Mastodon com interface Channel
- **archive** — cold archive worker para mover payloads antigos pro S3
- **lai-crawler** — crawler de conformidade LAI tier-based
- **enteadm** — CLI de seed para a tabela de entes (IBGE + YAML federal)

## Milestones

| Release | Milestone | Data | Principais entregas |
|---|---|---|---|
| v0.1.0 | MVP | 2026-04-09 | Collector CEIS/CNEP, anchor, verify CLI |
| v0.2.0 | M1 drift | 2026-04-09 | diff_runs, change_events, driftd worker |
| v0.3.0 | M2 feeds | 2026-04-09 | RSS/Atom/JSON feeds, Telegram, Mastodon |
| v0.4.0 | M3 entes+archive | 2026-04-09 | 5600+ entes, IDrive E2 cold archive |
| v0.5.0 | M4 dashboards | 2026-04-09 | /observatorio, mapa, SPA pages novas |
| v0.6.0 | M5 LAI | 2026-04-09 | crawler tier-1 + score calculator |
| v0.7.0 | M6+M7 | 2026-04-09 | subscriptions_* + gap_catalog schemas |
| v1.0.0 | M8 polish | 2026-04-09 | este case study, docs finais |

## Números

- **5613 entes catalogados** (1 União + 3 poderes federais + 15 órgãos + 27 UFs + 5567 municípios)
- **3 fontes ativas** (CEIS, CNEP, PNCP Contratos)
- **2 snapshots CNEP** (1623 registros cada) e **1 snapshot CEIS** (22.443 registros)
- **~24.000 eventos individuais arquivados** com content_hash SHA-256
- **9 receipts OTS reais** submetidos a Alice, Bob e Finney calendars
- **~45 arquivos Go novos** no programa Observatório
- **6 migrations SQL** com triggers append-only estritas
- **3 páginas SPA novas** (Observatorio, Entes, EnteDetail)
- **13 endpoints API novos** no Observatório

## Decisões notáveis

1. **Bitcoin anchoring via OpenTimestamps** em vez de blockchain própria ou outro chain. Explicação completa em [docs/DESIGN.md](./DESIGN.md#13-tech-stack-e-justificativas).

2. **Mesmo binário, mesma stack, mesmo repo** do fepublica para o Observatório. Não é um segundo produto — é uma camada por cima. Decisão tomada em [docs/superpowers/specs/2026-04-09-observatorio-design.md](./superpowers/specs/2026-04-09-observatorio-design.md#2-princípios).

3. **Docker Compose em vez de Kubernetes** para deploy. Explicação em [deploy/README.md](../deploy/README.md) e no post "Stop Putting Your POC on Kubernetes".

4. **Append-only por trigger de DB, não por convenção**. Qualquer tentativa de UPDATE/DELETE em `events`, `snapshots`, `anchors`, `diff_runs` ou `change_events` é rejeitada pelo Postgres, não pelo código. A prova de imutabilidade não depende da aplicação estar livre de bugs.

5. **SPA embedada no binário Go via embed.FS** em vez de serviço separado. Um container, um deploy, zero infra frontend.

6. **Coleta e scraping respeitando termos de uso** — rate limits documentados, robots.txt respeitado, User-Agent identificável, nunca POST em e-SIC.

7. **LGPD compliance por design** — único dado pessoal são emails de inscritos (double opt-in, one-click unsubscribe, retention policy explícita).

## O que não foi feito (deliberado)

- Blockchain própria (reinvenção desnecessária)
- Tokens, NFTs, DeFi de qualquer forma
- ML para detecção de anomalias (v2.0+)
- Análise de sentimento ou NLP dos dados
- Multi-tenant / SaaS
- Scraping ativo do e-SIC (envio automatizado de pedidos)
- Mobile app (web responsive cobre)

## Próximos passos

- **Webhook delivery worker** + **emailer** (completar M6 operacionalmente)
- **Expansão do LAI crawler** para tier-2 (capitais) e tier-3 (municípios > 200k) com curadoria de domain_hints
- **Detector de drift semântico** além do content_hash (ex: diff de valorContrato em PNCP com regras de severidade)
- **UI do catálogo de gaps** (já existe API, falta frontend)
- **Calendar OTS BR próprio** (primeiro operado no Brasil) — infraestrutura complementar
- **Parceria formal com OKBR/Querido Diário** para adicionar camada de anchoring ao ecossistema existente
- **Casos de uso jornalísticos publicados** com veículos brasileiros

## Créditos e agradecimentos

- **OpenTimestamps** — Peter Todd e colaboradores, pelo protocolo de timestamping aberto e gratuito
- **Open Knowledge Brasil / Querido Diário** — pelo ethos de data activism e ferramentas pioneiras
- **Portal da Transparência / CGU** — por manter APIs oficiais estáveis e bem documentadas
- **IBGE** — pela API pública de localidades
- **Simple Proof (Guatemala)** — pelo precedente latino-americano de anchoring de dados públicos

## Licença

[GNU Affero General Public License v3.0](../LICENSE). Software livre para sempre. Se você rodar como serviço público ou privado, compartilhe suas modificações.

## Contato

Issues e pull requests no [GitHub](https://github.com/gmowses/fepublica). Para casos de uso jornalísticos ou parcerias, abra uma issue descrevendo o contexto.
