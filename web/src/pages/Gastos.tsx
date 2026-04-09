import { useQuery } from "@tanstack/react-query";
import { Link, useSearchParams } from "react-router-dom";
import { Search, TrendingUp, AlertTriangle, Building2, Users, Wallet } from "lucide-react";
import { useState } from "react";

interface Stats {
  total_contratos: number;
  valor_total_global: number;
  orgaos_unicos: number;
  fornecedores_unicos: number;
}

interface TopRow {
  key: string;
  nome: string;
  count: number;
  total_valor: number;
}

interface Contrato {
  id: number;
  external_id: string;
  orgao_cnpj?: string;
  orgao_razao_social?: string;
  uf?: string;
  fornecedor_ni?: string;
  fornecedor_nome?: string;
  valor_global?: number;
  data_assinatura?: string;
  objeto_contrato?: string;
  fornecedor_sancionado: boolean;
}

function formatBRL(n?: number): string {
  if (n == null) return "—";
  return n.toLocaleString("pt-BR", { style: "currency", currency: "BRL", maximumFractionDigits: 0 });
}

function formatCNPJ(s?: string): string {
  if (!s) return "";
  const d = s.replace(/\D/g, "");
  if (d.length === 14) {
    return d.replace(/^(\d{2})(\d{3})(\d{3})(\d{4})(\d{2})$/, "$1.$2.$3/$4-$5");
  }
  if (d.length === 11) {
    return d.replace(/^(\d{3})(\d{3})(\d{3})(\d{2})$/, "$1.$2.$3-$4");
  }
  return s;
}

export function Gastos() {
  const [sp, setSp] = useSearchParams();
  const [searchInput, setSearchInput] = useState(sp.get("q") ?? "");
  const search = sp.get("q") ?? "";

  const statsQ = useQuery({
    queryKey: ["gastos-stats"],
    queryFn: () => fetch("/api/gastos/stats").then((r) => r.json()) as Promise<Stats>,
  });
  const topForn = useQuery({
    queryKey: ["gastos-top-fornecedores"],
    queryFn: () =>
      fetch("/api/gastos/top-fornecedores?limit=10").then((r) => r.json()) as Promise<{ top: TopRow[] }>,
  });
  const topOrgaos = useQuery({
    queryKey: ["gastos-top-orgaos"],
    queryFn: () =>
      fetch("/api/gastos/top-orgaos?limit=10").then((r) => r.json()) as Promise<{ top: TopRow[] }>,
  });
  const listQ = useQuery({
    queryKey: ["gastos-contratos", search],
    queryFn: () => {
      const p = new URLSearchParams({ limit: "20", order: "valor_desc" });
      if (search) p.set("q", search);
      return fetch("/api/gastos/contratos?" + p).then((r) => r.json()) as Promise<{
        total: number;
        contratos: Contrato[];
      }>;
    },
  });

  const submitSearch = (e: React.FormEvent) => {
    e.preventDefault();
    const next = new URLSearchParams(sp);
    if (searchInput) next.set("q", searchInput);
    else next.delete("q");
    setSp(next);
  };

  const stats = statsQ.data;
  const contratos = listQ.data?.contratos ?? [];
  const totalContratos = listQ.data?.total ?? 0;

  return (
    <div className="container-app py-6 md:py-10">
      <nav className="text-xs font-mono text-ink-dim mb-3">
        <Link to="/" className="hover:text-accent">início</Link> / gastos
      </nav>

      {/* Hero */}
      <div className="text-[0.72rem] font-mono uppercase tracking-widest text-accent mb-3">
        Rastreador de gastos públicos
      </div>
      <h1 className="text-3xl md:text-5xl font-bold leading-tight tracking-tight max-w-4xl">
        Quanto o governo brasileiro contratou,<br className="hidden sm:block" /> com quem, e para quê.
      </h1>
      <p className="mt-4 text-lg text-ink-dim max-w-3xl">
        Contratos públicos federais, estaduais e municipais coletados do Portal Nacional de
        Contratações Públicas (PNCP). Cruzados automaticamente com os cadastros de empresas
        sancionadas (CEIS e CNEP) — quando um fornecedor está em uma lista de impedidos, você vê.
      </p>

      {/* Search */}
      <form onSubmit={submitSearch} className="mt-6 flex gap-2 max-w-3xl">
        <div className="flex-1 flex items-center gap-2 bg-bg-card border border-ink/10 rounded-md px-4 py-3">
          <Search className="size-5 text-ink-dim" />
          <input
            className="bg-transparent outline-none flex-1 text-base placeholder:text-ink-dim"
            placeholder="busque por CNPJ, nome da empresa, órgão público ou objeto do contrato…"
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
          />
        </div>
        <button type="submit" className="btn btn-primary">Buscar</button>
      </form>

      {/* Stats */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mt-8">
        <StatCard
          k="Valor total contratado"
          v={formatBRL(stats?.valor_total_global)}
          Icon={Wallet}
        />
        <StatCard
          k="Contratos arquivados"
          v={(stats?.total_contratos ?? 0).toLocaleString("pt-BR")}
          Icon={TrendingUp}
        />
        <StatCard
          k="Órgãos contratantes"
          v={(stats?.orgaos_unicos ?? 0).toLocaleString("pt-BR")}
          Icon={Building2}
        />
        <StatCard
          k="Fornecedores únicos"
          v={(stats?.fornecedores_unicos ?? 0).toLocaleString("pt-BR")}
          Icon={Users}
        />
      </div>

      {/* Top lists */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-3 mt-8">
        <div className="card">
          <h2 className="text-lg font-semibold mb-3">Top 10 fornecedores por valor</h2>
          <TopList rows={topForn.data?.top ?? []} linkPrefix="/gastos/fornecedores/" />
        </div>
        <div className="card">
          <h2 className="text-lg font-semibold mb-3">Top 10 órgãos por valor</h2>
          <TopList rows={topOrgaos.data?.top ?? []} linkPrefix={null} />
        </div>
      </div>

      {/* Contratos list */}
      <h2 className="text-xl font-semibold mt-10 mb-3">
        {search ? `Resultados para "${search}"` : "Maiores contratos arquivados"}
        <span className="text-ink-dim text-sm font-normal ml-2">
          ({totalContratos.toLocaleString("pt-BR")} total)
        </span>
      </h2>

      <div className="space-y-3">
        {contratos.map((c) => (
          <ContratoCard key={c.id} c={c} />
        ))}
        {contratos.length === 0 && !listQ.isLoading && (
          <div className="card text-center text-ink-dim py-8">
            Nenhum contrato encontrado {search ? `para "${search}"` : ""}.
          </div>
        )}
      </div>
    </div>
  );
}

function StatCard({ k, v, Icon }: { k: string; v: string; Icon: any }) {
  return (
    <div className="card">
      <div className="flex items-center justify-between">
        <div className="stat-k">{k}</div>
        <Icon className="size-4 text-accent" />
      </div>
      <div className="stat-v">{v}</div>
    </div>
  );
}

function TopList({ rows, linkPrefix }: { rows: TopRow[]; linkPrefix: string | null }) {
  if (rows.length === 0) {
    return <div className="text-ink-dim text-sm">sem dados ainda</div>;
  }
  const max = Math.max(...rows.map((r) => r.total_valor));
  return (
    <ol className="space-y-2">
      {rows.map((r, i) => {
        const pct = max > 0 ? (r.total_valor / max) * 100 : 0;
        const content = (
          <div className="flex items-center gap-3 py-1">
            <span className="text-ink-dim text-xs font-mono w-6 text-right">{i + 1}</span>
            <div className="flex-1 min-w-0">
              <div className="text-sm truncate">{r.nome || r.key}</div>
              <div className="h-1.5 bg-bg-soft rounded-full mt-1 overflow-hidden">
                <div className="h-full bg-accent rounded-full" style={{ width: `${pct}%` }} />
              </div>
              <div className="flex justify-between text-xs text-ink-dim mt-0.5">
                <span>{formatCNPJ(r.key)}</span>
                <span>
                  {r.count} contratos · {formatBRL(r.total_valor)}
                </span>
              </div>
            </div>
          </div>
        );
        if (linkPrefix && r.key) {
          return (
            <li key={r.key}>
              <Link to={linkPrefix + encodeURIComponent(r.key)} className="block hover:text-accent">
                {content}
              </Link>
            </li>
          );
        }
        return <li key={r.key + i}>{content}</li>;
      })}
    </ol>
  );
}

function ContratoCard({ c }: { c: Contrato }) {
  return (
    <div className="card">
      {c.fornecedor_sancionado && (
        <div className="flex items-center gap-2 text-danger text-xs font-mono mb-2">
          <AlertTriangle className="size-4" />
          FORNECEDOR PRESENTE EM LISTA DE SANÇÕES (CEIS/CNEP)
        </div>
      )}
      <div className="flex flex-wrap gap-3 items-start justify-between">
        <div className="min-w-0 flex-1">
          <div className="text-xs text-ink-dim font-mono">
            {c.data_assinatura} {c.uf ? "· " + c.uf : ""}
          </div>
          <div className="mt-1 font-semibold line-clamp-2">
            {c.objeto_contrato || "Sem objeto informado"}
          </div>
          <div className="mt-2 text-sm">
            <span className="text-ink-dim">De </span>
            <span className="font-medium">{c.orgao_razao_social || c.orgao_cnpj || "—"}</span>
            <span className="text-ink-dim"> para </span>
            {c.fornecedor_ni ? (
              <Link
                to={`/gastos/fornecedores/${encodeURIComponent(c.fornecedor_ni)}`}
                className="font-medium hover:text-accent"
              >
                {c.fornecedor_nome || formatCNPJ(c.fornecedor_ni)}
              </Link>
            ) : (
              <span className="font-medium">{c.fornecedor_nome || "—"}</span>
            )}
          </div>
        </div>
        <div className="text-right shrink-0">
          <div className="text-2xl font-bold text-accent">{formatBRL(c.valor_global)}</div>
          <div className="text-xs text-ink-dim font-mono">contrato</div>
        </div>
      </div>
    </div>
  );
}
