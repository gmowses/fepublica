# Operações — produção Docker Compose

Este documento cobre operação do dia a dia da instância oficial do Fé Pública (`fepublica.gmowses.cloud`) rodando em uma VPS compartilhada com outros projetos, usando Docker Compose + Traefik.

Para o setup inicial, veja [`deploy/README.md`](./README.md).

## Stack em produção

- VPS Debian 13, Docker 29, Docker Compose v2
- Traefik existente (contêiner `cloudface_traefik` na rede `cloudface_proxy`)
- Postgres 16 em container dedicado, volume nomeado `fepublica_postgres_data`
- Três contêineres fepublica: `api`, `collector` (serve), `anchor` (loop)
- API bind a `127.0.0.1:8080`, Traefik roteia via rede Docker

## Atualização

### Pulling updates do git

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

### Usando imagens publicadas (ghcr.io)

A partir da tag `v0.1.0` e seguintes, as imagens são publicadas em:

- `ghcr.io/gmowses/fepublica-api:<tag>`
- `ghcr.io/gmowses/fepublica-collector:<tag>`
- `ghcr.io/gmowses/fepublica-anchor:<tag>`

Para trocar o fluxo de build local por pull do registry, edite `deploy/compose/docker-compose.prod.yml` para cada serviço:

```yaml
  api:
    image: ghcr.io/gmowses/fepublica-api:v0.1.0
    build: !reset null
```

E rode:

```bash
docker compose \
  -f docker-compose.yml \
  -f deploy/compose/docker-compose.prod.yml \
  pull
docker compose \
  -f docker-compose.yml \
  -f deploy/compose/docker-compose.prod.yml \
  up -d
```

## Backups

A única peça stateful é o volume `fepublica_postgres_data`. Perder esse volume **não destrói** as provas ancoradas (elas estão em Bitcoin), mas destrói o índice que mapeia `external_id` → prova, forçando recoleta completa.

### Backup diário simples

```bash
# no host:
TS=$(date +%Y%m%d-%H%M)
docker compose -f /opt/fepublica/docker-compose.yml -f /opt/fepublica/deploy/compose/docker-compose.prod.yml \
  exec -T postgres pg_dump -U fepublica -d fepublica -Fc \
  > /var/backups/fepublica/fepublica-${TS}.dump
```

Automatize via cron:

```
0 3 * * * cd /opt/fepublica && ./scripts/backup.sh >> /var/log/fepublica-backup.log 2>&1
```

Retenção sugerida: 7 daily + 4 weekly + 12 monthly. Use `logrotate` ou `restic` para gerenciar.

### Restore

```bash
docker compose -f /opt/fepublica/docker-compose.yml -f /opt/fepublica/deploy/compose/docker-compose.prod.yml \
  exec -T postgres psql -U fepublica -d fepublica -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"

cat /var/backups/fepublica/fepublica-20260409-0300.dump | \
docker compose -f /opt/fepublica/docker-compose.yml -f /opt/fepublica/deploy/compose/docker-compose.prod.yml \
  exec -T postgres pg_restore -U fepublica -d fepublica --clean --if-exists -
```

### Off-site

Encaminhar os dumps para armazenamento externo (S3, Backblaze B2, outro VPS) é fortemente recomendado:

```
*/0 4 * * * restic -r b2:fepublica-backups:/fepublica backup /var/backups/fepublica --password-file ~/.restic-pw
```

## Logs

### Acesso rápido

```bash
cd /opt/fepublica
docker compose -f docker-compose.yml -f deploy/compose/docker-compose.prod.yml logs -f api
docker compose -f docker-compose.yml -f deploy/compose/docker-compose.prod.yml logs -f collector
docker compose -f docker-compose.yml -f deploy/compose/docker-compose.prod.yml logs -f anchor
```

### Log rotation

Os containers logam em JSON para stdout. O daemon do Docker rotaciona por padrão (veja `/etc/docker/daemon.json`). Configuração recomendada:

```json
{
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "50m",
    "max-file": "5"
  }
}
```

Reinicie o daemon Docker após mudança.

## Métricas

O endpoint `/metrics` do serviço `api` expõe Prometheus. Como a API está atrás do Traefik, a forma mais simples de scrapa-la é expor apenas dentro da rede compartilhada `cloudface_proxy` via um Prometheus que também esteja nela:

```yaml
# prometheus.yml snippet
scrape_configs:
  - job_name: fepublica
    static_configs:
      - targets: ['fepublica-api-1:8080']
    metrics_path: /metrics
```

Se você não quer expor `/metrics` publicamente via Traefik, o padrão atual já é seguro: o endpoint só é acessível via `127.0.0.1:8080` no host ou via a rede Docker interna.

## Forçar anchoring imediato

```bash
docker compose -f /opt/fepublica/docker-compose.yml -f /opt/fepublica/deploy/compose/docker-compose.prod.yml \
  run --rm anchor --once
```

Use para: forçar construção da Merkle root de um snapshot recém-coletado, submeter âncoras em um calendar que falhou, rodar a rotina de upgrade manualmente.

## Forçar coleta manual

```bash
docker compose -f /opt/fepublica/docker-compose.yml -f /opt/fepublica/deploy/compose/docker-compose.prod.yml \
  run --rm collector run --source ceis
docker compose -f /opt/fepublica/docker-compose.yml -f /opt/fepublica/deploy/compose/docker-compose.prod.yml \
  run --rm collector run --source cnep
docker compose -f /opt/fepublica/docker-compose.yml -f /opt/fepublica/deploy/compose/docker-compose.prod.yml \
  run --rm collector run --source pncp-contratos
```

Útil para primeira coleta após deploy, recoleta pontual após incidente, teste após upgrade.

## Monitoramento básico (sem Prometheus)

Um cron simples que checa `/health` e alerta via email/slack se falhar:

```bash
#!/usr/bin/env bash
if ! curl -fsS --max-time 10 https://fepublica.gmowses.cloud/health > /dev/null; then
  echo "fepublica health check failed at $(date -u +%FT%TZ)" | \
    mail -s "[fepublica] health check failed" alerts@example.com
fi
```

## Auto-update via Watchtower (opcional)

Se você quer que os containers se atualizem sozinhos quando uma nova tag sair em ghcr.io, adicione o [Watchtower](https://containrrr.dev/watchtower/):

```yaml
# docker-compose.override.yml, só no VPS
services:
  watchtower:
    image: containrrr/watchtower:latest
    restart: unless-stopped
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    command: --cleanup --interval 3600 fepublica-api-1 fepublica-collector-1 fepublica-anchor-1
```

Recomendação: **não** ative em produção se você não tem testes automatizados rigorosos — prefira updates controlados manualmente.

## Disaster recovery checklist

Se a VPS sumir totalmente:

1. Provisionar nova VPS Debian + Docker + Traefik (ou outro reverse proxy).
2. Restaurar o backup mais recente do volume Postgres.
3. `git clone` do repo em `/opt/fepublica` na nova VPS.
4. Criar `.env` com o mesmo `POSTGRES_PASSWORD` do backup e o `TRANSPARENCIA_API_KEY` atual.
5. Subir stack.
6. Atualizar DNS `fepublica.gmowses.cloud` para o novo IP.
7. Aguardar propagação + Let's Encrypt reemissão.

RTO (recovery time objective) esperado: < 1h, dominado por download de imagens e propagação DNS.

RPO (recovery point objective): igual ao intervalo do cron de backup (recomendado: 1 dia).

## Rotacionando credenciais

### Token Portal da Transparência

Se a chave vazar ou for revogada:

1. Gerar nova chave via https://portaldatransparencia.gov.br/api-de-dados/cadastrar-email (mesmo email).
2. Atualizar `.env` na VPS.
3. `docker compose -f docker-compose.yml -f deploy/compose/docker-compose.prod.yml up -d --force-recreate collector anchor api`.

### Senha Postgres

1. Parar a stack.
2. Gerar nova senha.
3. Atualizar `.env`.
4. Editar a senha no volume Postgres via `psql` em um container one-shot, ou destruir o volume e restaurar do backup com a nova senha.
5. Subir a stack.

### Token Cloudflare

Como o DNS foi criado uma vez e é estático, após o setup não precisamos mais do token. Revogue ele em Cloudflare Dashboard > My Profile > API Tokens assim que o DNS estiver funcionando.
