# Fé Pública

> Arquivo imutável e criptograficamente verificável de dados públicos brasileiros.

**Status**: `alpha` — em desenvolvimento ativo.
**Licença**: [AGPL-3.0](./LICENSE)

Fé Pública é um serviço open source que coleta dados de portais públicos brasileiros (Portal da Transparência, PNCP, e outros), organiza cada coleta em uma árvore de Merkle, ancora a raiz dessa árvore em Bitcoin via [OpenTimestamps](https://opentimestamps.org/), e expõe uma API de verificação que permite a qualquer pessoa provar — offline, sem depender da nossa infraestrutura — que um dado público existia em determinada data e não foi alterado desde então.

O nome vem do termo jurídico brasileiro: **fé pública** é a garantia de autenticidade que certos documentos oficiais carregam por força de lei. Este projeto busca reproduzir a mesma garantia para dados abertos, usando criptografia em vez de autoridade.

## Por que isso existe

Dado público brasileiro **muda e desaparece silenciosamente**. Registros do Cadastro de Empresas Inidôneas e Suspensas (CEIS) são retirados da lista sem trilha pública. Contratos no PNCP recebem emendas retroativas sem changelog. Edições do Diário Oficial da União acontecem por ato formal, mas nem sempre são fáceis de reconstruir após o fato.

Hoje, quem precisa provar "esse dado existia nesta data" depende da boa-fé do portal oficial, ou de registros manuais. Este projeto oferece uma alternativa:

- **Snapshots periódicos** de datasets públicos, via APIs oficiais, dentro dos termos de uso.
- **Hashes encadeados** em árvores de Merkle, de modo que uma prova curta basta para demonstrar inclusão de um registro específico.
- **Âncoras em Bitcoin** via OpenTimestamps — um protocolo gratuito e sem operação de nó próprio no MVP.
- **Verificação independente** via CLI que não depende do servidor Fé Pública continuar no ar. Sua prova é sua para sempre.

## Casos de uso

- **Jornalismo investigativo**: provar que uma empresa estava (ou não) numa lista de sanções em data X, mesmo que o portal oficial tenha sido editado depois.
- **Pesquisa acadêmica**: citar snapshots reproduzíveis de datasets públicos em artigos.
- **Compliance e due diligence**: manter trilha verificável de consultas ao CEIS/CNEP no momento da contratação.
- **Accountability civil**: monitorar edições retroativas de contratos públicos no PNCP.
- **Evidência processual**: gerar provas de existência de conteúdo público em data certa.

## Escopo do MVP

| Em escopo | Fora do MVP |
|---|---|
| Coletor CEIS (Portal da Transparência) | UI web completa |
| Coletor CNEP | Multi-tenant |
| Merkle tree + batching | Alertas / notificações |
| Anchor worker via OpenTimestamps | Calendar server OTS próprio |
| API HTTP de verificação | Suporte a múltiplas fontes via plugin |
| API HTTP de diff entre snapshots | PNCP, DOU (v2) |
| CLI standalone de verificação offline | Helm chart (v2) |
| Docker Compose para local + produção | Kubernetes operator (v3) |

## Status atual

Este repositório está em fase inicial. Nem todas as partes descritas aqui estão implementadas. Confira [`docs/ROADMAP.md`](./docs/ROADMAP.md) para o estado real por componente.

## Quickstart (local)

```bash
# 1. Cadastre-se e obtenha um token da API do Portal da Transparência:
#    https://portaldatransparencia.gov.br/api-de-dados/cadastrar-email

# 2. Clone e configure:
git clone https://github.com/gmowses/fepublica
cd fepublica
cp .env.example .env
# edite .env e coloque TRANSPARENCIA_API_KEY=<seu-token>

# 3. Suba o stack:
make up

# 4. Rode a migração:
docker compose --profile migrate run --rm migrate

# 5. Dispare um coletor sob demanda:
docker compose run --rm collector run --source ceis

# 6. Veja o estado da API:
curl http://localhost:8080/health
curl http://localhost:8080/snapshots
```

## Arquitetura resumida

```
┌──────────────┐    ┌───────────┐    ┌──────────┐    ┌───────────┐
│  Portal da   │    │           │    │          │    │           │
│ Transparência├───►│ collector ├───►│ postgres ├───►│  anchor   │
│    (API)     │    │           │    │          │    │  worker   │
└──────────────┘    └───────────┘    └─────┬────┘    └─────┬─────┘
                                           │               │
                                     ┌─────▼─────┐         │
                                     │   api     │         ▼
                                     │  (http)   │    ┌──────────────┐
                                     └─────┬─────┘    │   OTS        │
                                           │          │  calendars   │
                             ┌─────────────┼──────────┤  (Bitcoin)   │
                             │             │          └──────────────┘
                             ▼             ▼
                      ┌───────────┐  ┌──────────┐
                      │  verify   │  │ journal  │
                      │    CLI    │  │ / audit  │
                      └───────────┘  └──────────┘
```

Componentes:

- **collector** — consome API oficial, grava em `events` (append-only) + `snapshots`.
- **merkle** — pacote interno que constrói árvore e gera provas de inclusão.
- **anchor worker** — periodicamente seleciona snapshots não ancorados, constrói Merkle tree, envia hash raiz para calendars OTS, guarda o receipt.
- **api** — endpoints HTTP de consulta, verificação, e diff histórico.
- **verify CLI** — binário standalone que valida uma prova offline sem depender do servidor Fé Pública.

Veja [`docs/DESIGN.md`](./docs/DESIGN.md) para detalhes.

## Garantias que o serviço oferece

1. **Integridade**: dado um snapshot e uma prova, você pode verificar sem nós que o registro existia naquele snapshot.
2. **Tempo**: dado um receipt OTS, você pode verificar (via Bitcoin) que o snapshot foi ancorado antes de um determinado bloco Bitcoin — logo, antes de um momento no tempo.
3. **Independência**: a verificação não requer o servidor Fé Pública estar no ar. Se sumirmos do mapa, sua prova continua válida enquanto Bitcoin existir.
4. **Reprodutibilidade**: a política de coleta é pública e versionada. Qualquer um pode rodar uma instância paralela e comparar.

## Garantias que o serviço **não** oferece

1. **Autenticidade da fonte**: Fé Pública arquiva o que a API oficial retorna. Se o governo publicou um dado errado, Fé Pública registra esse erro. Não somos fact-checker.
2. **Completude**: podemos ter gaps de coleta (falha de API, rate limit, bug). Sempre publicaremos nossa cobertura real.
3. **Privacidade**: todos os dados coletados já são públicos por lei. Este projeto não coleta, armazena ou processa dados pessoais não-públicos.

## Roadmap

- **v0.1 (MVP)**: CEIS + CNEP, anchoring via calendars OTS públicos, CLI de verificação, API básica.
- **v0.2**: PNCP, diff histórico, UI mínima de busca.
- **v0.3**: DOU, Helm chart, observabilidade (Prometheus metrics, Grafana dashboards).
- **v1.0**: Calendar server OTS próprio ("primeiro calendar operado no Brasil"), webhook para jornalistas/pesquisadores, documentação completa.

## Contribuir

Veja [`CONTRIBUTING.md`](./CONTRIBUTING.md). Contribuições são bem-vindas, especialmente:

- Adição de novas fontes (cada fonte é um pacote em `internal/transparencia/`).
- Melhoria do parser defensivo de payload governamental.
- Tradução da documentação para inglês.
- Casos de uso jornalísticos (abra uma issue contando o que você precisa).

## Agradecimentos

- [OpenTimestamps](https://opentimestamps.org/) — Peter Todd e colaboradores, por manterem um protocolo gratuito e independente de ancoragem em Bitcoin.
- [Open Knowledge Brasil](https://ok.org.br/) e o projeto [Querido Diário](https://queridodiario.ok.org.br/), por mostrarem que coletor público aberto funciona.
- [Portal da Transparência / CGU](https://portaldatransparencia.gov.br/) por manter APIs abertas, estáveis e documentadas.
- [Simple Proof](https://simpleproof.com/) pelo precedente latino-americano de anchoring de dado público em Bitcoin.

## Licença

Este projeto é distribuído sob a [GNU Affero General Public License v3.0](./LICENSE).

Escolhemos AGPL em vez de MIT deliberadamente: se alguém oferecer Fé Pública como serviço público ou privado, deve abrir o código das modificações. Infraestrutura de integridade de dados públicos não deveria virar produto fechado.
