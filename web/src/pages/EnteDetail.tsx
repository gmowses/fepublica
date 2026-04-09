import { Link, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { ExternalLink } from "lucide-react";
import { formatDate } from "../lib/api";

interface Ente {
  id: string;
  nome: string;
  nome_curto?: string;
  esfera: string;
  tipo: string;
  poder?: string;
  uf?: string;
  ibge_code?: string;
  cnpj?: string;
  populacao?: number;
  domain_hint?: string;
  parent_id?: string;
  tier: number;
  active: boolean;
  created_at: string;
  updated_at: string;
}

export function EnteDetail() {
  const { id } = useParams();
  const enteQ = useQuery({
    queryKey: ["ente", id],
    queryFn: () => fetch(`/api/entes/${encodeURIComponent(id ?? "")}`).then((r) => r.json()) as Promise<Ente>,
    enabled: !!id,
  });

  if (enteQ.isLoading) {
    return <div className="container-app py-16 text-ink-dim">carregando…</div>;
  }
  if (enteQ.isError || !enteQ.data) {
    return (
      <div className="container-app py-16 text-ink-dim">
        ente não encontrado.{" "}
        <Link to="/entes" className="text-accent hover:underline">
          voltar
        </Link>
      </div>
    );
  }
  const e = enteQ.data;

  return (
    <div className="container-app py-6 md:py-10">
      <nav className="text-xs font-mono text-ink-dim mb-3">
        <Link to="/" className="hover:text-accent">
          início
        </Link>{" "}
        /{" "}
        <Link to="/entes" className="hover:text-accent">
          entes
        </Link>{" "}
        / <strong className="text-ink">{e.id}</strong>
      </nav>
      <h1 className="text-2xl md:text-3xl font-bold tracking-tight">{e.nome}</h1>
      <div className="text-ink-dim font-mono text-sm mt-1">
        id: {e.id} · esfera: {e.esfera} · tipo: {e.tipo}
        {e.uf ? " · UF: " + e.uf : ""}
        {" · tier: "} {e.tier}
      </div>

      {e.domain_hint && (
        <a
          className="btn mt-4"
          href={e.domain_hint}
          target="_blank"
          rel="noopener"
        >
          <ExternalLink className="size-4" /> Portal oficial
        </a>
      )}

      <div className="grid grid-cols-1 md:grid-cols-2 gap-3 mt-6">
        <div className="card">
          <div className="stat-k">Metadados</div>
          <dl className="mt-2 text-sm space-y-1">
            <div className="flex justify-between">
              <dt className="text-ink-dim">poder</dt>
              <dd className="font-mono">{e.poder ?? "—"}</dd>
            </div>
            <div className="flex justify-between">
              <dt className="text-ink-dim">IBGE</dt>
              <dd className="font-mono">{e.ibge_code ?? "—"}</dd>
            </div>
            <div className="flex justify-between">
              <dt className="text-ink-dim">CNPJ</dt>
              <dd className="font-mono text-xs">{e.cnpj ?? "—"}</dd>
            </div>
            <div className="flex justify-between">
              <dt className="text-ink-dim">população</dt>
              <dd className="font-mono">
                {e.populacao ? e.populacao.toLocaleString("pt-BR") : "—"}
              </dd>
            </div>
            <div className="flex justify-between">
              <dt className="text-ink-dim">parent</dt>
              <dd className="font-mono text-xs">
                {e.parent_id ? (
                  <Link className="text-accent hover:underline" to={`/entes/${e.parent_id}`}>
                    {e.parent_id}
                  </Link>
                ) : (
                  "—"
                )}
              </dd>
            </div>
          </dl>
        </div>
        <div className="card">
          <div className="stat-k">Auditoria</div>
          <dl className="mt-2 text-sm space-y-1">
            <div className="flex justify-between">
              <dt className="text-ink-dim">criado em</dt>
              <dd>{formatDate(e.created_at)}</dd>
            </div>
            <div className="flex justify-between">
              <dt className="text-ink-dim">atualizado</dt>
              <dd>{formatDate(e.updated_at)}</dd>
            </div>
            <div className="flex justify-between">
              <dt className="text-ink-dim">ativo</dt>
              <dd className={e.active ? "text-ok" : "text-danger"}>
                {e.active ? "sim" : "não"}
              </dd>
            </div>
          </dl>
        </div>
      </div>

      <div className="card mt-6">
        <div className="stat-k">Atividade</div>
        <p className="text-ink-dim text-sm mt-2">
          Feed de mudanças por ente é habilitado em v0.6.0 quando o detector de
          drift popular o campo ente_id via heurística. Por enquanto, consulte{" "}
          <Link to="/recent" className="text-accent hover:underline">
            /recent
          </Link>{" "}
          para ver o estado atual do arquivo.
        </p>
      </div>
    </div>
  );
}
