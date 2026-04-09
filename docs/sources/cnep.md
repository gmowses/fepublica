# Fonte: CNEP — Cadastro Nacional de Empresas Punidas

Source ID: `cnep`

## Descrição

O CNEP lista empresas que sofreram sanções administrativas com base na Lei Anticorrupção (Lei nº 12.846/2013). É um cadastro específico para as penalidades aplicadas a pessoas jurídicas por atos lesivos contra a administração pública nacional ou estrangeira. Mantido pela CGU.

Diferencia-se do CEIS por cobrir especificamente as sanções da Lei Anticorrupção, enquanto o CEIS cobre um espectro mais amplo de impedimentos de contratar.

## Base legal

- **Lei nº 12.846/2013** (Lei Anticorrupção).
- **Decreto nº 8.420/2015** — Regulamenta a Lei Anticorrupção.
- **Decreto nº 8.777/2016** — Política de Dados Abertos.
- **Lei nº 12.527/2011** — LAI.

## Endpoint

```
GET https://api.portaldatransparencia.gov.br/api-de-dados/cnep
```

Mesma autenticação, paginação e rate limit do CEIS. Ver [`ceis.md`](./ceis.md).

## Esquema

O schema é praticamente idêntico ao do CEIS, adicionando o campo:

- `valorMulta` — string representando o valor da multa aplicada.

Todos os demais campos (`id`, `dataInicioSancao`, `orgaoSancionador`, `sancionado`, etc.) têm o mesmo significado do CEIS.

## Volume

Aproximadamente 2.000–5.000 registros. Volume menor que o CEIS porque sanções da Lei Anticorrupção são menos frequentes que impedimentos de contratar em geral.

## Cadência

Padrão: diária, 04:15 BRT. Configurável via `COLLECTOR_CNEP_SCHEDULE`.

## Identificação

Usamos o campo `id` retornado pela API como `external_id` no Fé Pública.

## Referências

- Portal oficial: https://portaldatransparencia.gov.br/sancoes/cnep
- Texto da Lei Anticorrupção: http://www.planalto.gov.br/ccivil_03/_ato2011-2014/2013/lei/l12846.htm
