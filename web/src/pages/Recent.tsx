import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { useSearchParams } from "react-router-dom";
import {
  ArrowLeftRight,
  Info,
  Plus,
  Minus,
  PenLine,
  Filter,
} from "lucide-react";
import { formatDate, formatNumber } from "../lib/api";

interface DiffRun {
  id: number;
  source_id: string;
  snapshot_a_id: number;
  snapshot_b_id: number;
  added_count: number;
  removed_count: number;
  modified_count: number;
  ran_at: string;
  duration_ms: number;
}

interface ChangeEvent {
  id: number;
  diff_run_id: number;
  source_id: string;
  external_id: string;
  change_type: "added" | "removed" | "modified";
  detected_at: string;
  severity: "info" | "warn" | "alert";
}

// Recent now reads pre-computed drift_runs and change_events from the
// Observatório M1 backend, instead of computing client-side diffs.
export function Recent() {
  const [searchParams, setSearchParams] = useSearchParams();
  const source = searchParams.get("source") || "";
  const severity = searchParams.get("severity") || "";

  const runsQ = useQuery({
    queryKey: ["diff-runs", source],
    queryFn: () => {
      const p = new URLSearchParams({ limit: "20" });
      if (source) p.set("source", source);
      return fetch(`/api/diff-runs?${p}`).then((r) => r.json()) as Promise<{
        diff_runs: DiffRun[];
      }>;
    },
  });

  const eventsQ = useQuery({
    queryKey: ["change-events", source, severity],
    queryFn: () => {
      const p = new URLSearchParams({ limit: "50" });
      if (source) p.set("source", source);
      if (severity) p.set("severity", severity);
      return fetch(`/api/change-events?${p}`).then((r) => r.json()) as Promise<{
        total: number;
        limit: number;
        offset: number;
        change_events: ChangeEvent[];
      }>;
    },
  });

  const runs = runsQ.data?.diff_runs ?? [];
  const events = eventsQ.data?.change_events ?? [];
  const totalEvents = eventsQ.data?.total ?? 0;

  const totalAdded = runs.reduce((a, r) => a + r.added_count, 0);
  const totalRemoved = runs.reduce((a, r) => a + r.removed_count, 0);
  const totalModified = runs.reduce((a, r) => a + r.modified_count, 0);
  const totalChanges = totalAdded + totalRemoved + totalModified;

  const setFilter = (key: string, value: string) => {
    const next = new URLSearchParams(searchParams);
    if (value) next.set(key, value);
    else next.delete(key);
    setSearchParams(next);
  };

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
        Feed cronológico de alterações detectadas entre snapshots consecutivos
        de cada fonte monitorada. Os diffs são persistidos pelo{" "}
        <code className="text-accent">driftd</code> worker do Observatório e
        classificados por severidade antes de serem publicados.
      </p>

      {/* Filter bar */}
      <div className="card mt-6 flex flex-wrap items-center gap-3">
        <Filter className="size-4 text-ink-dim" />
        <label className="text-xs text-ink-dim">fonte</label>
        <select
          className="bg-bg border border-ink/10 rounded-md px-2 py-1 text-sm font-mono"
          value={source}
          onChange={(e) => setFilter("source", e.target.value)}
        >
          <option value="">todas</option>
          <option value="ceis">ceis</option>
          <option value="cnep">cnep</option>
          <option value="pncp-contratos">pncp-contratos</option>
        </select>
        <label className="text-xs text-ink-dim">severidade</label>
        <select
          className="bg-bg border border-ink/10 rounded-md px-2 py-1 text-sm font-mono"
          value={severity}
          onChange={(e) => setFilter("severity", e.target.value)}
        >
          <option value="">todas</option>
          <option value="info">info</option>
          <option value="warn">warn</option>
          <option value="alert">alert</option>
        </select>
      </div>

      {/* Summary */}
      <div className="card mt-6 grid grid-cols-2 md:grid-cols-4 gap-3 items-center">
        <div>
          <div className="stat-k">diff runs</div>
          <div className="stat-v">{formatNumber(runs.length)}</div>
        </div>
        <div>
          <div className="stat-k">eventos filtrados</div>
          <div className="stat-v">{formatNumber(totalEvents)}</div>
        </div>
        <div>
          <div className="stat-k">mudanças totais</div>
          <div className="stat-v text-accent">{formatNumber(totalChanges)}</div>
        </div>
        <div className="text-xs text-ink-dim">
          <div className="flex items-center gap-2">
            <Plus className="size-3 text-ok" /> {formatNumber(totalAdded)} adds
          </div>
          <div className="flex items-center gap-2">
            <Minus className="size-3 text-danger" /> {formatNumber(totalRemoved)}{" "}
            removes
          </div>
          <div className="flex items-center gap-2">
            <PenLine className="size-3 text-accent" /> {formatNumber(totalModified)}{" "}
            mods
          </div>
        </div>
      </div>

      {/* Diff runs timeline */}
      <h2 className="text-lg font-semibold mt-8 mb-3">Últimas execuções</h2>
      {runsQ.isLoading && (
        <div className="text-ink-dim text-sm">carregando…</div>
      )}
      {runs.length === 0 && !runsQ.isLoading && (
        <div className="card text-sm text-ink-dim flex items-center gap-2">
          <Info className="size-4" />
          Ainda não há execuções de diff registradas. O driftd roda
          automaticamente no background a cada 2 minutos.
        </div>
      )}
      <div className="space-y-3">
        {runs.map((r) => (
          <div key={r.id} className="card">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div className="flex items-center gap-3">
                <ArrowLeftRight className="size-4 text-accent" />
                <div>
                  <div className="font-semibold">
                    {r.source_id}{" "}
                    <span className="text-ink-dim font-normal font-mono text-xs">
                      #{r.snapshot_a_id} → #{r.snapshot_b_id}
                    </span>
                  </div>
                  <div className="text-xs text-ink-dim mt-0.5">
                    {formatDate(r.ran_at)} · {r.duration_ms}ms
                  </div>
                </div>
              </div>
              <Link
                to={`/diff/${r.snapshot_a_id}/${r.snapshot_b_id}`}
                className="text-xs text-accent hover:underline"
              >
                ver diff →
              </Link>
            </div>
            <div className="grid grid-cols-3 gap-2 mt-3">
              <DiffStat icon={Plus} label="adicionados" count={r.added_count} color="text-ok" />
              <DiffStat icon={Minus} label="removidos" count={r.removed_count} color="text-danger" />
              <DiffStat icon={PenLine} label="alterados" count={r.modified_count} color="text-accent" />
            </div>
          </div>
        ))}
      </div>

      {/* Change events stream */}
      {events.length > 0 && (
        <>
          <h2 className="text-lg font-semibold mt-10 mb-3">
            Stream de eventos{severity ? ` (${severity})` : ""}
          </h2>
          <div className="card overflow-x-auto p-0">
            <table className="w-full text-sm">
              <thead className="text-xs uppercase tracking-wider text-ink-dim">
                <tr>
                  <th className="text-left px-4 py-2">fonte</th>
                  <th className="text-left px-4 py-2">tipo</th>
                  <th className="text-left px-4 py-2">external_id</th>
                  <th className="text-left px-4 py-2">sev</th>
                  <th className="text-left px-4 py-2">detectado em</th>
                  <th className="text-left px-4 py-2"></th>
                </tr>
              </thead>
              <tbody>
                {events.map((e) => (
                  <tr key={e.id} className="border-t border-ink/10">
                    <td className="px-4 py-2 font-mono">{e.source_id}</td>
                    <td className="px-4 py-2">
                      <ChangeTypeChip type={e.change_type} />
                    </td>
                    <td className="px-4 py-2 font-mono text-xs">
                      {e.external_id}
                    </td>
                    <td className="px-4 py-2">
                      <SeverityChip sev={e.severity} />
                    </td>
                    <td className="px-4 py-2 text-ink-dim text-xs">
                      {formatDate(e.detected_at)}
                    </td>
                    <td className="px-4 py-2 text-right">
                      <Link
                        to={`/snapshots/${e.diff_run_id}/events/${encodeURIComponent(e.external_id)}`}
                        className="text-accent hover:underline text-xs"
                      >
                        inspecionar →
                      </Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}
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

function ChangeTypeChip({ type }: { type: string }) {
  const c =
    type === "added"
      ? "chip-ok"
      : type === "removed"
      ? "chip-danger"
      : "chip-warn";
  return <span className={`chip ${c}`}>{type}</span>;
}

function SeverityChip({ sev }: { sev: string }) {
  const c = sev === "alert" ? "chip-danger" : sev === "warn" ? "chip-warn" : "";
  return <span className={`chip ${c}`}>{sev}</span>;
}
