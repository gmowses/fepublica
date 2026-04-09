import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { BrazilMap } from "../components/BrazilMap";
import {
  Activity,
  AlertTriangle,
  Archive,
  Database,
  FileSearch,
  MapPin,
} from "lucide-react";
import { formatNumber } from "../lib/api";

interface Stats {
  total_entes: number;
  active_sources: number;
  total_snapshots: number;
  total_events: number;
  total_diff_runs: number;
  total_change_events: number;
  alert_count: number;
  warn_count: number;
}

export function Observatorio() {
  const statsQ = useQuery({
    queryKey: ["observatorio-stats"],
    queryFn: () => fetch("/api/observatorio/stats").then((r) => r.json()) as Promise<Stats>,
  });
  const entesByUFQ = useQuery({
    queryKey: ["entes-by-uf"],
    queryFn: () =>
      fetch("/api/observatorio/entes-by-uf").then((r) => r.json()) as Promise<{
        by_uf: Record<string, number>;
      }>,
  });
  const changesByUFQ = useQuery({
    queryKey: ["changes-by-uf"],
    queryFn: () =>
      fetch("/api/observatorio/changes-by-uf").then((r) => r.json()) as Promise<{
        by_uf: Record<string, number>;
      }>,
  });

  const stats = statsQ.data;
  const entesByUF = entesByUFQ.data?.by_uf ?? {};
  const changesByUF = changesByUFQ.data?.by_uf ?? {};

  // Use change_events heat when present, otherwise fall back to ente coverage
  // so the map is never all-gray.
  const hasChangeData = Object.values(changesByUF).some((n) => n > 0);
  const heatSource = hasChangeData ? changesByUF : entesByUF;
  const heatLabel = hasChangeData
    ? "mudanças detectadas por UF"
    : "cobertura de entes por UF";

  return (
    <div className="container-app py-6 md:py-10">
      <nav className="text-xs font-mono text-ink-dim mb-3">
        <Link to="/" className="hover:text-accent">
          início
        </Link>{" "}
        / observatório
      </nav>
      <h1 className="text-3xl md:text-4xl font-bold tracking-tight">
        Observatório de Transparência
      </h1>
      <p className="text-ink-dim mt-2 max-w-3xl">
        Visão consolidada do estado atual do arquivo de dados públicos
        brasileiros. Drift detectado, entes monitorados, atividade geográfica e
        severidade acumulada.
      </p>

      {/* Big stats */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mt-6">
        <StatCard
          k="Entes catalogados"
          v={formatNumber(stats?.total_entes)}
          sub="federal + UFs + municípios"
          Icon={MapPin}
        />
        <StatCard
          k="Registros arquivados"
          v={formatNumber(stats?.total_events)}
          sub={`em ${formatNumber(stats?.total_snapshots)} snapshots`}
          Icon={Archive}
        />
        <StatCard
          k="Mudanças detectadas"
          v={formatNumber(stats?.total_change_events)}
          sub={`${formatNumber(stats?.total_diff_runs)} diff runs`}
          Icon={FileSearch}
        />
        <StatCard
          k="Severidade alta"
          v={formatNumber((stats?.alert_count ?? 0) + (stats?.warn_count ?? 0))}
          sub={`${formatNumber(stats?.alert_count)} alerts, ${formatNumber(stats?.warn_count)} warns`}
          Icon={AlertTriangle}
          highlight={(stats?.alert_count ?? 0) > 0}
        />
      </div>

      {/* Map */}
      <div className="card mt-6">
        <div className="flex items-center justify-between mb-2">
          <div>
            <div className="stat-k">Mapa por UF</div>
            <div className="text-lg font-semibold mt-1">{heatLabel}</div>
          </div>
          <Activity className="size-5 text-accent" />
        </div>
        <BrazilMap heatByUF={heatSource} />
        <p className="text-xs text-ink-dim mt-2">
          {hasChangeData
            ? "Tom mais quente = mais mudanças detectadas nos snapshots recentes."
            : "Nenhuma mudança detectada ainda. O mapa mostra quantos entes estão catalogados por UF (cobertura), que é a base para futuras detecções."}
        </p>
      </div>

      {/* Links */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-3 mt-6">
        <Link to="/entes" className="card hover:border-accent transition">
          <div className="stat-k">Explorar entes</div>
          <div className="font-semibold mt-1 flex items-center gap-2">
            <Database className="size-4" /> {formatNumber(stats?.total_entes)}{" "}
            catalogados
          </div>
          <div className="text-sm text-ink-dim mt-2">
            Busca paginada por nome, filtros por esfera, UF e tier.
          </div>
        </Link>
        <Link to="/recent" className="card hover:border-accent transition">
          <div className="stat-k">Mudanças recentes</div>
          <div className="font-semibold mt-1 flex items-center gap-2">
            <FileSearch className="size-4" /> Feed de drift
          </div>
          <div className="text-sm text-ink-dim mt-2">
            Timeline cronológica de diff_runs e change_events persistidos.
          </div>
        </Link>
        <a
          href="/api/feeds/all/atom"
          className="card hover:border-accent transition"
        >
          <div className="stat-k">Feed Atom/JSON</div>
          <div className="font-semibold mt-1 flex items-center gap-2">
            <Activity className="size-4" /> Assinar
          </div>
          <div className="text-sm text-ink-dim mt-2">
            Distribuição passiva via RSS, JSON Feed, Telegram e Mastodon.
          </div>
        </a>
      </div>
    </div>
  );
}

function StatCard({
  k,
  v,
  sub,
  Icon,
  highlight,
}: {
  k: string;
  v: string;
  sub?: string;
  Icon: any;
  highlight?: boolean;
}) {
  return (
    <div className={`card ${highlight ? "border-danger/40" : ""}`}>
      <div className="flex items-center justify-between">
        <div className="stat-k">{k}</div>
        <Icon className={`size-4 ${highlight ? "text-danger" : "text-accent"}`} />
      </div>
      <div className={`stat-v ${highlight ? "text-danger" : ""}`}>{v}</div>
      {sub && <div className="text-xs text-ink-dim mt-1">{sub}</div>}
    </div>
  );
}
