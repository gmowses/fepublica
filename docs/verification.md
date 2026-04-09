# Verificando uma prova do Fé Pública

Este documento explica passo a passo como verificar offline uma prova emitida pela API do Fé Pública, desde o download até a validação criptográfica completa contra Bitcoin via OpenTimestamps.

## Por que verificar

A garantia central do Fé Pública não vem do servidor continuar no ar — vem da prova ser verificável independentemente. Se você precisa provar em um contexto formal (auditoria, processo judicial, pesquisa acadêmica, reportagem) que um dado público existia em uma data específica, a prova emitida pela API substitui capturas de tela e referências a URLs que podem ser editadas.

## O que você precisa

1. **A prova** — arquivo JSON baixado de `GET /snapshots/{id}/events/{external_id}/proof`.
2. **O CLI `fepublica-verify`** — binário standalone. Download em [releases](https://github.com/gmowses/fepublica/releases) ou compile do fonte (`make verify-cli`).
3. **(Opcional) O CLI oficial OpenTimestamps** — para verificar a âncora final em Bitcoin. Instalar com `pip install opentimestamps-client`.

Nada disso depende do servidor Fé Pública continuar respondendo. Se o servidor sumir amanhã e você tiver salvo a prova, ela continua válida enquanto Bitcoin existir.

## Passo 1 — Baixar a prova

```bash
curl -s "https://fepublica.gmowses.cloud/snapshots/1/events/277765/proof" > proof.json
```

Inspecione o conteúdo:

```bash
jq '{version, source_id, snapshot_id, event: .event.external_id, root: .merkle.root, anchors: (.anchors | length)}' proof.json
```

Saída esperada:

```json
{
  "version": 1,
  "source_id": "cnep",
  "snapshot_id": 1,
  "event": "277765",
  "root": "a8c24fa6...",
  "anchors": 3
}
```

## Passo 2 — Verificação local com `fepublica-verify`

```bash
./fepublica-verify proof.json
```

Saída esperada:

```
[1/3] content hash matches: sha256:febff87b24e44c34b98d3e2938b5d5aadc01d4b7b07761efb269d0ecb7c6325c
[2/3] merkle proof valid (root sha256:a8c24fa68e6e4ead980559f0ed75aea96a5e56a5695ba3e00586391f09204f25)
[3/3] OTS anchors attached:
  #1 https://alice.btc.calendar.opentimestamps.org
      status=pending receipt=172 bytes submitted=2026-04-09T15:51:58Z
  ...
Local verification passed.
```

As três verificações que o CLI faz:

### 2.1 — Re-hash do JSON canônico

O CLI pega o `event.canonical_json` da prova, serializa-o em forma canônica (chaves ordenadas alfabeticamente, sem whitespace extra), calcula `SHA-256`, e compara com o `event.content_hash` declarado. Se não bater, a prova foi adulterada.

### 2.2 — Validação da prova Merkle

O CLI aplica cada passo em `merkle.siblings` (cada passo indica um hash irmão e seu lado — esquerdo ou direito), hasheia par a par subindo a árvore, e compara o resultado final com `merkle.root` declarado. Se não bater, o evento não pertence à árvore ancorada.

### 2.3 — Enumeração das âncoras OTS

O CLI lista as âncoras anexadas, mostrando para cada uma: o calendar server, o status (`pending` ou `upgraded`), o bloco Bitcoin (se `upgraded`) e o tamanho do receipt.

Um `status=pending` significa que o calendar server ainda não publicou a transação Bitcoin que confirma essa raiz. Calendars tipicamente batizam a cada poucas horas. Uma âncora `upgraded=true` garante que o receipt já contém a prova de inclusão em uma transação Bitcoin confirmada.

## Passo 3 — Extrair e verificar com `ots verify` (verificação fim-a-fim)

Se pelo menos uma âncora está `upgraded`, você pode verificar a cadeia completa até Bitcoin usando o CLI oficial:

```bash
./fepublica-verify extract proof.json --out ./receipts/
ots verify ./receipts/snapshot-1-anchor-1.ots
```

O `ots verify` vai:

1. Decodificar o receipt binário.
2. Seguir a cadeia de hashing interna até a raiz commitada na transação Bitcoin.
3. Consultar um full node ou um servidor SPV para confirmar que essa transação existe, está confirmada, e está em que bloco.
4. Retornar o timestamp (data/hora do bloco Bitcoin).

Se o `ots verify` passa, você tem prova criptográfica independente de que:

- O `content_hash` declarado na prova bate com o JSON canônico.
- Esse hash estava incluído na árvore de Merkle do snapshot.
- Essa árvore foi ancorada em Bitcoin **antes** do timestamp retornado.

Combinado: **o dado público `external_id` existia com esse conteúdo exato antes desse bloco Bitcoin, e não pode ter sido alterado desde então sem que a raiz da árvore seja quebrada**.

## Verificando sem o `ots verify`

Se você não pode instalar o cliente Python, ainda assim o `fepublica-verify` faz as duas primeiras validações (hash + Merkle), que cobrem 90% dos casos práticos (aqueles em que o ponto de dúvida é "esse dado existia com esse conteúdo naquele snapshot?"). A verificação Bitcoin é a camada adicional que responde "em que momento do tempo esse snapshot foi finalizado?".

## Em um processo formal

Se você está usando uma prova do Fé Pública como evidência em um contexto formal (processo judicial, auditoria fiscal, dissertação acadêmica), recomendamos:

1. **Baixar e arquivar a prova JSON completa** — inclui o `canonical_json` do evento, que é a fonte de verdade.
2. **Rodar o `fepublica-verify` no momento do arquivamento** — registra localmente que as validações 1-2 passaram.
3. **Extrair os receipts `.ots` e arquivá-los junto**.
4. **Aguardar pelo menos 24 horas e re-verificar com `ots verify`** — nesse ponto pelo menos um dos calendars terá confirmado em Bitcoin.
5. **Documentar o bloco Bitcoin** no qual a raiz foi ancorada, e a data/hora desse bloco, como parte do laudo/relatório.

O bloco Bitcoin serve como o timestamp de autoridade máxima: nenhuma parte pode contestar "isso foi forjado depois", porque reescrever Bitcoin após confirmação profunda é economicamente impraticável.

## Problemas comuns

### "content hash mismatch"

O `canonical_json` na prova foi alterado depois da emissão, ou o CLI tem um bug de canonicalização. Reporte uma issue com a prova problemática.

### "merkle proof does not reconstruct declared root"

O `siblings` ou o `index` foi alterado, ou há incompatibilidade de versão entre o CLI e o servidor. Use um CLI de versão compatível com o servidor que emitiu a prova.

### "ots verify: commitment not found"

A âncora está `upgraded=true` mas o `ots verify` não consegue encontrar a transação Bitcoin referenciada. Pode ser um dos:

- Seu cliente `ots` está consultando um SPV que não tem o bloco.
- A transação Bitcoin referenciada ainda não alcançou a profundidade mínima.
- O receipt foi gerado por um calendar que tem um bug.

Nesse caso, use as outras âncoras (a prova tem 3 por padrão) ou aguarde confirmações adicionais.

## Referências

- Protocolo OpenTimestamps: https://github.com/opentimestamps/opentimestamps-server/blob/master/doc/design.md
- RFC 8785 (JSON Canonicalization Scheme): https://datatracker.ietf.org/doc/html/rfc8785
- Binary Merkle tree: https://en.wikipedia.org/wiki/Merkle_tree
