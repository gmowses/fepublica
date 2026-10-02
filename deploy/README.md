# Deploy em Docker Swarm

Este diretório contém os artefatos de produção para uma VPS com Docker Swarm e Traefik. O stack não cria domínio, DNS, credenciais ou serviços externos.

[`compose/docker-stack.yml`](./compose/docker-stack.yml) cria Postgres privado, API e workers de processamento. Sem chave do Portal da Transparência, o coletor agenda somente PNCP; CEIS, CNEP e cartões são bloqueados no código.

## Pré-requisitos

- Docker 29+ com Swarm ativo no manager.
- Rede overlay externa `traefik-public` e Traefik com provider Swarm, entrypoints `web` e `websecure`, e resolver `letsencrypt`.
- Um diretório de release local ao manager, que contenha `db/migrations`.
- O domínio planejado é `fepublica.mowses.dev.br`, mas o DNS A ainda precisa apontar para a VPS. O stack exige `FEPUBLICA_DOMAIN` e não possui domínio padrão embutido.

## Preparação do release

No manager, dentro do checkout que será usado como release:

```bash
export FEPUBLICA_RELEASE_DIR="$PWD"
export FEPUBLICA_DOMAIN=fepublica.mowses.dev.br
export POSTGRES_PASSWORD="$(openssl rand -base64 32)"
export FEPUBLICA_IMAGE_PREFIX=fepublica
export FEPUBLICA_IMAGE_VERSION=1.6.0

for target in api anchor collector contratos-indexer ceap-indexer cartoes-indexer driftd forenses-runner; do
  docker build --build-arg TARGET="$target" --build-arg VERSION="$FEPUBLICA_IMAGE_VERSION" \
    -t "$FEPUBLICA_IMAGE_PREFIX/$target:$FEPUBLICA_IMAGE_VERSION" .
done
```

As imagens são locais ao manager; os serviços têm `node.role == manager` como restrição. Para evitar tentativa de resolver imagem em registry inexistente, use `--resolve-image never` no deploy.

Antes de executar qualquer deploy, revise as variáveis acima e confirme que o domínio, a senha e o diretório de release são os desejados.

```bash
docker stack deploy --resolve-image never \
  -c deploy/compose/docker-stack.yml fepublica
```

O bind de migrations é somente leitura e o entrypoint oficial do Postgres só o processa ao inicializar um volume vazio. Ele não reaplica migrations em um volume já existente.

## Verificação

Após deploy autorizado, use o domínio configurado:

```bash
docker service ls
docker service ps fepublica_api
docker service ps fepublica_postgres
curl --fail --silent --show-error "https://$FEPUBLICA_DOMAIN/api/health"
```

O endpoint Prometheus fica em `127.0.0.1:9090` dentro do contêiner da API e não recebe rota Traefik pública.

## Backup

[`../scripts/backup.sh`](../scripts/backup.sh) seleciona o contêiner do serviço Swarm `fepublica_postgres` pelo label exato, grava um dump atomicamente, executa `pg_restore --list` antes de publicá-lo e remove somente dumps próprios com mais de 14 dias. Defina `FEPUBLICA_BACKUP_DIR` se `/var/backups/fepublica` não for o destino desejado.
