# Fé Pública — Design

Este documento é a referência técnica do projeto. Ele descreve o problema que o Fé Pública resolve, as decisões de arquitetura, os trade-offs considerados, e o que está explicitamente fora de escopo. É uma fonte viva: atualize quando decisões mudarem.

## Sumário

- [1. Problema](#1-problema)
- [2. Usuários-alvo](#2-usuários-alvo)
- [3. Propriedades do sistema](#3-propriedades-do-sistema)
- [4. Escopo](#4-escopo)
- [5. Arquitetura](#5-arquitetura)
- [6. Modelo de dados](#6-modelo-de-dados)
- [7. Estratégia de coleta](#7-estratégia-de-coleta)
- [8. Merkle + anchoring](#8-merkle--anchoring)
- [9. Verificação](#9-verificação)
- [10. API HTTP](#10-api-http)
- [11. Observabilidade e operação](#11-observabilidade-e-operação)
- [12. Segurança e ameaças](#12-segurança-e-ameaças)
- [13. Tech stack e justificativas](#13-tech-stack-e-justificativas)
- [14. Fora de escopo](#14-fora-de-escopo)
- [15. Questões abertas](#15-questões-abertas)
- [16. Glossário](#16-glossário)

---

## 1. Problema

Dados públicos brasileiros são a base de pesquisa, jornalismo, compliance e accountability civil. No entanto:

1. **Edição silenciosa é possível e acontece.** Registros podem ser alterados ou removidos sem changelog público fácil de consultar. Contratos recebem emendas retroativas. Sanções aparecem e desaparecem.
2. **Não existe terceiro verificável.** Hoje, quem precisa provar "este dado público existia em X" depende da boa-fé do próprio portal oficial. Não há um arquivo externo, criptograficamente verificável, independente do governo.
3. **Provas de existência são frágeis.** Capturas de tela, prints, PDFs exportados — todos são contestáveis em contexto técnico ou jurídico sério.

O Fé Pública preenche essa lacuna sendo um arquivo externo, reprodutível, auditável, e verificável criptograficamente sem depender da sua própria infraestrutura continuar existindo.

## 2. Usuários-alvo

Prioridade decrescente:

1. **Jornalistas investigativos** brasileiros (Agência Pública, Repórter Brasil, Piauí, Folha, Intercept BR, Ponte, Brasil de Fato). Precisam citar snapshots de dados públicos em matérias e defender a citação se o portal original for editado depois.
2. **Pesquisadores acadêmicos** em políticas públicas, ciência política, direito administrativo. Precisam de provas de existência reprodutíveis em artigos.
3. **Compliance officers** em empresas que contratam com o governo. Precisam de trilha de consulta ao CEIS/CNEP no momento da contratação.
4. **Ativistas de dados abertos** (OKBR, Transparência Brasil, Abraji). Precisam de ferramenta auxiliar que complementa os seus próprios coletores.
5. **Desenvolvedores de ferramentas de accountability**. Precisam de uma API de anchoring que possam chamar do próprio produto.

Usuários **não**-alvo no MVP: grande público, end users sem contexto técnico, órgãos de governo (precisariam de features específicas e governança diferente).

## 3. Propriedades do sistema

As propriedades abaixo são objetivos de projeto. Todas as decisões subsequentes derivam delas.

### P1. Verificação independente
Dada uma prova emitida pelo sistema, um terceiro deve poder verificá-la sem depender do servidor Fé Pública estar no ar. A verificação deve usar somente: (a) a prova, (b) uma biblioteca OpenTimestamps padrão, (c) acesso a Bitcoin (via full node, SPV, ou bloco específico).

### P2. Imutabilidade auditada
Nenhum dado coletado é jamais alterado. O banco usa append-only com trigger anti-UPDATE/DELETE. Qualquer alteração no dado-fonte gera um novo snapshot, nunca sobrescreve o anterior.

### P3. Minimização de confiança no operador
O operador do Fé Pública (nós, você se auto-hospedar) não deve ser capaz de forjar provas sem deixar rastro detectável. O anchoring em Bitcoin força isso: se tentarmos manipular, a prova não bate.

### P4. Transparência operacional
A política de coleta (cadência, endpoints, parsers) é versionada em git. Qualquer mudança é pública. Qualquer um pode rodar instância paralela e comparar.

### P5. Baixa manutenção
Uma pessoa deve conseguir operar a instância oficial por anos com trabalho mensal mínimo. Isso significa: sem stacks de observabilidade pesadas, sem clusters, sem múltiplas linguagens, sem dependências frágeis de terceiros.

### P6. Respeito às APIs de origem
Todas as interações com portais oficiais respeitam rate limits documentados, termos de uso, e base legal. Nunca fazer scraping quando a API oficial existe. Sempre identificar-se no User-Agent.

## 4. Escopo

### Em escopo (v0.1 / MVP)

- Coletor CEIS (via Portal da Transparência API).
- Coletor CNEP (via Portal da Transparência API).
- Persistência append-only em Postgres.
- Construção de Merkle tree por snapshot.
- Anchoring em Bitcoin via OpenTimestamps (calendars públicos).
- API HTTP: health, listar snapshots, baixar prova, baixar receipt OTS.
- CLI standalone de verificação offline.
- Deploy via Docker Compose.
- Documentação inicial.

### Em escopo (v0.2)

- Coletor PNCP (API pública sem auth).
- Diff histórico entre snapshots (qual registro mudou, quando).
- UI mínima web para busca e verificação.
- Observabilidade: métricas Prometheus + dashboard Grafana mínimo.
- Alerta no feed RSS quando detectamos mudança anômala.

### Em escopo (v0.3+)

- Coletor DOU (Diário Oficial da União).
- Helm chart.
- Arquitetura de plugin: nova fonte = um YAML + um pacote Go.
- Operator Kubernetes (opcional).

### Em escopo (v1.0+)

- Calendar server OpenTimestamps brasileiro (primeiro em operação no Brasil).
- Webhook / feed de mudanças estruturado para integração com ferramentas de parceiros.

### Fora de escopo (permanente)

- Interface para fact-checking (não somos árbitros de verdade; somos arquivo).
- Hospedagem de conteúdo privado ou com dados pessoais (não é nossa missão e invoca LGPD).
- Blockchain própria (reinvenção desnecessária; OTS/Bitcoin resolvem).
- Tokens, NFTs, DeFi, smart contracts em geral.
- Substituição dos portais oficiais (somos camada de verificação, não de distribuição primária).

## 5. Arquitetura

```
┌─────────────────┐
│ Portais oficiais│
│  (HTTPS APIs)   │
└────────┬────────┘
         │ GET / schedule
         ▼
┌─────────────────┐        ┌──────────────┐
│    collector    │───────►│              │
│   (per source)  │        │   postgres   │
└─────────────────┘        │              │
                           │  events      │
┌─────────────────┐        │  snapshots   │
│   anchor worker │───────►│  merkle_roots│
│   (scheduled)   │        │  anchors     │
└────────┬────────┘        └──────┬───────┘
         │                        │
         │                        │
         ▼                        ▼
┌─────────────────┐        ┌──────────────┐
│  OTS calendars  │        │     api      │
│  (public HTTPS) │        │   (HTTP)     │
└─────────────────┘        └──────┬───────┘
                                  │
                                  ▼
                           ┌──────────────┐
                           │  verify CLI  │
                           │ (standalone) │
                           └──────────────┘
```

Componentes são deliberadamente simples. Cada um roda como processo Go separado, compartilhando Postgres como substrato de estado. Nenhum RPC interno entre eles — comunicação é sempre via banco. Isso permite reiniciar, atualizar ou reescrever qualquer componente independentemente.

### 5.1 collector

Um binário, múltiplas fontes. Entrypoint aceita `--source ceis|cnep|...` e uma fonte é um pacote em `internal/transparencia/<nome>` implementando a interface:

```go
type Source interface {
    Name() string
    Fetch(ctx context.Context, since time.Time) ([]RawRecord, error)
    Parse(raw RawRecord) (Record, error)
}
```

Cadência: cron via `github.com/robfig/cron/v3` embutido no processo (não depende de cron do host). Default: CEIS 04:00 BRT diário, CNEP 04:15 BRT diário — janela de 300 req/min do Portal da Transparência.

Persistência: para cada execução, cria um `snapshot` em `snapshots`, e insere todos os registros como `events` com FK para o snapshot. Nada é atualizado; se o dado mudou, ele aparece como um novo evento em um snapshot posterior.

### 5.2 anchor worker

Processo separado. A cada `ANCHOR_BATCH_INTERVAL` (default 6h), procura snapshots sem âncora:

1. Para cada snapshot não ancorado, busca todos os eventos, serializa cada um de forma determinística, hasheia com SHA-256, constrói uma árvore de Merkle.
2. Submete a raiz da árvore para cada calendar OTS configurado (via HTTP POST /digest).
3. Armazena o `receipt` retornado como blob opaco em `anchors`.
4. Posteriormente, uma rotina de "upgrade" periódica (uma vez por dia) reexecuta `ots upgrade` nos receipts pendentes para anexar as provas de inclusão na blockchain Bitcoin à medida que são confirmadas.

Racional para separar collector e anchor: (a) falha em um não bloqueia o outro; (b) anchor pode ser reiniciado/reimplementado sem afetar coleta; (c) permite backfill seletivo (ancorar snapshots antigos que ainda não foram).

### 5.3 api

Servidor HTTP simples. Endpoints listados em [10. API HTTP](#10-api-http). Stateless — lê tudo do Postgres. Projetado para rodar atrás de um reverse proxy existente (Traefik, nginx) via labels do compose.

### 5.4 verify CLI

Binário standalone que, dada uma prova JSON exportada pela API, valida:

1. Se a prova de inclusão Merkle é válida para a raiz declarada.
2. Se o receipt OTS anexado é válido (usa biblioteca OTS padrão ou shell-out para `ots verify`).
3. Se a raiz ancorada bate com a raiz da Merkle tree.

Não requer conexão com o servidor Fé Pública. Só requer: (a) a prova, (b) acesso a Bitcoin (via SPV, full node, ou snapshot de bloco). Essa é a propriedade P1.

## 6. Modelo de dados

Tabelas principais (Postgres 16):

### sources

Lista de fontes configuradas. Quase estático. Uma linha por fonte (ceis, cnep, pncp, etc).

```sql
CREATE TABLE sources (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    base_url     TEXT NOT NULL,
    description  TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### snapshots

Cada execução de um coletor cria um snapshot.

```sql
CREATE TABLE snapshots (
    id               BIGSERIAL PRIMARY KEY,
    source_id        TEXT NOT NULL REFERENCES sources(id),
    collected_at     TIMESTAMPTZ NOT NULL,
    api_version      TEXT,
    record_count     INT NOT NULL,
    bytes_size       BIGINT NOT NULL,
    merkle_root      BYTEA,  -- NULL até o anchor worker rodar
    merkle_computed_at TIMESTAMPTZ,
    collector_version TEXT NOT NULL,
    notes            TEXT,
    UNIQUE (source_id, collected_at)
);
CREATE INDEX ON snapshots (source_id, collected_at DESC);
```

### events

Cada registro individual coletado. Append-only. `content_hash` é SHA-256 da forma canônica JSON do registro.

```sql
CREATE TABLE events (
    id            BIGSERIAL PRIMARY KEY,
    snapshot_id   BIGINT NOT NULL REFERENCES snapshots(id),
    source_id     TEXT NOT NULL REFERENCES sources(id),
    external_id   TEXT NOT NULL,  -- o "id" que a API oficial retorna
    content_hash  BYTEA NOT NULL, -- SHA-256 do JSON canônico
    canonical_json JSONB NOT NULL,
    collected_at  TIMESTAMPTZ NOT NULL,
    UNIQUE (snapshot_id, external_id)
);
CREATE INDEX ON events (source_id, external_id);
CREATE INDEX ON events (content_hash);
```

Trigger impedindo UPDATE e DELETE em events:

```sql
CREATE OR REPLACE FUNCTION events_immutable()
RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'events are append-only';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER events_no_update BEFORE UPDATE ON events
    FOR EACH ROW EXECUTE FUNCTION events_immutable();

CREATE TRIGGER events_no_delete BEFORE DELETE ON events
    FOR EACH ROW EXECUTE FUNCTION events_immutable();
```

### anchors

Resultado do anchoring. Um snapshot pode ter múltiplas âncoras (uma por calendar).

```sql
CREATE TABLE anchors (
    id           BIGSERIAL PRIMARY KEY,
    snapshot_id  BIGINT NOT NULL REFERENCES snapshots(id),
    calendar_url TEXT NOT NULL,
    submitted_at TIMESTAMPTZ NOT NULL,
    receipt      BYTEA NOT NULL,  -- bytes do receipt OTS
    upgraded     BOOLEAN NOT NULL DEFAULT FALSE,
    upgraded_at  TIMESTAMPTZ,
    block_height INT,              -- bloco Bitcoin onde a prova foi anexada, após upgrade
    UNIQUE (snapshot_id, calendar_url)
);
```

Mesma trigger de imutabilidade se aplica, exceto que `upgraded`, `upgraded_at`, `block_height` e `receipt` podem ser atualizados somente pela rotina de upgrade, e apenas na direção correta (nunca voltar a falso).

## 7. Estratégia de coleta

### Portal da Transparência — CEIS e CNEP

- **Auth**: header `chave-api-dados: <token>`. Token via cadastro Gov.br.
- **Paginação**: `?pagina=N` (1-indexed). Parar em página vazia.
- **Cadência**: diária, janela 04:00-05:59 BRT (dentro do limite 300 req/min).
- **Rate limit**: 1 req/s para estar confortavelmente abaixo do limite diurno de 90/min, e também da janela madrugada de 300/min.
- **Backoff**: exponencial com jitter em 429/5xx. Circuit breaker: se 3 erros consecutivos, suspende coleta e marca snapshot como `notes = 'aborted: api failures'`.
- **Parser**: defensivo. Campos ausentes/null são permitidos. Qualquer erro de parsing é logado mas não interrompe (o evento é marcado como `content_hash = NULL` para investigação manual).

### Forma canônica para hashing

Cada registro é serializado em **JSON canônico**:

- Chaves em ordem alfabética.
- Sem espaços extras.
- Números como strings se vieram como strings.
- Arrays preservam ordem original (diferente de chaves).
- Encoding UTF-8.
- Escape mínimo compatível com RFC 8785.

Isso garante que, dado o mesmo registro, duas execuções em máquinas diferentes geram o mesmo hash.

## 8. Merkle + anchoring

### Merkle tree

- Algoritmo: árvore binária completa, nós internos como `SHA-256(left || right)`, folhas como `content_hash` dos eventos.
- Se número de folhas não é potência de 2, a última folha é duplicada para preencher.
- A ordem das folhas é a ordem de inserção no snapshot (normalmente a ordem retornada pela API, preservada em `events.id`).
- Cada folha recebe um **índice** e uma **prova de inclusão** (vetor de hashes dos irmãos ao longo do caminho da folha à raiz).

Pacote: `internal/merkle`. Dependências: apenas `crypto/sha256` do stdlib. Implementação em ≤200 linhas.

### Serialização da raiz para anchoring

A raiz (32 bytes) é enviada como bytes brutos para cada calendar OTS configurado:

```
POST https://alice.btc.calendar.opentimestamps.org/digest
Content-Type: application/octet-stream
Body: <32 bytes>
```

O calendar retorna um `receipt` em formato binário OpenTimestamps, que é armazenado em `anchors.receipt`.

### Upgrade do receipt

Um receipt recém-emitido só contém a promessa do calendar de incluir a raiz na próxima tx Bitcoin. Depois que essa tx é confirmada e profundeada, o calendar publica a prova de inclusão em Bitcoin. Nosso serviço precisa reexecutar um "upgrade" do receipt para anexar essa prova final.

Rotina: uma vez por dia, o anchor worker executa:

```
POST https://alice.btc.calendar.opentimestamps.org/timestamp/<hash>
```

E, se bem-sucedido, atualiza `anchors.receipt` com a versão upgradeada, marca `upgraded = TRUE`, `upgraded_at = NOW()`, e extrai `block_height` do receipt.

### Alternativas consideradas

- **Anchoring direto em Bitcoin sem OTS**: custo (dezenas de satoshis por âncora, multiplicado por cadência) e complexidade (operar nó). Rejeitado.
- **Anchoring em Ethereum via calldata**: custo de gas imprevisível, posicionamento ruim (cripto-bro). Pode ser opção adicional opt-in no futuro. Rejeitado para MVP.
- **Timestamping interno via assinatura HMAC**: trivial, mas não atende P3 (o operador pode forjar). Rejeitado.
- **Sigstore / Rekor**: alternativa legítima (árvore Merkle append-only como serviço). Mas depende de infra do projeto Sigstore continuar no ar. OTS tem menos dependência operacional. Rejeitado para MVP, possível adição futura opt-in.

## 9. Verificação

A prova exportada pela API é um JSON contendo:

```json
{
  "version": 1,
  "source_id": "ceis",
  "snapshot_id": 42,
  "snapshot_collected_at": "2026-04-09T04:00:15Z",
  "event": {
    "external_id": "12345",
    "content_hash": "sha256:abc...",
    "canonical_json": { ... }
  },
  "merkle": {
    "root": "sha256:def...",
    "index": 1337,
    "siblings": ["sha256:...", "sha256:...", ...]
  },
  "anchors": [
    {
      "calendar_url": "https://alice.btc.calendar.opentimestamps.org",
      "receipt_base64": "...",
      "upgraded": true,
      "block_height": 892345
    }
  ]
}
```

O `verify` CLI faz três validações:

1. **Re-hash**: recalcula `content_hash` a partir de `canonical_json`. Deve bater.
2. **Merkle**: aplica `siblings` em ordem para reconstituir a raiz. Deve bater com `merkle.root`.
3. **OTS**: para cada âncora, decodifica o receipt, verifica a cadeia interna, e (se `upgraded`) verifica que a transação Bitcoin referenciada existe e confirma o hash raiz. A última parte requer acesso a Bitcoin.

Saída do CLI:

```
$ fepublica-verify proof.json
✓ content hash matches
✓ merkle proof valid (root sha256:def...)
✓ OTS receipt valid (via alice.btc.calendar.opentimestamps.org)
✓ anchored in Bitcoin block 892345 at 2026-04-09T10:14:22Z
✓ event "12345" from source "ceis" existed in snapshot 42 (collected 2026-04-09T04:00:15Z)
```

Se qualquer passo falha, o CLI exit code != 0 e imprime o passo que falhou.

## 10. API HTTP

MVP. Endpoints:

- `GET /health` — health check para docker/k8s.
- `GET /sources` — lista de fontes configuradas.
- `GET /snapshots?source=ceis&since=2026-01-01&limit=20` — lista snapshots.
- `GET /snapshots/{id}` — detalhes de um snapshot (counts, merkle_root, status de anchoring).
- `GET /events/{snapshot_id}/{external_id}/proof` — baixa a prova JSON completa (conforme seção 9).
- `GET /anchors/{snapshot_id}` — lista âncoras associadas a um snapshot.
- `GET /anchors/{snapshot_id}/{calendar_id}/receipt.ots` — baixa o receipt OTS binário.

Formato: JSON. Autenticação: nenhuma no MVP (tudo é público). Rate limit: nginx/Traefik na frente.

Idempotente, cacheável, sem cookies, sem state. Qualquer cliente pode consumir.

## 11. Observabilidade e operação

### MVP

- Logs estruturados em JSON via `zerolog`. Campos: `level`, `ts`, `component` (collector/anchor/api), `source_id`, `snapshot_id`, `err`.
- `GET /health` retorna JSON com: última coleta por fonte, último anchor, estado do DB, versão do binário.
- Sem Prometheus, sem Grafana, sem tracing distribuído. Prematuro pro escopo.

### v0.2+

- Exporter Prometheus em `/metrics` com: `collector_records_total{source=...}`, `collector_errors_total{source=...}`, `anchor_batches_total`, `anchor_duration_seconds`, `snapshots_without_anchor`.
- Dashboard Grafana mínimo no repo (`deploy/grafana/dashboard.json`).

## 12. Segurança e ameaças

### Modelo de ameaça

1. **Atacante remoto tenta DoS**: mitigação via rate limit no reverse proxy, sem auth para leitura (logo nada pra proteger de brute force).
2. **Atacante tenta alterar dados**: trigger de imutabilidade no Postgres + Merkle anchoring tornam qualquer alteração detectável post-hoc.
3. **Operador desonesto (nós) tenta forjar provas**: não pode, sem comprometer Bitcoin, pois as provas dependem do anchoring em blockchain pública.
4. **Atacante compromete nosso VPS**: pode interromper o serviço mas não pode alterar provas antigas já ancoradas. Pode ancorar dados falsos daqui pra frente; mitigação é usar a instância pública como uma-de-várias, com reprodução independente.
5. **Calendar OTS desaparece**: mitigação por redundância (sempre enviar a pelo menos 2 calendars diferentes) + possibilidade de operar calendar próprio em v1.0.
6. **Bitcoin desaparece ou sofre reorg profundo**: aceito como risco residual; Bitcoin é o alicerce de confiança.

### Secrets

- `TRANSPARENCIA_API_KEY`: token da API do Portal da Transparência. Sensível (pode ser revogado se vazar). Armazenado em variável de ambiente, nunca commitado.
- Em produção no VPS, carregado via `.env` restrito (chmod 600, owner único).
- Em futuro de produção mais sério: Vault ou similar.

### Dados pessoais (LGPD)

- CEIS e CNEP contêm nomes de empresas (CNPJs), não pessoas físicas. Não há PII no escopo.
- Cuidado futuro com fontes que podem conter CPF (folha de servidores, benefícios sociais): essas fontes ficam fora de escopo permanente do produto.

## 13. Tech stack e justificativas

| Camada | Escolha | Por quê |
|---|---|---|
| Linguagem | Go 1.23 | Binários estáticos, operacional simples, match com perfil do mantenedor, libs de crypto robustas no stdlib. |
| Banco | Postgres 16 | Trigger robusto, JSONB para canonical, maduro, conhecido. |
| Scheduler | `robfig/cron/v3` embarcado | Zero daemon extra, cron syntax familiar. |
| HTTP server | stdlib `net/http` | Não precisa de framework; rotas são poucas. |
| CLI | `spf13/cobra` | Padrão de facto Go, ergonômico. |
| Logging | `rs/zerolog` | JSON estruturado, performance, API limpa. |
| DB driver | `jackc/pgx/v5` | Padrão moderno, bom suporte a JSONB. |
| OTS | HTTP direto para calendars + shell-out para `ots` CLI na verificação | Evita binding Go imaturo. |
| Container | Distroless nonroot | Superfície de ataque mínima. |
| Orquestração | Docker Compose | Baixa complexidade, suficiente para MVP. |
| Edge | Traefik existente no VPS | Reuso de infra conforme padrão do mantenedor. |

### Decisões rejeitadas

- **Rust** em vez de Go: Go vence por ergonomia e por match com o resto do stack do mantenedor. A verificação de performance do Rust não importa para este workload.
- **Node.js**: ecossistema de crypto e serialização canônica menos sólido que Go.
- **Python**: velocidade de desenvolvimento boa, mas empacotar e distribuir o CLI de verificação em Python é pior que em Go (um binário estático vs gerenciar ambiente Python).
- **SQLite em vez de Postgres**: SQLite não tem JSONB robusto e não suporta trigger stateful do jeito que queremos.
- **MongoDB**: não faz sentido para este workload, e padrões de imutabilidade são mais difíceis.
- **Kubernetes**: premature. Docker Compose resolve.

## 14. Fora de escopo

Permanentemente fora do produto (não só do MVP):

- Fact-checking de conteúdo coletado.
- Moderação editorial de dados coletados.
- Coleta de dados que exigem login individual ou de usuário final.
- Hospedagem de dados pessoais ou dados sob LGPD sensível.
- Substituição dos portais oficiais.
- Token/NFT/DeFi de qualquer natureza.
- Smart contract de qualquer natureza.
- Blockchain alternativa própria.
- Integração com carteira cripto de usuário final.

## 15. Questões abertas

Questões que precisarão ser decididas antes de ganhar tração, mas que não bloqueiam o MVP:

- **Governança**: até onde vai "projeto pessoal" e quando institucionalizar? (NEES/UFAL como opção posterior.)
- **Sustentabilidade financeira**: se o custo operacional passar de ~R$ 50/mês, como cobrir? Grant? Doação? Manter custo baixo por design é a primeira resposta.
- **Inclusão de novas fontes sensíveis**: DOU, SALIC, etc. têm implicações políticas. Definir critério público antes de adicionar.
- **Calendar OTS BR**: quando vale rodar um? Começa justificado se chegarmos a >10k anchoring requests/mês ou se houver demanda externa.
- **Política de retenção**: nunca deletar dados parece óbvio, mas custo de storage cresce. Precisa plano de longo prazo.
- **Diferenciação vs. parceria**: manter projeto independente ou buscar vínculo formal com Querido Diário/OKBR? Decidir após primeira reação pública.

## 16. Glossário

- **Append-only**: dados são sempre inseridos, nunca alterados ou deletados. Qualquer "mudança" é um novo registro.
- **Canonical JSON**: representação determinística de um objeto JSON, de modo que qualquer implementação correta gera o mesmo output. Base para hashing.
- **CEIS**: Cadastro de Empresas Inidôneas e Suspensas, mantido pela CGU. Lista empresas impedidas de contratar com o poder público.
- **CNEP**: Cadastro Nacional de Empresas Punidas, mantido pela CGU. Lista empresas penalizadas pela Lei Anticorrupção.
- **Merkle tree**: árvore binária de hashes onde cada nó interno é `hash(left || right)`. Permite prova de inclusão compacta.
- **OpenTimestamps (OTS)**: protocolo aberto para timestamping de documentos via Bitcoin, mantido por Peter Todd e contribuidores.
- **Calendar server (OTS)**: servidor que agrega hashes de múltiplos clientes e os commita periodicamente em Bitcoin, retornando receipts que podem ser upgradeados para provas completas.
- **Receipt (OTS)**: blob binário que permite a verificação offline de um timestamp. Começa como "promessa" e é upgradeado após a tx Bitcoin ser confirmada.
- **PNCP**: Portal Nacional de Contratações Públicas. Agregador federal de todos os contratos públicos brasileiros desde 2023.
- **DOU**: Diário Oficial da União. Publicação oficial dos atos do governo federal.
