import { useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { api, formatBytes, formatDate, formatNumber, shortHash } from "../lib/api";
import {
  ArrowLeftRight,
  ChevronLeft,
  ChevronRight,
  FileJson,
  Search,
} from "lucide-react";

export function SnapshotDetail() {
  const { id } = useParams();
  const snapId = Number(id);
  const [search, setSearch] = useState("");
  const [offset, setOffset] = useState(0);
  const pageSize = 50;

  const snapQ = useQuery({
    queryKey: ["snapshot", snapId],
    queryFn: () => api.snapshot(snapId),
    enabled: !Number.isNaN(snapId),
  });
  const anchorsQ = useQuery({
    queryKey: ["snapshot-anchors", snapId],
    queryFn: () => api.snapshotAnchors(snapId),
    enabled: !Number.isNaN(snapId),
  });
  const eventsQ = useQuery({
    queryKey: ["snapshot-events", snapId, search, offset],
    queryFn: () => api.snapshotEvents(snapId, { limit: pageSize, offset, search }),
    enabled: !Number.isNaN(snapId),
  });
  const allSnapsQ = useQuery({
    queryKey: ["snapshots-for-diff", snapQ.data?.source_id],
    queryFn: () => api.snapshots({ source: snapQ.data?.source_id, limit: 20 }),
    enabled: !!snapQ.data?.source_id,
  });

  const diffCandidates = useMemo(() => {
    return (allSnapsQ.data?.snapshots ?? []).filter((s) => s.id !== snapId);
  }, [allSnapsQ.data, snapId]);

  if (snapQ.isLoading) {
    return <div className="container-app py-16 text-ink-dim">carregando…</div>;
  }
  if (snapQ.isError || !snapQ.data) {
    return (
      <div className="container-app py-16 text-ink-dim">
        snapshot não encontrado.{" "}
        <Link to="/" className="text-accent hover:underline">
          voltar
        </Link>
      </div>
    );
  }
  const snap = snapQ.data;

  return (
    <div className="container-app py-6 md:py-10">
      <nav className="text-xs font-mono text-ink-dim mb-3">
        <Link to="/" className="hover:text-accent">
          início
        </Link>{" "}
        / snapshots / <strong className="text-ink">#{snap.id}</strong>
      </nav>
      <div className="flex flex-wrap items-start justify-between gap-3 mb-4">
        <div>
          <h1 className="text-2xl md:text-3xl font-bold tracking-tight">
            Snapshot #{snap.id}
          </h1>
          <div className="text-ink-dim font-mono text-sm mt-1">
            fonte:{" "}
            <Link
              to={`/sources/${snap.source_id}`}
              className="text-accent hover:underline"
            >
              {snap.source_id}
            </Link>
            {" · "}coletado em {formatDate(snap.collected_at)}
          </div>
        </div>
        {diffCandidates.length > 0 && (
          <div className="flex items-center gap-2">
            <label className="text-xs text-ink-dim">comparar com</label>
            <select
              className="bg-bg-card border border-ink/10 rounded-md px-2 py-1 text-sm font-mono"
              onChange={(e) => {
                if (e.target.value) {
                  window.location.href = `/diff/${snap.id}/${e.target.value}`;
                }
              }}
            >
              <option value="">selecione…</option>
              {diffCandidates.map((c) => (
                <option key={c.id} value={c.id}>
                  #{c.id} · {formatDate(c.collected_at)}
                </option>
              ))}
            </select>
          </div>
        )}
      </div>

      <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-6">
        <div className="card">
          <div className="stat-k">Registros</div>
          <div className="stat-v">{formatNumber(snap.record_count)}</div>
        </div>
        <div className="card">
          <div className="stat-k">Tamanho canônico</div>
          <div className="stat-v">{formatBytes(snap.bytes_size)}</div>
        </div>
        <div className="card">
          <div className="stat-k">Coletor</div>
          <div className="stat-v text-base break-all">
            {snap.collector_version}
          </div>
        </div>
        <div className="card">
          <div className="stat-k">Raiz Merkle</div>
          <div className="font-mono text-[0.72rem] break-all mt-1 text-ink-dim">
            {snap.merkle_root ?? "pendente"}
          </div>
        </div>
      </div>

      {/* Anchors */}
      <section className="mb-6">
        <h2 className="text-lg font-semibold mb-2">Âncoras OpenTimestamps</h2>
        {anchorsQ.isLoading && (
          <div className="text-ink-dim text-sm">carregando…</div>
        )}
        {anchorsQ.data?.anchors?.length === 0 && (
          <div className="text-ink-dim text-sm">sem âncoras associadas.</div>
        )}
        {anchorsQ.data?.anchors && anchorsQ.data.anchors.length > 0 && (
          <div className="card overflow-x-auto p-0">
            <table className="w-full text-sm">
              <thead className="text-xs uppercase tracking-wider text-ink-dim">
                <tr>
                  <th className="text-left px-4 py-2">id</th>
                  <th className="text-left px-4 py-2">calendar</th>
                  <th className="text-left px-4 py-2">submetido</th>
                  <th className="text-left px-4 py-2">status</th>
                  <th className="text-left px-4 py-2">receipt</th>
                </tr>
              </thead>
              <tbody>
                {anchorsQ.data.anchors.map((a) => (
                  <tr key={a.id} className="border-t border-ink/10">
                    <td className="px-4 py-2 font-mono">#{a.id}</td>
                    <td className="px-4 py-2">
                      <a
                        className="text-accent hover:underline"
                        href={a.calendar_url}
                        target="_blank"
                        rel="noopener"
                      >
                        {new URL(a.calendar_url).host}
                      </a>
                    </td>
                    <td className="px-4 py-2 text-ink-dim">
                      {formatDate(a.submitted_at)}
                    </td>
                    <td className="px-4 py-2">
                      {a.upgraded ? (
                        <span className="chip chip-ok">
                          confirmado{a.block_height ? ` · block ${a.block_height}` : ""}
                        </span>
                      ) : (
                        <span className="chip chip-warn">pending</span>
                      )}
                    </td>
                    <td className="px-4 py-2 font-mono text-ink-dim">
                      {a.receipt_bytes} B
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      {/* Events */}
      <section>
        <h2 className="text-lg font-semibold mb-2">Eventos</h2>
        <div className="flex items-center gap-2 mb-3">
          <div className="flex items-center gap-2 bg-bg-card border border-ink/10 rounded-md px-3 py-2 flex-1 max-w-md">
            <Search className="size-4 text-ink-dim" />
            <input
              className="bg-transparent outline-none text-sm font-mono flex-1 placeholder:text-ink-dim"
              placeholder="filtrar por external_id…"
              value={search}
              onChange={(e) => {
                setSearch(e.target.value);
                setOffset(0);
              }}
            />
          </div>
          <span className="text-xs text-ink-dim">
            {eventsQ.data ? formatNumber(eventsQ.data.total) + " registros" : ""}
          </span>
        </div>

        <div className="card overflow-x-auto p-0">
          <table className="w-full text-sm">
            <thead className="text-xs uppercase tracking-wider text-ink-dim">
              <tr>
                <th className="text-left px-4 py-2">external_id</th>
                <th className="text-left px-4 py-2">content hash</th>
                <th className="text-left px-4 py-2"></th>
              </tr>
            </thead>
            <tbody>
              {eventsQ.data?.events.map((e) => (
                <tr key={e.id} className="border-t border-ink/10">
                  <td className="px-4 py-2 font-mono">
                    <Link
                      to={`/snapshots/${snap.id}/events/${encodeURIComponent(e.external_id)}`}
                      className="hover:text-accent"
                    >
                      {e.external_id}
                    </Link>
                  </td>
                  <td className="px-4 py-2 font-mono text-[0.72rem] text-ink-dim">
                    {shortHash(e.content_hash, 16)}
                  </td>
                  <td className="px-4 py-2 text-right">
                    <Link
                      to={`/snapshots/${snap.id}/events/${encodeURIComponent(e.external_id)}`}
                      className="text-accent hover:underline inline-flex items-center gap-1"
                    >
                      <FileJson className="size-4" /> ver
                    </Link>
                  </td>
                </tr>
              ))}
              {eventsQ.data?.events.length === 0 && (
                <tr>
                  <td
                    colSpan={3}
                    className="px-4 py-8 text-center text-ink-dim"
                  >
                    nenhum evento{search ? ` para "${search}"` : ""}
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>

        {eventsQ.data && eventsQ.data.total > pageSize && (
          <div className="flex items-center gap-2 mt-3 text-sm">
            <button
              className="btn disabled:opacity-40"
              disabled={offset === 0}
              onClick={() => setOffset(Math.max(0, offset - pageSize))}
            >
              <ChevronLeft className="size-4" /> anterior
            </button>
            <button
              className="btn disabled:opacity-40"
              disabled={offset + pageSize >= eventsQ.data.total}
              onClick={() => setOffset(offset + pageSize)}
            >
              próxima <ChevronRight className="size-4" />
            </button>
            <div className="text-ink-dim ml-auto font-mono text-xs">
              {formatNumber(offset + 1)}–
              {formatNumber(Math.min(offset + pageSize, eventsQ.data.total))} de{" "}
              {formatNumber(eventsQ.data.total)}
            </div>
          </div>
        )}
      </section>

      <div className="mt-8 flex gap-3 flex-wrap">
        {diffCandidates.length > 0 && (
          <Link
            to={`/diff/${snap.id}/${diffCandidates[0].id}`}
            className="btn"
          >
            <ArrowLeftRight className="size-4" /> diff com snapshot anterior
          </Link>
        )}
        <a
          className="btn"
          href={`/api/snapshots/${snap.id}`}
          target="_blank"
          rel="noopener"
        >
          <FileJson className="size-4" /> JSON bruto
        </a>
      </div>
    </div>
  );
}
