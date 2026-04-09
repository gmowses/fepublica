import { useQuery } from "@tanstack/react-query";
import { Link, useSearchParams } from "react-router-dom";
import { ChevronLeft, ChevronRight, Search } from "lucide-react";
import { formatNumber } from "../lib/api";

interface Ente {
  id: string;
  nome: string;
  nome_curto?: string;
  esfera: string;
  tipo: string;
  uf?: string;
  tier: number;
}

export function Entes() {
  const [sp, setSp] = useSearchParams();
  const q = sp.get("q") ?? "";
  const esfera = sp.get("esfera") ?? "";
  const uf = sp.get("uf") ?? "";
  const offset = Number(sp.get("offset") ?? "0");
  const pageSize = 100;

  const listQ = useQuery({
    queryKey: ["entes", q, esfera, uf, offset],
    queryFn: () => {
      const params = new URLSearchParams({ limit: String(pageSize), offset: String(offset) });
      if (q) params.set("q", q);
      if (esfera) params.set("esfera", esfera);
      if (uf) params.set("uf", uf);
      return fetch("/api/entes?" + params).then((r) => r.json()) as Promise<{
        total: number;
        entes: Ente[];
      }>;
    },
  });

  const entes = listQ.data?.entes ?? [];
  const total = listQ.data?.total ?? 0;

  const setFilter = (key: string, value: string) => {
    const next = new URLSearchParams(sp);
    if (value) next.set(key, value);
    else next.delete(key);
    next.delete("offset");
    setSp(next);
  };

  const setOffset = (n: number) => {
    const next = new URLSearchParams(sp);
    if (n > 0) next.set("offset", String(n));
    else next.delete("offset");
    setSp(next);
  };

  return (
    <div className="container-app py-6 md:py-10">
      <nav className="text-xs font-mono text-ink-dim mb-3">
        <Link to="/" className="hover:text-accent">
          início
        </Link>{" "}
        / entes
      </nav>
      <h1 className="text-2xl md:text-3xl font-bold tracking-tight">
        Entes públicos brasileiros
      </h1>
      <p className="text-ink-dim mt-2 max-w-3xl">
        União, 3 poderes federais, órgãos de controle, 27 UFs e 5570+
        municípios. Base pra segmentação geográfica dos dados arquivados e pro
        futuro crawler de conformidade LAI.
      </p>

      <div className="card mt-6 flex flex-wrap items-center gap-3">
        <div className="flex items-center gap-2 bg-bg border border-ink/10 rounded-md px-3 py-2 flex-1 min-w-[200px]">
          <Search className="size-4 text-ink-dim" />
          <input
            className="bg-transparent outline-none text-sm flex-1 placeholder:text-ink-dim"
            placeholder="buscar por nome…"
            defaultValue={q}
            onKeyDown={(e) => {
              if (e.key === "Enter") setFilter("q", (e.target as HTMLInputElement).value);
            }}
          />
        </div>
        <select
          className="bg-bg-card border border-ink/10 rounded-md px-2 py-2 text-sm"
          value={esfera}
          onChange={(e) => setFilter("esfera", e.target.value)}
        >
          <option value="">todas esferas</option>
          <option value="federal">federal</option>
          <option value="estadual">estadual</option>
          <option value="municipal">municipal</option>
          <option value="distrital">distrital</option>
        </select>
        <input
          className="bg-bg-card border border-ink/10 rounded-md px-2 py-2 text-sm w-20 text-center uppercase"
          placeholder="UF"
          maxLength={2}
          defaultValue={uf}
          onKeyDown={(e) => {
            if (e.key === "Enter") setFilter("uf", (e.target as HTMLInputElement).value.toUpperCase());
          }}
        />
        <span className="text-xs text-ink-dim ml-auto">
          {listQ.data ? formatNumber(total) + " resultados" : "carregando…"}
        </span>
      </div>

      <div className="card mt-4 overflow-x-auto p-0">
        <table className="w-full text-sm">
          <thead className="text-xs uppercase tracking-wider text-ink-dim">
            <tr>
              <th className="text-left px-4 py-2">id</th>
              <th className="text-left px-4 py-2">nome</th>
              <th className="text-left px-4 py-2">esfera</th>
              <th className="text-left px-4 py-2">tipo</th>
              <th className="text-left px-4 py-2">UF</th>
              <th className="text-left px-4 py-2">tier</th>
              <th className="text-left px-4 py-2"></th>
            </tr>
          </thead>
          <tbody>
            {entes.map((e) => (
              <tr key={e.id} className="border-t border-ink/10">
                <td className="px-4 py-2 font-mono text-xs">{e.id}</td>
                <td className="px-4 py-2">{e.nome}</td>
                <td className="px-4 py-2">
                  <span className="chip">{e.esfera}</span>
                </td>
                <td className="px-4 py-2 font-mono text-xs text-ink-dim">{e.tipo}</td>
                <td className="px-4 py-2 font-mono">{e.uf ?? ""}</td>
                <td className="px-4 py-2 tabular-nums">{e.tier}</td>
                <td className="px-4 py-2 text-right">
                  <Link
                    to={`/entes/${encodeURIComponent(e.id)}`}
                    className="text-accent hover:underline text-xs"
                  >
                    ver →
                  </Link>
                </td>
              </tr>
            ))}
            {entes.length === 0 && !listQ.isLoading && (
              <tr>
                <td colSpan={7} className="px-4 py-8 text-center text-ink-dim">
                  nenhum ente encontrado
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {total > pageSize && (
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
            disabled={offset + pageSize >= total}
            onClick={() => setOffset(offset + pageSize)}
          >
            próxima <ChevronRight className="size-4" />
          </button>
          <div className="text-ink-dim ml-auto font-mono text-xs">
            {formatNumber(offset + 1)}–{formatNumber(Math.min(offset + pageSize, total))} de {formatNumber(total)}
          </div>
        </div>
      )}
    </div>
  );
}
