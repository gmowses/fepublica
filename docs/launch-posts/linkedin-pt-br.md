# LinkedIn — PT-BR (executivo)

> Publicar como post simples, sem capa. Comprimento ~ 1900 caracteres,
> dentro do limite. Tom: técnico-executivo, sem hype.

---

Lancei o **Fé Pública** — um arquivo verificável dos dados públicos brasileiros, ancorado em Bitcoin.

🔗 https://fepublica.gmowses.cloud
💾 https://github.com/gmowses/fepublica

A motivação é simples: dados públicos no Brasil mudam silenciosamente. Empresas saem da lista de impedidos sem aviso. Contratos somem. Documentos viram 404. Quando alguém percebe, não tem mais como provar como estava antes.

O Fé Pública resolve isso fazendo três coisas:

**1. Coleta** automaticamente CEIS, CNEP, contratos do PNCP e transações do CPGF (cartão corporativo do governo federal). Hoje são ~27 mil registros de empresas sancionadas, 1.616 contratos públicos rastreados (R$ 47M+) e snapshots mensais do cartão corporativo.

**2. Sela criptograficamente** cada coleta. Toda snapshot vira uma árvore de Merkle, e a raiz é ancorada em Bitcoin via OpenTimestamps. Se um dado oficial mudar amanhã, dá pra provar como estava ontem — sem precisar confiar no servidor continuar no ar.

**3. Cruza** automaticamente os dados. Quando uma empresa em lista de impedidos (CEIS/CNEP) aparece num contrato público, fica marcada em vermelho. O cidadão vê. O jornalista vê. A defesa do erário vê.

A página /gastos é a parte mais cidadã: digita um CNPJ, vê todos os contratos públicos federais, estaduais e municipais que aquela empresa fechou. Vê quanto ela recebeu. Vê se está em lista de impedidos. Tudo cruzado automaticamente.

A stack: **Go 1.23 + Postgres 16 + React/Vite + Docker Compose**, rodando atrás de Traefik. Sem Kubernetes, sem AWS, sem Lambda — uma única VPS, custo mensal de café. Licença AGPL-3.0.

Não é afiliado a nenhum órgão público. É construído em cima das APIs que já existem, organizadas de um jeito que faz sentido pra quem precisa fiscalizar.

#TransparênciaPública #DadosAbertos #Blockchain #GovTech #SoftwareLivre
