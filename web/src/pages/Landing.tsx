import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api, formatDate, formatNumber, shortHash } from "../lib/api";
import {
  ArrowRight,
  CheckCircle2,
  Database,
  FileCheck2,
  Fingerprint,
  Search,
  ShieldCheck,
} from "lucide-react";
import { SnapshotsChart } from "../components/SnapshotsChart";

export function Landing() {
  const sourcesQ = useQuery({ queryKey: ["sources"], queryFn: api.sources });
  const snapsQ = useQuery({
    queryKey: ["snapshots", { limit: 200 }],
    queryFn: () => api.snapshots({ limit: 200 }),
  });
  const sources = sourcesQ.data?.sources ?? [];
  const snapshots = snapsQ.data?.snapshots ?? [];
  const completeSnapshots = snapshots.filter(
    (snapshot) => snapshot.lifecycle_status === "complete",
  );
  const completeRecords = completeSnapshots.reduce(
    (total, snapshot) => total + snapshot.record_count,
    0,
  );
  const latestComplete = completeSnapshots[0];

  return (
    <div className="pb-8">
      <section className="container-app pt-10 md:pt-16">
        <div className="grid items-start gap-10 lg:grid-cols-[minmax(0,1fr)_23rem] lg:gap-16">
          <div>
            <p className="stat-k mb-4 text-accent">
              Arquivo público verificável · release 1.6
            </p>
            <h1 className="max-w-3xl font-display text-4xl font-bold leading-[0.98] tracking-[-0.045em] text-ink sm:text-5xl md:text-6xl">
              Dados públicos.
              <br />
              Memória verificável.
            </h1>
            <p className="mt-6 max-w-2xl text-base leading-7 text-ink-dim md:text-lg">
              Fé Pública preserva coletas de fontes oficiais para que cidadãos,
              jornalistas e pesquisadores possam consultar, comparar e examinar
              os registros ao longo do tempo.
            </p>
            <div className="mt-7 flex flex-wrap gap-3">
              <Link to="/gastos" className="btn btn-primary">
                <Search className="size-4" /> Explorar gastos{" "}
                <ArrowRight className="size-4" />
              </Link>
              <Link to="/about" className="btn">
                <ShieldCheck className="size-4" /> Como verificamos
              </Link>
            </div>
          </div>
          <PreservationFlow />
        </div>
      </section>

      <section className="container-app mt-14 border-y border-ink/15 py-6">
        <div className="grid gap-6 sm:grid-cols-3">
          <Overview
            k="Fontes disponíveis"
            v={
              sourcesQ.isLoading
                ? "…"
                : sourcesQ.isError
                  ? "—"
                  : formatNumber(sources.length)
            }
            sub={
              sourcesQ.isError
                ? "catálogo indisponível"
                : "cadastradas neste catálogo"
            }
          />
          <Overview
            k="Coletas concluídas"
            v={
              snapsQ.isLoading
                ? "…"
                : snapsQ.isError
                  ? "—"
                  : formatNumber(completeSnapshots.length)
            }
            sub={
              snapsQ.isError
                ? "arquivo recente indisponível"
                : "entre as 200 mais recentes"
            }
          />
          <Overview
            k="Registros em coletas concluídas"
            v={
              snapsQ.isLoading
                ? "…"
                : snapsQ.isError
                  ? "—"
                  : formatNumber(completeRecords)
            }
            sub={
              snapsQ.isError
                ? "arquivo recente indisponível"
                : "entre as 200 mais recentes"
            }
          />
        </div>
      </section>

      <section className="container-app py-12 md:py-16">
        <div className="flex flex-wrap items-end justify-between gap-4">
          <div>
            <p className="stat-k mb-2">Para investigar</p>
            <h2 className="font-display text-2xl font-bold tracking-tight md:text-3xl">
              Comece pela pergunta que importa.
            </h2>
          </div>
          <Link
            to="/recent"
            className="text-sm font-semibold text-accent hover:underline"
          >
            Ver coletas recentes
          </Link>
        </div>
        <div className="mt-6 grid gap-4 md:grid-cols-3">
          <UseCase
            icon={Search}
            title="Consultar gastos"
            body="Pesquise contratos e despesas por órgão, fornecedor ou identificador."
            to="/gastos"
            cta="Explorar gastos"
          />
          <UseCase
            icon={FileCheck2}
            title="Comparar coletas"
            body="Abra um snapshot e compare versões para examinar mudanças nos registros."
            to="/recent"
            cta="Ver coletas"
          />
          <UseCase
            icon={Fingerprint}
            title="Examinar a prova"
            body="Baixe uma prova de inclusão quando a raiz Merkle estiver disponível."
            to="/about"
            cta="Entender a verificação"
          />
        </div>
      </section>

      <section className="container-app grid gap-5 border-t border-ink/15 py-12 lg:grid-cols-[minmax(0,1.35fr)_minmax(18rem,.65fr)]">
        <div className="card overflow-hidden p-0">
          <div className="flex flex-wrap items-start justify-between gap-3 border-b border-ink/15 px-5 py-4">
            <div>
              <p className="stat-k">Arquivo recente</p>
              <h2 className="mt-1 font-display text-xl font-bold tracking-tight">
                Coletas concluídas
              </h2>
            </div>
            <span className="chip chip-ok">
              <CheckCircle2 className="size-3" /> ciclo encerrado
            </span>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="text-left text-ink-dim">
                <tr>
                  <th className="px-5 py-3">snapshot</th>
                  <th className="px-5 py-3">fonte</th>
                  <th className="px-5 py-3">coletado em</th>
                  <th className="px-5 py-3 text-right">registros</th>
                  <th className="px-5 py-3">raiz Merkle</th>
                </tr>
              </thead>
              <tbody>
                {completeSnapshots.slice(0, 8).map((snapshot) => (
                  <tr key={snapshot.id}>
                    <td className="px-5 py-3 font-mono">
                      <Link
                        to={`/snapshots/${snapshot.id}`}
                        className="font-medium text-accent hover:underline"
                      >
                        #{snapshot.id}
                      </Link>
                    </td>
                    <td className="px-5 py-3">{snapshot.source_id}</td>
                    <td className="px-5 py-3 text-ink-dim">
                      {formatDate(snapshot.collected_at)}
                    </td>
                    <td className="px-5 py-3 text-right tabular-nums">
                      {formatNumber(snapshot.record_count)}
                    </td>
                    <td className="px-5 py-3 font-mono text-xs text-ink-dim">
                      {snapshot.merkle_root
                        ? shortHash(snapshot.merkle_root, 12)
                        : "raiz pendente"}
                    </td>
                  </tr>
                ))}
                {snapsQ.isError && (
                  <tr>
                    <td
                      colSpan={5}
                      className="px-5 py-10 text-center text-ink-dim"
                    >
                      Não foi possível carregar as coletas recentes.{" "}
                      <button
                        type="button"
                        className="font-semibold text-accent hover:underline"
                        onClick={() => snapsQ.refetch()}
                      >
                        Tentar novamente
                      </button>
                    </td>
                  </tr>
                )}
                {!snapsQ.isLoading &&
                  !snapsQ.isError &&
                  completeSnapshots.length === 0 && (
                    <tr>
                      <td
                        colSpan={5}
                        className="px-5 py-10 text-center text-ink-dim"
                      >
                        Ainda não há coletas concluídas entre as 200 mais
                        recentes.
                      </td>
                    </tr>
                  )}
              </tbody>
            </table>
          </div>
        </div>
        <div className="card">
          <p className="stat-k">Volume observado</p>
          <h2 className="mt-1 font-display text-xl font-bold tracking-tight">
            Registros por coleta
          </h2>
          {snapsQ.isError ? (
            <div className="flex h-56 items-center text-sm text-ink-dim">
              Não foi possível carregar o volume das coletas.
            </div>
          ) : (
            <SnapshotsChart snapshots={completeSnapshots} />
          )}
          <p className="text-sm leading-6 text-ink-dim">
            O gráfico considera somente coletas concluídas entre os 200
            snapshots carregados nesta página.
          </p>
        </div>
      </section>

      <section className="container-app py-8">
        <div className="flex flex-wrap items-end justify-between gap-3">
          <div>
            <p className="stat-k mb-2">Catálogo</p>
            <h2 className="font-display text-2xl font-bold tracking-tight">
              Fontes disponíveis
            </h2>
          </div>
          {latestComplete && (
            <p className="text-sm text-ink-dim">
              Última coleta concluída: {formatDate(latestComplete.collected_at)}
            </p>
          )}
        </div>
        {sourcesQ.isError ? (
          <p className="mt-5 card text-sm text-ink-dim">
            Não foi possível carregar o catálogo de fontes.{" "}
            <button
              type="button"
              className="font-semibold text-accent hover:underline"
              onClick={() => sourcesQ.refetch()}
            >
              Tentar novamente
            </button>
          </p>
        ) : (
          <div className="mt-5 grid gap-4 md:grid-cols-2 xl:grid-cols-3">
            {sources.map((source) => (
              <Link
                key={source.id}
                to={`/sources/${source.id}`}
                className="card group hover:border-accent"
              >
                <Database className="size-5 text-accent" />
                <p className="mt-4 font-mono text-xs text-ink-dim">
                  {source.id}
                </p>
                <h3 className="mt-1 font-semibold group-hover:text-accent">
                  {source.name}
                </h3>
                {source.description && (
                  <p className="mt-2 line-clamp-3 text-sm leading-6 text-ink-dim">
                    {source.description}
                  </p>
                )}
              </Link>
            ))}
            {!sourcesQ.isLoading && sources.length === 0 && (
              <p className="card text-sm text-ink-dim">
                Nenhuma fonte está disponível no catálogo neste momento.
              </p>
            )}
          </div>
        )}
      </section>
    </div>
  );
}

function PreservationFlow() {
  const steps = [
    ["Fonte", "Coleta em API oficial"],
    ["Registro", "Conteúdo canônico recebe hash"],
    ["Conjunto", "Raiz Merkle reúne os registros"],
    ["Tempo", "Prova pode ser submetida a carimbos de tempo"],
  ];
  return (
    <aside className="border-l-2 border-accent bg-bg-soft px-5 py-5">
      <p className="stat-k text-accent">Da fonte à prova</p>
      <p className="mt-2 text-sm leading-6 text-ink-dim">
        Um resumo do processo de preservação. A raiz Merkle, sozinha, não
        confirma uma âncora em Bitcoin.
      </p>
      <ol className="mt-5 space-y-4">
        {steps.map(([title, detail], index) => (
          <li key={title} className="grid grid-cols-[1.5rem_1fr] gap-3">
            <span className="font-mono text-xs text-accent">0{index + 1}</span>
            <span>
              <strong className="block text-sm">{title}</strong>
              <span className="text-sm text-ink-dim">{detail}</span>
            </span>
          </li>
        ))}
      </ol>
    </aside>
  );
}

function Overview({ k, v, sub }: { k: string; v: string; sub: string }) {
  return (
    <div>
      <p className="stat-k">{k}</p>
      <p className="stat-v text-3xl">{v}</p>
      <p className="mt-1 text-sm text-ink-dim">{sub}</p>
    </div>
  );
}

function UseCase({
  icon: Icon,
  title,
  body,
  to,
  cta,
}: {
  icon: typeof Search;
  title: string;
  body: string;
  to: string;
  cta: string;
}) {
  return (
    <Link to={to} className="card group">
      <Icon className="size-5 text-accent" />
      <h3 className="mt-5 font-display text-xl font-bold tracking-tight">
        {title}
      </h3>
      <p className="mt-2 text-sm leading-6 text-ink-dim">{body}</p>
      <span className="mt-5 inline-flex items-center gap-1 text-sm font-semibold text-accent group-hover:underline">
        {cta} <ArrowRight className="size-4" />
      </span>
    </Link>
  );
}
