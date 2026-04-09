import { Link, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { api, formatDate, formatNumber, shortHash } from "../lib/api";
import { SnapshotsChart } from "../components/SnapshotsChart";
import { ExternalLink } from "lucide-react";

export function SourceDetail() {
  const { id } = useParams();

  const sourcesQ = useQuery({ queryKey: ["sources"], queryFn: api.sources });
  const snapsQ = useQuery({
    queryKey: ["snapshots-by-source", id],
    queryFn: () => api.snapshots({ source: id, limit: 100 }),
    enabled: !!id,
  });

  const source = sourcesQ.data?.sources.find((s) => s.id === id);
  const snapshots = snapsQ.data?.snapshots ?? [];
  const totalEvents = snapshots.reduce((acc, s) => acc + s.record_count, 0);

  if (!source && sourcesQ.isSuccess) {
    return (
      <div className="container-app py-16 text-ink-dim">
        fonte "{id}" não encontrada.{" "}
        <Link to="/" className="text-accent hover:underline">
          voltar
        </Link>
      </div>
    );
  }

  return (
    <div className="container-app py-6 md:py-10">
      <nav className="text-xs font-mono text-ink-dim mb-3">
        <Link to="/" className="hover:text-accent">
          início
        </Link>{" "}
        / sources / <strong className="text-ink">{id}</strong>
      </nav>
      {source && (
        <>
          <h1 className="text-2xl md:text-3xl font-bold tracking-tight">
            {source.name}
          </h1>
          <div className="text-ink-dim text-sm mt-1 font-mono">
            source_id: {source.id}
          </div>
          <p className="text-ink-dim mt-3 max-w-3xl">{source.description}</p>
          <a
            className="btn mt-4"
            href={source.base_url}
            target="_blank"
            rel="noopener"
          >
            <ExternalLink className="size-4" /> API oficial
          </a>
        </>
      )}

      <div className="grid grid-cols-2 md:grid-cols-3 gap-3 mt-6">
        <div className="card">
          <div className="stat-k">Snapshots</div>
          <div className="stat-v">{formatNumber(snapshots.length)}</div>
        </div>
        <div className="card">
          <div className="stat-k">Registros totais</div>
          <div className="stat-v">{formatNumber(totalEvents)}</div>
        </div>
        <div className="card">
          <div className="stat-k">Última coleta</div>
          <div className="stat-v text-base">
            {snapshots[0] ? formatDate(snapshots[0].collected_at) : "—"}
          </div>
        </div>
      </div>

      <div className="card mt-6">
        <div className="stat-k mb-1">Volume por coleta</div>
        <SnapshotsChart snapshots={snapshots} />
      </div>

      <h2 className="text-lg font-semibold mt-8 mb-2">Histórico de snapshots</h2>
      <div className="card overflow-x-auto p-0">
        <table className="w-full text-sm">
          <thead className="text-xs uppercase tracking-wider text-ink-dim">
            <tr>
              <th className="text-left px-4 py-2">id</th>
              <th className="text-left px-4 py-2">coletado em</th>
              <th className="text-right px-4 py-2">registros</th>
              <th className="text-left px-4 py-2">raiz merkle</th>
              <th className="text-left px-4 py-2"></th>
            </tr>
          </thead>
          <tbody>
            {snapshots.map((s) => (
              <tr key={s.id} className="border-t border-ink/10">
                <td className="px-4 py-2 font-mono">#{s.id}</td>
                <td className="px-4 py-2 text-ink-dim">
                  {formatDate(s.collected_at)}
                </td>
                <td className="px-4 py-2 text-right tabular-nums">
                  {formatNumber(s.record_count)}
                </td>
                <td className="px-4 py-2 font-mono text-[0.72rem] text-ink-dim">
                  {s.merkle_root ? (
                    <span className="chip chip-ok">
                      {shortHash(s.merkle_root, 14)}
                    </span>
                  ) : (
                    <span className="chip chip-warn">pendente</span>
                  )}
                </td>
                <td className="px-4 py-2">
                  <Link
                    to={`/snapshots/${s.id}`}
                    className="text-accent hover:underline"
                  >
                    explorar →
                  </Link>
                </td>
              </tr>
            ))}
            {snapshots.length === 0 && (
              <tr>
                <td colSpan={5} className="px-4 py-8 text-center text-ink-dim">
                  sem snapshots ainda para esta fonte
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
