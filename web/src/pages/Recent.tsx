import { useMemo } from "react";
import { Link } from "react-router-dom";
import { useQuery, useQueries } from "@tanstack/react-query";
import { api, formatDate, formatNumber } from "../lib/api";
import { ArrowLeftRight, Info, Plus, Minus, PenLine } from "lucide-react";

// The Recent page runs diffs between consecutive snapshots of each source and
// aggregates the results into a cross-source feed. This is a client-side
// approximation of the future automatic drift detector.
export function Recent() {
  const snapsQ = useQuery({
    queryKey: ["snapshots", { limit: 200 }],
    queryFn: () => api.snapshots({ limit: 200 }),
  });

  const pairs = useMemo(() => {
    const bySource = new Map<string, number[]>();
    for (const s of snapsQ.data?.snapshots ?? []) {
      if (!bySource.has(s.source_id)) bySource.set(s.source_id, []);
      bySource.get(s.source_id)!.push(s.id);
    }
    const out: Array<{ source: string; a: number; b: number }> = [];
    for (const [source, ids] of bySource) {
      ids.sort((x, y) => x - y);
      for (let i = 1; i < ids.length; i++) {
        out.push({ source, a: ids[i - 1], b: ids[i] });
      }
    }
    return out.slice(-10).reverse();
  }, [snapsQ.data]);

  const diffQueries = useQueries({
    queries: pairs.map((p) => ({
      queryKey: ["diff", p.a, p.b],
      queryFn: () => api.diff(p.a, p.b),
      staleTime: 60_000,
    })),
  });

  const totalChanges = diffQueries.reduce((acc, q) => {
    if (q.data) {
      return (
        acc + q.data.summary.added + q.data.summary.removed + q.data.summary.changed
      );
    }
    return acc;
  }, 0);

  return (
    <div className="container-app py-6 md:py-10">
      <nav className="text-xs font-mono text-ink-dim mb-3">
        <Link to="/" className="hover:text-accent">
          início
        </Link>{" "}
        / recentes
      </nav>
      <h1 className="text-2xl md:text-3xl font-bold tracking-tight">
        Mudanças recentes
      </h1>
      <p className="text-ink-dim mt-2 max-w-3xl">
        Diffs entre snapshots consecutivos de cada fonte monitorada. Cada
        entrada representa o delta entre uma coleta e a imediatamente anterior.
        Este é um feed cronológico de "o que mudou no arquivo público".
      </p>

      {pairs.length === 0 && (
        <div className="card mt-6 text-sm text-ink-dim flex items-center gap-2">
          <Info className="size-4" />
          Ainda não há snapshots suficientes por fonte para gerar um diff.
          Precisamos de pelo menos 2 coletas consecutivas.
        </div>
      )}

      {pairs.length > 0 && (
        <div className="card mt-6 mb-6 text-sm">
          <span className="text-ink-dim">últimos {pairs.length} diffs · </span>
          <span className="font-semibold text-accent">
            {formatNumber(totalChanges)} mudanças totais
          </span>
        </div>
      )}

      <div className="space-y-3">
        {pairs.map((p, i) => {
          const q = diffQueries[i];
          return (
            <div key={`${p.a}-${p.b}`} className="card">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div className="flex items-center gap-3">
                  <ArrowLeftRight className="size-4 text-accent" />
                  <div>
                    <div className="font-semibold">
                      {p.source}{" "}
                      <span className="text-ink-dim font-normal font-mono text-xs">
                        #{p.a} → #{p.b}
                      </span>
                    </div>
                    {q.data && (
                      <div className="text-xs text-ink-dim mt-0.5">
                        {formatDate(q.data.snapshot_a.collected_at)} →{" "}
                        {formatDate(q.data.snapshot_b.collected_at)}
                      </div>
                    )}
                  </div>
                </div>
                <Link
                  to={`/diff/${p.a}/${p.b}`}
                  className="text-xs text-accent hover:underline"
                >
                  ver diff completo →
                </Link>
              </div>

              {q.isLoading && (
                <div className="text-xs text-ink-dim mt-3">calculando…</div>
              )}
              {q.data && (
                <div className="grid grid-cols-3 gap-2 mt-3">
                  <DiffStat
                    icon={Plus}
                    label="adicionados"
                    count={q.data.summary.added}
                    color="text-ok"
                  />
                  <DiffStat
                    icon={Minus}
                    label="removidos"
                    count={q.data.summary.removed}
                    color="text-danger"
                  />
                  <DiffStat
                    icon={PenLine}
                    label="alterados"
                    count={q.data.summary.changed}
                    color="text-accent"
                  />
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}

function DiffStat({
  icon: Icon,
  label,
  count,
  color,
}: {
  icon: any;
  label: string;
  count: number;
  color: string;
}) {
  return (
    <div className="bg-bg-soft border border-ink/10 rounded-md px-3 py-2">
      <div className="flex items-center justify-between">
        <span className="text-xs text-ink-dim">{label}</span>
        <Icon className={`size-3 ${color}`} />
      </div>
      <div className={`text-lg font-semibold tabular-nums ${color}`}>
        {formatNumber(count)}
      </div>
    </div>
  );
}
