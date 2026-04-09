import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api, formatDate, formatNumber, shortHash } from "../lib/api";
import {
  BarChart3,
  Check,
  Download,
  FileSearch,
  Shield,
  Zap,
} from "lucide-react";
import { BrazilMap } from "../components/BrazilMap";
import { SnapshotsChart } from "../components/SnapshotsChart";

export function Landing() {
  const sourcesQ = useQuery({ queryKey: ["sources"], queryFn: api.sources });
  const snapsQ = useQuery({
    queryKey: ["snapshots", { limit: 200 }],
    queryFn: () => api.snapshots({ limit: 200 }),
  });
  const healthQ = useQuery({
    queryKey: ["health"],
    queryFn: api.health,
    refetchInterval: 60_000,
  });

  const sources = sourcesQ.data?.sources ?? [];
  const snapshots = snapsQ.data?.snapshots ?? [];
  const totalEvents = snapshots.reduce((acc, s) => acc + s.record_count, 0);
  const totalAnchored = snapshots.filter((s) => !!s.merkle_root).length;
  const latest = snapshots[0];

  return (
    <div>
      {/* Hero */}
      <section className="container-app pt-10 md:pt-16 pb-8">
        <div className="text-[0.72rem] font-mono uppercase tracking-widest text-accent mb-3">
          Fé Pública · alpha
        </div>
        <h1 className="text-3xl md:text-5xl font-bold leading-[1.1] tracking-tight max-w-4xl">
          Dados públicos brasileiros,
          <br className="hidden sm:block" /> verificáveis criptograficamente.
        </h1>
        <p className="mt-4 text-lg text-ink-dim max-w-3xl">
          Coletamos dados de portais oficiais, organizamos cada coleta em uma
          árvore de Merkle e ancoramos a raiz em Bitcoin via{" "}
          <a
            className="underline hover:text-accent"
            href="https://opentimestamps.org"
            target="_blank"
            rel="noopener"
          >
            OpenTimestamps
          </a>
          . Qualquer pessoa pode baixar uma prova e verificá-la offline — sem
          precisar confiar neste servidor continuar no ar.
        </p>
        <div className="mt-6 flex flex-wrap gap-3 items-center">
          <Link to="/about" className="btn btn-primary">
            <Shield className="size-4" /> Como verificar
          </Link>
          <Link to="/recent" className="btn">
            <Zap className="size-4" /> Mudanças recentes
          </Link>
          <a
            className="btn"
            href="https://github.com/gmowses/fepublica"
            target="_blank"
            rel="noopener"
          >
            <Download className="size-4" /> Repositório
          </a>
          {healthQ.data && (
            <span className="chip">
              <Check className="size-3" /> uptime {healthQ.data.uptime}
            </span>
          )}
        </div>
      </section>

      {/* Stats grid */}
      <section className="container-app py-6">
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
          <Stat k="Fontes monitoradas" v={formatNumber(sources.length)} sub="portais oficiais" />
          <Stat
            k="Snapshots persistidos"
            v={formatNumber(snapshots.length)}
            sub={`${totalAnchored} com raiz merkle`}
          />
          <Stat
            k="Registros arquivados"
            v={formatNumber(totalEvents)}
            sub="eventos individuais"
          />
          <Stat
            k="Última coleta"
            v={latest ? formatDate(latest.collected_at).split(",")[0] : "—"}
            sub={latest ? `fonte: ${latest.source_id}` : "aguardando"}
          />
        </div>
      </section>

      {/* Map + chart */}
      <section className="container-app py-4 grid grid-cols-1 lg:grid-cols-2 gap-4">
        <div className="card">
          <div className="flex items-center justify-between mb-2">
            <div>
              <div className="stat-k">Cobertura geográfica</div>
              <div className="text-lg font-semibold mt-1">
                Estados com dados arquivados
              </div>
            </div>
            <BarChart3 className="size-5 text-accent" />
          </div>
          <BrazilMap />
          <p className="text-xs text-ink-dim mt-2">
            Mapa preliminar: mostra todas as unidades federativas. Segmentação
            automática por UF baseada em eventos coletados será ligada em v0.2
            quando os metadados estiverem indexados.
          </p>
        </div>
        <div className="card">
          <div className="flex items-center justify-between mb-2">
            <div>
              <div className="stat-k">Volume por snapshot</div>
              <div className="text-lg font-semibold mt-1">
                Registros arquivados ao longo do tempo
              </div>
            </div>
            <BarChart3 className="size-5 text-accent" />
          </div>
          <SnapshotsChart snapshots={snapshots} />
          <p className="text-xs text-ink-dim mt-2">
            Cada barra é um snapshot — uma coleta pontual de uma fonte. A
            altura representa quantos registros foram arquivados e hasheados
            naquele momento.
          </p>
        </div>
      </section>

      {/* Sources */}
      <section className="container-app py-6">
        <h2 className="text-xl font-semibold mb-3">Fontes monitoradas</h2>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
          {sources.map((s) => (
            <Link
              key={s.id}
              to={`/sources/${s.id}`}
              className="card hover:border-accent transition"
            >
              <div className="stat-k">{s.id}</div>
              <div className="font-semibold mt-1">{s.name}</div>
              <div className="text-sm text-ink-dim mt-2 line-clamp-3">
                {s.description}
              </div>
            </Link>
          ))}
        </div>
      </section>

      {/* Snapshots table */}
      <section className="container-app py-6">
        <div className="flex items-center justify-between mb-3">
          <h2 className="text-xl font-semibold">Snapshots recentes</h2>
          <Link to="/recent" className="text-sm text-ink-dim hover:text-accent">
            ver mudanças recentes →
          </Link>
        </div>
        <div className="card overflow-x-auto p-0">
          <table className="w-full text-sm">
            <thead className="text-left text-xs uppercase tracking-wider text-ink-dim">
              <tr>
                <th className="px-4 py-3">id</th>
                <th className="px-4 py-3">fonte</th>
                <th className="px-4 py-3">coletado em</th>
                <th className="px-4 py-3 text-right">registros</th>
                <th className="px-4 py-3">raiz merkle</th>
                <th className="px-4 py-3"></th>
              </tr>
            </thead>
            <tbody>
              {snapshots.slice(0, 20).map((s) => (
                <tr key={s.id} className="border-t border-ink/10">
                  <td className="px-4 py-3 font-mono">
                    <Link
                      to={`/snapshots/${s.id}`}
                      className="hover:text-accent"
                    >
                      #{s.id}
                    </Link>
                  </td>
                  <td className="px-4 py-3">{s.source_id}</td>
                  <td className="px-4 py-3 text-ink-dim">
                    {formatDate(s.collected_at)}
                  </td>
                  <td className="px-4 py-3 text-right tabular-nums">
                    {formatNumber(s.record_count)}
                  </td>
                  <td className="px-4 py-3 font-mono text-xs text-ink-dim">
                    {s.merkle_root ? (
                      <span className="chip chip-ok">
                        {shortHash(s.merkle_root, 12)}
                      </span>
                    ) : (
                      <span className="chip chip-warn">pendente</span>
                    )}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <Link
                      to={`/snapshots/${s.id}`}
                      className="text-accent hover:underline inline-flex items-center gap-1"
                    >
                      <FileSearch className="size-4" /> explorar
                    </Link>
                  </td>
                </tr>
              ))}
              {snapshots.length === 0 && (
                <tr>
                  <td
                    colSpan={6}
                    className="px-4 py-10 text-center text-ink-dim"
                  >
                    Ainda sem snapshots. A primeira coleta programada acontece
                    às 04:00 BRT.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}

function Stat({ k, v, sub }: { k: string; v: string; sub?: string }) {
  return (
    <div className="card">
      <div className="stat-k">{k}</div>
      <div className="stat-v">{v}</div>
      {sub && <div className="text-xs text-ink-dim mt-1">{sub}</div>}
    </div>
  );
}
