# Verificando uma prova do Fé Pública

Uma prova contém o registro preservado, seu hash, o caminho de inclusão na árvore de Merkle e os recibos OpenTimestamps disponíveis. Há duas verificações distintas: a integridade local do registro e a prova temporal contra Bitcoin.

A primeira funciona sem acessar o Fé Pública ou a rede. A segunda também independe do Fé Pública, mas exige acesso aos dados canônicos do Bitcoin. Nenhuma delas demonstra que a informação publicada pela fonte é verdadeira.

## Baixar e preservar a prova

Na página de um evento, use **Baixar prova**. Pela API, substitua o snapshot e o identificador por valores reais da instalação:

```bash
curl --fail --show-error \
  'https://fepublica.mowses.dev.br/api/snapshots/1/events/IDENTIFICADOR/proof' \
  -o proof.json
```

O endpoint atende somente snapshots concluídos com raiz Merkle consistente. Há limites de 200.000 registros por snapshot e 2 MiB para o conteúdo do evento. Uma coleta em andamento, uma raiz pendente ou limites excedidos geram erro explícito.

Preserve o JSON completo. As datas informadas pelo coletor são metadados; não substituem uma prova temporal verificada.

## Verificar o registro e a árvore

Obtenha `fepublica-verify` nas [releases](https://github.com/gmowses/fepublica/releases), usando uma versão compatível com a instalação, ou compile com `make verify-cli`.

```bash
./bin/fepublica-verify proof.json
```

O programa:

1. Recalcula o SHA-256 do conteúdo canônico e compara com o hash declarado.
2. Reconstitui o caminho Merkle e compara com a raiz declarada.
3. Valida a estrutura dos recibos anexados e informa se há atestação de bloco Bitcoin.

Recibos malformados são rejeitados. O campo `upgraded` do JSON não é usado como evidência de confirmação. A saída `Local verification passed` não significa que a ancoragem Bitcoin foi verificada.

A canonicalização usada é a implementação versionada do projeto, em `internal/canonjson`. Não se deve presumir compatibilidade com outra implementação apenas porque ambas ordenam chaves JSON.

## Extrair recibos interoperáveis

```bash
./bin/fepublica-verify extract proof.json --out ./receipts/
```

Os arquivos `.ots` são emitidos no formato `DetachedTimestampFile` v1. Incluem o algoritmo SHA-256, a raiz Merkle como digest e o timestamp serializado recebido do calendário.

O alvo da prova temporal é a raiz da coleta, não o arquivo `proof.json`. Por isso é necessário informar o digest ao cliente oficial:

```bash
ROOT=$(jq -er '.merkle.root' proof.json)
ots info ./receipts/snapshot-1-anchor-1.ots
ots verify -d "$ROOT" ./receipts/snapshot-1-anchor-1.ots
```

Substitua o nome pelo arquivo extraído. Consulte a instalação e configuração de acesso ao Bitcoin na [documentação do cliente oficial](https://github.com/opentimestamps/opentimestamps-client).

## Recibo pendente e atestação Bitcoin

- **Pendente:** há um recibo válido, mas ainda não há atestação Bitcoin nele. Não existe prazo garantido para conclusão.
- **Recibo com atestação Bitcoin:** o timestamp contém uma atestação e a altura do bloco indicada. Isso permite tentar a verificação completa; a presença desses campos não valida o bloco por si só.
- **Verificação completa:** o cliente oficial aplica as operações criptográficas e verifica o compromisso contra os dados canônicos do Bitcoin.

Para atualizar um arquivo pendente, use o cliente oficial:

```bash
ots upgrade ./receipts/snapshot-1-anchor-1.ots
ots verify -d "$ROOT" ./receipts/snapshot-1-anchor-1.ots
```

Quando a verificação completa passa, a cadeia liga o registro preservado à raiz Merkle e ao compromisso temporal do bloco verificado. Isso não comprova autoria da fonte, completude da coleta nem regularidade de um contrato ou despesa.

## Problemas comuns

- **Hash divergente:** o conteúdo não corresponde ao hash declarado. Preserve o arquivo original e compare a versão do verificador.
- **Caminho Merkle divergente:** o registro não reconstrói a raiz declarada. Não trate a prova como válida.
- **Sem recibos:** o snapshot ainda não tem uma submissão OpenTimestamps disponível. As duas verificações locais podem ser executadas, mas o CLI retorna erro porque a prova entregue está sem âncoras.
- **Recibo pendente:** atualize-o depois e tente novamente. O Fé Pública não garante um horário de confirmação.
- **Falha de rede, TLS ou acesso ao Bitcoin:** resolva a conectividade do verificador. Uma falha de acesso não confirma nem invalida o compromisso criptográfico.

## Referências

- [Formato Timestamp e DetachedTimestampFile](https://github.com/opentimestamps/python-opentimestamps/blob/3af46432efc4a4c4e7bba439c1f49bce808eb3e5/opentimestamps/core/timestamp.py)
- [Atestações OpenTimestamps](https://github.com/opentimestamps/python-opentimestamps/blob/3af46432efc4a4c4e7bba439c1f49bce808eb3e5/opentimestamps/core/notary.py)
- [Cliente oficial OpenTimestamps](https://github.com/opentimestamps/opentimestamps-client)
