import { Link, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { api, formatDate, formatNumber, shortHash } from "../lib/api";
import { ArrowLeftRight, Minus, Plus, PenLine } from "lucide-react";

export function DiffViewer() {
  const { a, b } = useParams();
  const idA = Number(a);
  const idB = Number(b);

  const diffQ = useQuery({
    queryKey: ["diff", idA, idB],
    queryFn: () => api.diff(idA, idB),
    enabled: !!idA && !!idB,
  });

  if (diffQ.isLoading) {
    return (
      <div className="container-app py-16 text-ink-dim">
        calculando diff…
      </div>
    );
  }
  if (diffQ.isError || !diffQ.data) {
    return (
      <div className="container-app py-16 text-ink-dim">
        não foi possível carregar o diff.{" "}
        <Link to="/" className="text-accent hover:underline">
          voltar
        </Link>
      </div>
    );
  }
  const d = diffQ.data;
  const added = d.added ?? [];
  const removed = d.removed ?? [];
  const changed = d.changed ?? [];

  return (
    <div className="container-app py-6 md:py-10">
      <nav className="text-xs font-mono text-ink-dim mb-3">
        <Link to="/" className="hover:text-accent">
          início
        </Link>{" "}
        / diff{" "}
        <Link to={`/snapshots/${idA}`} className="hover:text-accent">
          #{idA}
        </Link>{" "}
        ↔{" "}
        <Link to={`/snapshots/${idB}`} className="hover:text-accent">
          #{idB}
        </Link>
      </nav>

      <div className="flex items-center gap-3 mb-2">
        <ArrowLeftRight className="size-6 text-accent" />
        <h1 className="text-2xl md:text-3xl font-bold tracking-tight">
          Diff entre snapshots
        </h1>
      </div>
      <div className="text-sm text-ink-dim font-mono mb-6">
        fonte: {d.source_id} · {formatDate(d.snapshot_a.collected_at)} →{" "}
        {formatDate(d.snapshot_b.collected_at)}
      </div>

      {/* Summary */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-3 mb-6">
        <SummaryCard
          label="adicionados"
          count={d.summary.added}
          total={d.snapshot_b.record_count}
          color="text-ok"
          Icon={Plus}
        />
        <SummaryCard
          label="removidos"
          count={d.summary.removed}
          total={d.snapshot_a.record_count}
          color="text-danger"
          Icon={Minus}
        />
        <SummaryCard
          label="alterados"
          count={d.summary.changed}
          total={Math.max(d.snapshot_a.record_count, d.snapshot_b.record_count)}
          color="text-accent"
          Icon={PenLine}
        />
      </div>

      {d.summary.added === 0 &&
        d.summary.removed === 0 &&
        d.summary.changed === 0 && (
          <div className="card text-center py-10">
            <div className="text-4xl mb-3">∅</div>
            <div className="text-lg font-semibold">
              Nada mudou entre os dois snapshots
            </div>
            <div className="text-sm text-ink-dim mt-2">
              Os {formatNumber(d.snapshot_a.record_count)} registros são
              idênticos em ambas as coletas.
            </div>
          </div>
        )}

      <DiffList
        title="Registros adicionados"
        subtitle="presentes no snapshot mais novo, ausentes no anterior"
        items={added}
        snapshotId={idB}
        color="ok"
        hashField="hash_b"
      />
      <DiffList
        title="Registros removidos"
        subtitle="presentes no snapshot anterior, ausentes no mais novo — possível edição silenciosa"
        items={removed}
        snapshotId={idA}
        color="danger"
        hashField="hash_a"
      />
      <DiffList
        title="Registros alterados"
        subtitle="presentes em ambos com content hash diferente — o conteúdo do registro mudou"
        items={changed}
        snapshotId={idB}
        color="warn"
        hashField="hash_b"
        showBothHashes
      />
    </div>
  );
}

function SummaryCard({
  label,
  count,
  total,
  color,
  Icon,
}: {
  label: string;
  count: number;
  total: number;
  color: string;
  Icon: any;
}) {
  const pct = total > 0 ? ((count / total) * 100).toFixed(2) : "0";
  return (
    <div className="card">
      <div className="flex items-center justify-between">
        <div className="stat-k">{label}</div>
        <Icon className={`size-4 ${color}`} />
      </div>
      <div className={`stat-v ${color}`}>{formatNumber(count)}</div>
      <div className="text-xs text-ink-dim mt-1">{pct}% do total</div>
    </div>
  );
}

function DiffList({
  title,
  subtitle,
  items,
  snapshotId,
  color,
  hashField,
  showBothHashes = false,
}: {
  title: string;
  subtitle: string;
  items: Array<{ external_id: string; hash_a?: string; hash_b?: string }>;
  snapshotId: number;
  color: "ok" | "danger" | "warn";
  hashField: "hash_a" | "hash_b";
  showBothHashes?: boolean;
}) {
  if (items.length === 0) return null;
  const chipClass =
    color === "ok" ? "chip-ok" : color === "danger" ? "chip-danger" : "chip-warn";
  return (
    <section className="mb-6">
      <h2 className="text-lg font-semibold mb-1">
        {title} <span className={`chip ${chipClass} ml-2`}>{items.length}</span>
      </h2>
      <p className="text-sm text-ink-dim mb-2">{subtitle}</p>
      <div className="card overflow-x-auto p-0">
        <table className="w-full text-sm">
          <thead className="text-xs uppercase tracking-wider text-ink-dim">
            <tr>
              <th className="text-left px-4 py-2">external_id</th>
              <th className="text-left px-4 py-2">hash</th>
              {showBothHashes && <th className="text-left px-4 py-2">hash novo</th>}
              <th className="text-left px-4 py-2"></th>
            </tr>
          </thead>
          <tbody>
            {items.slice(0, 200).map((it) => (
              <tr key={it.external_id} className="border-t border-ink/10">
                <td className="px-4 py-2 font-mono">{it.external_id}</td>
                <td className="px-4 py-2 font-mono text-[0.72rem] text-ink-dim">
                  {shortHash(showBothHashes ? it.hash_a : it[hashField], 16)}
                </td>
                {showBothHashes && (
                  <td className="px-4 py-2 font-mono text-[0.72rem] text-ink-dim">
                    {shortHash(it.hash_b, 16)}
                  </td>
                )}
                <td className="px-4 py-2 text-right">
                  <Link
                    to={`/snapshots/${snapshotId}/events/${encodeURIComponent(it.external_id)}`}
                    className="text-accent hover:underline text-xs"
                  >
                    inspecionar →
                  </Link>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {items.length > 200 && (
          <div className="px-4 py-3 text-xs text-ink-dim border-t border-ink/10">
            mostrando primeiros 200 de {items.length}
          </div>
        )}
      </div>
    </section>
  );
}
