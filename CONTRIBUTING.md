# Contribuindo com Fé Pública

Obrigado pelo interesse. Este documento explica como rodar o projeto localmente, o fluxo de contribuição, e as áreas onde contribuições são mais bem-vindas.

## Rodando localmente

### Pré-requisitos

- Go 1.23+
- Docker e Docker Compose
- Um token da API do Portal da Transparência. Cadastre-se em https://portaldatransparencia.gov.br/api-de-dados/cadastrar-email — o token chega por email.

### Setup

```bash
git clone https://github.com/gmowses/fepublica
cd fepublica
cp .env.example .env
# edite .env e coloque seu TRANSPARENCIA_API_KEY

make tidy
make build
make up
make migrate
```

### Rodando um coletor sob demanda

```bash
docker compose run --rm collector run --source ceis
docker compose run --rm collector run --source cnep
```

### Rodando testes

```bash
make test
make test-cover
```

### Lint

```bash
make lint
```

Configurado via `.golangci.yml` na raiz do repo. Recomendado rodar antes de abrir MR.

## Como adicionar uma nova fonte

Uma "fonte" é qualquer portal público brasileiro com API ou endpoint acessível, cujos dados são estáveis o suficiente para ser arquivados.

Passos:

1. Crie um pacote em `internal/transparencia/<nome>/` implementando a interface `Source`:

   ```go
   type Source interface {
       Name() string
       Fetch(ctx context.Context, since time.Time) ([]RawRecord, error)
       Parse(raw RawRecord) (Record, error)
   }
   ```

2. Registre a fonte em `cmd/collector/main.go`.

3. Adicione uma linha em `db/migrations/002_add_source_<nome>.sql` para inserir a fonte na tabela `sources`.

4. Documente a fonte em `docs/sources/<nome>.md` com: URL base, auth (se houver), ToS, rate limit, volume estimado, e por que essa fonte vale arquivar.

5. Escreva testes. Cobertura mínima: parser lidando com registros malformados, registros parciais, e um registro "dourado" de exemplo real.

## Áreas onde contribuições são especialmente bem-vindas

1. **Novas fontes** — PNCP, DOU, SALIC, Câmara e Senado, ministérios específicos. Começar pelo PNCP é mais impactante.
2. **Parser defensivo** — APIs governamentais mudam schemas sem aviso. Testes de propriedade para fuzzing de input seriam ouro.
3. **Tradução** — README e documentação em inglês. O projeto é brasileiro por natureza, mas interesse internacional ajudaria.
4. **Casos de uso documentados** — abra uma issue descrevendo um caso de uso jornalístico, acadêmico ou de compliance. Cada caso real nos ajuda a priorizar features.
5. **Verificação independente em outras linguagens** — uma implementação do `verify` em Python ou Rust como segunda opinião seria valiosa.

## Código de conduta

Seja decente. Sem discussão. Contribuidores com comportamento tóxico são removidos.

## Licença das contribuições

Ao contribuir, você concorda que suas contribuições serão licenciadas sob os mesmos termos do projeto — **AGPL-3.0**. Veja `LICENSE`.

## Abrindo um MR

1. Fork do repositório.
2. Branch descritiva: `feat/pncp-collector`, `fix/ceis-pagination`, `docs/english-readme`.
3. Commits em inglês, no presente do imperativo: `add pncp collector`, `fix off-by-one in merkle proof`.
4. Um MR por tópico. Não misture refactor com feature.
5. Inclua testes quando o código afeta lógica.
6. CI deve passar.

## Testando a instância pública antes de mergear

A instância oficial em `fepublica.gmowses.cloud` é o ambiente de referência. Antes de mergear mudança que afeta a API de verificação ou o formato da prova, teste contra uma prova real emitida pela instância pública para garantir retrocompatibilidade.

## Contato

Abra uma issue. Para assuntos sensíveis (vazamento de chave, problema de privacidade, etc.), contate o mantenedor diretamente via email listado no perfil do mantenedor no GitHub.
