# Operações de produção

## Serviços e rede

O stack `fepublica` mantém Postgres apenas na rede overlay interna. A API entra também na rede externa `traefik-public`; nenhuma porta Postgres ou API é publicada pelo Docker. O domínio planejado é `fepublica.mowses.dev.br` e requer DNS A antes de o certificado ser emitido. Traefik atende HTTPS e aplica HSTS, CSP, `nosniff`, limite de taxa e limite de requisições concorrentes.

O listener de métricas da API é `127.0.0.1:9090` no contêiner e não é exposto por Traefik. Faça scraping por uma solução de observabilidade explicitamente autorizada na rede interna.

## Atualização

O stack usa imagens locais versionadas. No manager, com as variáveis de release já definidas, construa a nova versão e execute:

```bash
for target in api anchor collector contratos-indexer ceap-indexer cartoes-indexer driftd forenses-runner; do
  docker build --build-arg TARGET="$target" --build-arg VERSION="$FEPUBLICA_IMAGE_VERSION" \
    -t "$FEPUBLICA_IMAGE_PREFIX/$target:$FEPUBLICA_IMAGE_VERSION" .
done
docker stack deploy --resolve-image never -c deploy/compose/docker-stack.yml fepublica
docker service ps fepublica_api
```

Não remova o volume `fepublica_postgres_data` durante uma atualização. O bind de migrations é lido pelo Postgres somente na criação de base nova; mudanças de schema em bases existentes exigem procedimento de migração revisado.

### Rollback após a migration 011

O código anterior a `v1.6.0` (incluindo `f1cabdf`/`v1.5`) não é compatível com o ciclo de vida de snapshots introduzido pela migration 011. Não faça rollback trocando apenas a imagem da aplicação.

Para uma base nova, o rollback operacional é para uma imagem `v1.6.0` anterior compatível. Para recuperar uma base existente, restaure um backup do banco em volume isolado e use a imagem pareada com esse backup. Não sobrescreva o volume corrente, registros preservados ou raízes já calculadas. No primeiro release `v1.6.0`, prefira correção forward ou mantenha a API offline até a recuperação estar concluída.

## Backup e restauração

Crie um backup:

```bash
FEPUBLICA_BACKUP_DIR=/var/backups/fepublica ./scripts/backup.sh
```

O script mantém permissões restritas, grava em arquivo temporário, valida o formato com `pg_restore --list` e só então renomeia para o nome final. A retenção é de 14 dias por padrão e afeta apenas `fepublica-postgres-*.dump` no diretório configurado.

Para uma restauração, pare primeiro a API, escolha explicitamente o dump e use um procedimento revisado para o volume existente. Não execute comandos de remoção de schema ou volume como parte da rotina normal.

## Coletores

O coletor inicia com PNCP, que é fonte pública. CEIS, CNEP e cartões exigem uma chave válida do Portal da Transparência e são bloqueados sem ela. A coleta inicial da Câmara deve ser feita manualmente, dentro do limite configurado pelo ambiente.
