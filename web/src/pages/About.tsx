import { Link } from "react-router-dom";
import { BookOpen, Github, Shield, Terminal } from "lucide-react";

export function About() {
  return (
    <div className="container-app py-6 md:py-10 max-w-3xl">
      <nav className="text-xs font-mono text-ink-dim mb-3">
        <Link to="/" className="hover:text-accent">
          início
        </Link>{" "}
        / sobre
      </nav>
      <h1 className="text-3xl md:text-4xl font-bold tracking-tight">
        Sobre o Fé Pública
      </h1>
      <p className="text-ink-dim mt-3 text-lg">
        Arquivo imutável e criptograficamente verificável de dados públicos
        brasileiros. Software livre, sem autoridade central, sem dependência
        deste servidor continuar no ar.
      </p>

      <section className="mt-8">
        <h2 className="text-xl font-semibold mb-2">Por que existe</h2>
        <p>
          Dados públicos brasileiros mudam e desaparecem silenciosamente.
          Registros são retirados de listas oficiais sem trilha. Contratos
          recebem emendas retroativas. Jornalistas de investigação, pesquisadores
          e profissionais de compliance precisam provar "esse dado existia nessa
          data" e hoje dependem de capturas de tela que são tecnicamente
          contestáveis.
        </p>
        <p className="mt-3">
          O Fé Pública oferece uma alternativa: um arquivo externo, reprodutível,
          verificável criptograficamente, e que continua válido mesmo que este
          servidor suma amanhã. A prova mora em Bitcoin.
        </p>
      </section>

      <section className="mt-8">
        <h2 className="text-xl font-semibold mb-2">Como funciona</h2>
        <ol className="list-decimal pl-5 space-y-2 text-ink">
          <li>
            <strong>Coletamos</strong> periodicamente dados de portais oficiais
            (Portal da Transparência, PNCP, etc.) via APIs públicas, respeitando
            rate limits e termos de uso.
          </li>
          <li>
            <strong>Canonicalizamos</strong> cada registro em JSON determinístico
            (chaves ordenadas, sem whitespace) e calculamos seu <code>SHA-256</code>.
          </li>
          <li>
            <strong>Agrupamos</strong> todos os hashes da coleta em uma{" "}
            <strong>árvore de Merkle</strong>.
          </li>
          <li>
            <strong>Ancoramos</strong> a raiz da árvore em Bitcoin via{" "}
            <a
              href="https://opentimestamps.org"
              target="_blank"
              rel="noopener"
              className="underline hover:text-accent"
            >
              OpenTimestamps
            </a>
            , que submete o hash a múltiplos calendar servers independentes.
          </li>
          <li>
            <strong>Servimos provas</strong> completas via API — cada registro
            pode ser baixado com sua prova de inclusão na árvore e os receipts
            OpenTimestamps que comprovam a ancoragem em Bitcoin.
          </li>
          <li>
            <strong>Verificação independente</strong>: qualquer pessoa baixa a
            prova e valida offline com um CLI standalone que não depende do
            nosso servidor continuar no ar.
          </li>
        </ol>
      </section>

      <section className="mt-8">
        <h2 className="text-xl font-semibold mb-2">O que garantimos</h2>
        <ul className="list-disc pl-5 space-y-2 text-ink">
          <li>
            <strong>Integridade</strong> — dado um registro e uma prova, qualquer
            um pode verificar que o registro existia naquele snapshot, exatamente
            com aquele conteúdo.
          </li>
          <li>
            <strong>Tempo</strong> — uma vez confirmada a transação Bitcoin que
            ancora a raiz, está estabelecido que o snapshot existia antes daquele
            bloco — e portanto antes de um momento específico no tempo.
          </li>
          <li>
            <strong>Independência</strong> — a verificação não depende do nosso
            servidor. Se sumirmos amanhã, a prova continua válida enquanto o
            Bitcoin existir.
          </li>
          <li>
            <strong>Reprodutibilidade</strong> — a política de coleta é pública e
            versionada. Qualquer um pode rodar uma instância paralela e comparar.
          </li>
        </ul>
      </section>

      <section className="mt-8">
        <h2 className="text-xl font-semibold mb-2">O que não garantimos</h2>
        <ul className="list-disc pl-5 space-y-2 text-ink-dim">
          <li>
            <strong>Autenticidade da fonte</strong>: arquivamos o que a API oficial
            retorna. Se o governo publicou um dado errado, arquivamos o erro.
          </li>
          <li>
            <strong>Completude</strong>: pode haver gaps de coleta por falha de
            API, rate limit, bug. Sempre publicamos nossa cobertura real.
          </li>
          <li>
            <strong>Interpretação</strong>: não somos fact-checker. Somos arquivo.
          </li>
        </ul>
      </section>

      <section className="mt-8">
        <h2 className="text-xl font-semibold mb-2">
          Como verificar uma prova
        </h2>
        <p className="mb-3">
          Baixe qualquer prova e valide offline usando o CLI standalone. Binários
          para macOS, Linux e Windows estão disponíveis nas{" "}
          <a
            href="https://github.com/gmowses/fepublica/releases"
            target="_blank"
            rel="noopener"
            className="underline hover:text-accent"
          >
            releases do GitHub
          </a>
          .
        </p>
        <pre className="bg-bg-soft border border-ink/10 rounded-md p-4 overflow-auto text-[0.78rem] font-mono">
          <Terminal className="size-4 inline text-accent mr-2" />
          <span className="text-ink-dim"># baixar prova</span>
          {"\n"}curl -s "https://fepublica.gmowses.cloud/api/snapshots/1/events/XXX/proof" {">"} proof.json{"\n\n"}
          <span className="text-ink-dim"># verificar offline</span>
          {"\n"}./fepublica-verify proof.json{"\n"}
        </pre>
        <p className="text-sm text-ink-dim mt-3">
          O CLI faz três validações: re-hash do JSON canônico, validação da prova
          Merkle, e listagem dos receipts OpenTimestamps. Para verificação
          fim-a-fim até a transação Bitcoin, extraia os receipts e rode{" "}
          <code>ots verify</code> do cliente oficial OpenTimestamps (
          <code>pip install opentimestamps-client</code>).
        </p>
      </section>

      <section className="mt-8">
        <h2 className="text-xl font-semibold mb-2">Stack técnico</h2>
        <p>
          Escrito em <strong>Go</strong>, com <strong>Postgres</strong> como
          storage imutável (triggers bloqueiam UPDATE/DELETE em eventos
          arquivados). Anchoring via <strong>OpenTimestamps</strong> para não ter
          que operar um full node Bitcoin. Frontend <strong>React + Vite</strong>{" "}
          servido estático pelo próprio binário Go via <code>embed.FS</code>.
          Deployed via Docker Compose. Licenciado{" "}
          <a
            href="https://github.com/gmowses/fepublica/blob/main/LICENSE"
            target="_blank"
            rel="noopener"
            className="underline hover:text-accent"
          >
            AGPL-3.0
          </a>
          .
        </p>
      </section>

      <section className="mt-8 flex flex-wrap gap-3">
        <a
          href="https://github.com/gmowses/fepublica"
          target="_blank"
          rel="noopener"
          className="btn btn-primary"
        >
          <Github className="size-4" /> Repositório
        </a>
        <a
          href="https://github.com/gmowses/fepublica/blob/main/docs/DESIGN.md"
          target="_blank"
          rel="noopener"
          className="btn"
        >
          <BookOpen className="size-4" /> Design doc
        </a>
        <a
          href="https://github.com/gmowses/fepublica/blob/main/docs/verification.md"
          target="_blank"
          rel="noopener"
          className="btn"
        >
          <Shield className="size-4" /> Guia de verificação
        </a>
      </section>
    </div>
  );
}
