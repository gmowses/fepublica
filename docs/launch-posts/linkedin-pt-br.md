# LinkedIn — PT-BR (rascunho de lançamento v1.6.0)

> Publicar somente após o DNS de `fepublica.mowses.dev.br` estar configurado e a instância ser verificada. Este texto não autoriza publicação automática.

Um dado público mudou. Como demonstrar qual conteúdo você havia preservado antes da mudança?

Essa é a pergunta por trás do Fé Pública, um projeto aberto que estou desenvolvendo para preservar e consultar dados públicos brasileiros.

O funcionamento tem quatro etapas:

1. Coletar registros de APIs oficiais e preservar uma versão da coleta.
2. Representar cada registro de forma determinística e calcular seu hash SHA-256.
3. Reunir os hashes em uma árvore de Merkle. Isso permite demonstrar que um registro pertence à coleta sem baixar todos os outros.
4. Submeter a raiz ao OpenTimestamps. Quando a prova temporal está completa e é verificada contra o Bitcoin, ela permite demonstrar que aquele compromisso criptográfico já existia antes do bloco correspondente.

A aplicação também permite consultar gastos, comparar coletas e examinar cruzamentos entre bases. Os resultados são pontos de partida para investigação; não são conclusões automáticas sobre irregularidades.

Na versão 1.6, uma revisão externa de segurança motivou correções e uma revisão mais ampla do projeto:

- comparações paginadas e provas com limites de tamanho e concorrência;
- proteção de credenciais, mascaramento de CPF na apresentação e reforços de segurança web;
- separação entre coletas em andamento, concluídas e com falha;
- consultas que evitam contar novamente o mesmo registro a cada coleta;
- correção de agregações que poderiam multiplicar valores nos cruzamentos;
- nova interface, com melhor leitura dos dados e navegação no celular;
- testes de integração com PostgreSQL, incluindo falhas parciais e concorrência.

Uma distinção essencial: preservar e verificar a integridade de um conteúdo não demonstra que a informação publicada pela fonte é verdadeira.

Para quem trabalha com engenharia de software, dados ou automação, o código reúne alguns problemas interessantes: histórico imutável, processamento com falhas, provas criptográficas e consultas sobre versões de dados.

Aplicação: https://fepublica.mowses.dev.br
Código: https://github.com/gmowses/fepublica

Que outra fonte de dados públicos você gostaria de poder acompanhar dessa forma?

#DadosAbertos #EngenhariaDeSoftware #OpenSource #Automação
