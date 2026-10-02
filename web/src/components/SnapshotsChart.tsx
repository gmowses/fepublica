import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  CartesianGrid,
  Cell,
} from "recharts";
import type { Snapshot } from "../lib/api";

const sourceColors: Record<string, string> = {
  ceis: "rgb(var(--color-accent))",
  cnep: "rgb(var(--color-ok))",
  "pncp-contratos": "rgb(var(--color-danger))",
};

export function SnapshotsChart({ snapshots }: { snapshots: Snapshot[] }) {
  const data = [...snapshots]
    .slice(0, 40)
    .reverse()
    .map((s) => ({
      id: `#${s.id}`,
      source: s.source_id,
      count: s.record_count,
      collected: new Date(s.collected_at).toLocaleDateString("pt-BR"),
    }));

  if (data.length === 0) {
    return (
      <div className="h-56 flex items-center justify-center text-ink-dim text-sm">
        sem dados ainda
      </div>
    );
  }

  return (
    <div className="h-56">
      <ResponsiveContainer width="100%" height="100%">
        <BarChart
          data={data}
          margin={{ top: 10, right: 10, left: -10, bottom: 0 }}
        >
          <CartesianGrid
            strokeDasharray="3 3"
            stroke="rgb(var(--color-border))"
          />
          <XAxis
            dataKey="id"
            tick={{ fontSize: 11, fill: "rgb(var(--color-ink-dim))" }}
            stroke="rgb(var(--color-ink-dim))"
          />
          <YAxis
            tick={{ fontSize: 11, fill: "rgb(var(--color-ink-dim))" }}
            stroke="rgb(var(--color-ink-dim))"
            tickFormatter={(v) =>
              v >= 1000 ? `${(v / 1000).toFixed(0)}k` : String(v)
            }
          />
          <Tooltip
            contentStyle={{
              background: "rgb(var(--color-bg-card))",
              border: "1px solid rgb(var(--color-border))",
              borderRadius: 6,
              fontSize: 12,
              color: "rgb(var(--color-ink))",
            }}
            labelStyle={{ color: "rgb(var(--color-accent))" }}
            formatter={(v: number, _name: string, props: any) => [
              v.toLocaleString("pt-BR") + " registros",
              props.payload.source + " · " + props.payload.collected,
            ]}
          />
          <Bar dataKey="count" radius={[4, 4, 0, 0]}>
            {data.map((d, i) => (
              <Cell
                key={i}
                fill={sourceColors[d.source] ?? "rgb(var(--color-accent))"}
              />
            ))}
          </Bar>
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}
