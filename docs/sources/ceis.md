# Fonte: CEIS — Cadastro de Empresas Inidôneas e Suspensas

Source ID: `ceis`

## Descrição

O CEIS lista empresas e pessoas físicas impedidas de participar de licitações e contratar com a administração pública federal, estadual e municipal no Brasil. É mantido pela CGU (Controladoria-Geral da União) e consolidado a partir das sanções aplicadas por órgãos da administração pública.

## Base legal

- **Lei nº 12.846/2013** (Lei Anticorrupção) — estabelece responsabilização de pessoas jurídicas por atos contra a administração pública.
- **Decreto nº 8.777/2016** — Política de Dados Abertos do Poder Executivo Federal.
- **Lei nº 12.527/2011** (LAI) — Lei de Acesso à Informação. Base legal para reuso e redistribuição pública.

## Endpoint

```
GET https://api.portaldatransparencia.gov.br/api-de-dados/ceis
```

**Autenticação**: header `chave-api-dados: <seu-token>`. Obter via cadastro gratuito em https://portaldatransparencia.gov.br/api-de-dados/cadastrar-email.

**Paginação**: parâmetro `pagina=N` (1-indexed). Tamanho fixo (~15 registros por página, default do servidor). Parar quando a resposta for um array vazio.

**Rate limit**: 90 req/min no horário comercial, 300 req/min entre 00:00 e 05:59 BRT.

## Esquema (resumo)

Cada registro é um objeto JSON com, entre outros:

- `id` — identificador interno no Portal da Transparência.
- `dataReferencia`, `dataInicioSancao`, `dataFimSancao`, `dataPublicacaoSancao`, `dataTransitadoJulgado`, `dataOrigemInformacao`.
- `tipoSancao` — objeto com `descricaoResumida` e `descricaoPortal`.
- `fonteSancao` — objeto com informações do órgão que registrou a sanção.
- `orgaoSancionador` — objeto com `nome`, `siglaUf`, `poder`, `esfera`.
- `sancionado` — objeto com `nome`, `codigoFormatado` (CNPJ/CPF formatado).
- `fundamentacao` — array de fundamentações legais.
- `textoPublicacao`, `linkPublicacao`, `detalhamentoPublicacao`, `numeroProcesso`.

## Volume

Aproximadamente 25.000–35.000 registros ativos e históricos. Taxa de crescimento: dezenas a centenas de novos registros por mês.

## Identificação (`external_id`) usada pelo Fé Pública

Usamos o campo `id` numérico retornado pela API como identificador externo. Isso garante que:

1. Dois snapshots consecutivos referenciam o mesmo registro pelo mesmo `external_id`.
2. Podemos detectar adições, remoções e alterações via diff entre snapshots.

## Cadência de coleta

Padrão: diária, 04:00 BRT (janela de 300 req/min). Configurável via `COLLECTOR_CEIS_SCHEDULE`.

## Arquivamento e LGPD

Nomes de empresas (CNPJs) e nomes de pessoas físicas sancionadas (CPFs formatados) aparecem nos dados. Esses dados já são públicos por força da Lei 12.846/2013, que determina a publicização das sanções. O Fé Pública não adiciona nenhum processamento adicional a esses campos — apenas arquiva o que a API oficial já publica.

## Referências

- Portal oficial: https://portaldatransparencia.gov.br/sancoes/ceis
- Dicionário de dados: https://portaldatransparencia.gov.br/swagger/
- Política de dados abertos: http://www.planalto.gov.br/ccivil_03/_ato2015-2018/2016/decreto/d8777.htm
