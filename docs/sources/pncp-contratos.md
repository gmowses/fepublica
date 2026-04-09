# Fonte: PNCP — Contratos Públicos

Source ID: `pncp-contratos`

## Descrição

O Portal Nacional de Contratações Públicas (PNCP) é o agregador federal de todas as contratações públicas brasileiras sob a Nova Lei de Licitações (Lei nº 14.133/2021), obrigatório para todos os entes federativos a partir de 2023. Cobre federal, estadual e municipal.

O recurso `contratos` traz os contratos celebrados — com valor, vigência, objeto, órgão contratante, fornecedor e aditivos.

## Base legal

- **Lei nº 14.133/2021** (Nova Lei de Licitações e Contratos Administrativos) — torna o PNCP obrigatório.
- **Lei nº 12.527/2011** (LAI) — reuso de dados abertos.

## Endpoint

```
GET https://pncp.gov.br/api/consulta/v1/contratos
```

**Autenticação**: **não requer** token. API pública.

**Parâmetros obrigatórios**: `dataInicial` e `dataFinal` (formato `YYYYMMDD`) — janela de consulta, com limite máximo de intervalo imposto pelo servidor.

**Parâmetros opcionais**: `pagina` (default 1), `tamanhoPagina` (máx 500).

**Paginação**: offset via `pagina` e `tamanhoPagina`. A resposta traz `totalPaginas` e `totalRegistros`.

## Estratégia de coleta do Fé Pública (v0.1)

Janela rolante de **30 dias** terminando na data de execução. Isso captura mudanças recentes (adições, aditivos, exclusões) sem sobrecarregar a API pública com backfill completo. Para cobertura histórica além dos últimos 30 dias, usaremos um modo de "backfill" em versões futuras.

## Esquema (resumo)

Cada registro é um objeto JSON com campos relevantes incluindo:

- `numeroControlePNCP` — identificador globalmente único atribuído pelo PNCP.
- `sequencialContrato`, `anoContrato`, `numeroContratoEmpenho`.
- `orgaoEntidade` — objeto com `cnpj`, `razaoSocial`, `poderId`, `esferaId`.
- `unidadeOrgao` — unidade contratante.
- `fornecedor` — objeto com `tipoPessoa`, `cnpjCpfIdgen`, `razaoSocial`.
- `objetoContrato` — descrição textual do objeto.
- `valorInicial`, `valorGlobal`, `valorAcumulado`.
- `dataVigenciaInicio`, `dataVigenciaFim`, `dataAssinatura`, `dataPublicacaoPncp`.
- `tipoContrato`, `categoriaProcesso`.

## Identificação

Usamos `numeroControlePNCP` como `external_id` quando disponível. Em casos raros de ausência, compomos `<cnpj>-<anoContrato>-<sequencialContrato>` como fallback.

## Volume

Variável. Em dias típicos o PNCP recebe milhares de novos registros. A janela de 30 dias do Fé Pública tipicamente resulta em centenas de milhares de registros por snapshot — mas o MVP trabalha com o volume completo dentro da RAM. Para escalar, v0.2 adicionará streaming e particionamento por data.

## Cadência

Padrão: diária, 04:30 BRT. Configurável via `COLLECTOR_PNCP_SCHEDULE`.

## Referências

- Portal oficial: https://pncp.gov.br
- Dados abertos: https://www.gov.br/pncp/pt-br/acesso-a-informacao/dados-abertos
- Documentação da API consulta: https://pncp.gov.br/api/consulta/swagger-ui/index.html
- Lei 14.133/2021: http://www.planalto.gov.br/ccivil_03/_ato2019-2022/2021/lei/l14133.htm
