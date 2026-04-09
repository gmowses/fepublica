import { Link, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { api } from "../lib/api";
import { Download, FileCode, Terminal } from "lucide-react";

export function EventDetail() {
  const { id, externalId } = useParams();
  const snapId = Number(id);
  const decodedExt = externalId ? decodeURIComponent(externalId) : "";

  const eventQ = useQuery({
    queryKey: ["event", snapId, decodedExt],
    queryFn: () => api.event(snapId, decodedExt),
    enabled: !!snapId && !!decodedExt,
  });
  const snapQ = useQuery({
    queryKey: ["snapshot", snapId],
    queryFn: () => api.snapshot(snapId),
    enabled: !!snapId,
  });

  if (eventQ.isLoading) {
    return <div className="container-app py-16 text-ink-dim">carregando…</div>;
  }
  if (eventQ.isError || !eventQ.data) {
    return (
      <div className="container-app py-16 text-ink-dim">
        evento não encontrado.{" "}
        <Link to="/" className="text-accent hover:underline">
          voltar
        </Link>
      </div>
    );
  }
  const ev = eventQ.data;
  const proofURL = `/api/snapshots/${snapId}/events/${encodeURIComponent(decodedExt)}/proof`;

  return (
    <div className="container-app py-6 md:py-10">
      <nav className="text-xs font-mono text-ink-dim mb-3">
        <Link to="/" className="hover:text-accent">
          início
        </Link>{" "}
        /{" "}
        <Link to={`/snapshots/${snapId}`} className="hover:text-accent">
          snapshot #{snapId}
        </Link>{" "}
        / events /{" "}
        <strong className="text-ink break-all">{decodedExt}</strong>
      </nav>

      <h1 className="text-2xl md:text-3xl font-bold tracking-tight break-all">
        {decodedExt}
      </h1>
      {snapQ.data && (
        <div className="text-ink-dim font-mono text-sm mt-1">
          evento individual em {snapQ.data.source_id} · snapshot #{snapId}
        </div>
      )}

      <div className="grid grid-cols-1 md:grid-cols-2 gap-3 mt-6">
        <div className="card">
          <div className="stat-k">external_id</div>
          <div className="font-mono text-sm mt-1 break-all">
            {ev.external_id}
          </div>
        </div>
        <div className="card">
          <div className="stat-k">content hash (sha256)</div>
          <div className="font-mono text-[0.72rem] mt-1 break-all text-ink-dim">
            {ev.content_hash}
          </div>
        </div>
      </div>

      <div className="flex flex-wrap gap-3 mt-6">
        <a
          className="btn btn-primary"
          href={proofURL}
          download={`proof-${snapId}-${decodedExt.replace(/[^a-zA-Z0-9_-]/g, "_")}.json`}
        >
          <Download className="size-4" /> Baixar prova (proof.json)
        </a>
        <a className="btn" href={proofURL} target="_blank" rel="noopener">
          <FileCode className="size-4" /> Ver prova inline
        </a>
      </div>

      <h2 className="text-lg font-semibold mt-8 mb-2">Conteúdo canônico</h2>
      <p className="text-sm text-ink-dim mb-2">
        Forma canônica do registro (JSON ordenado, sem whitespace extra) que é
        hasheada com SHA-256 para compor a árvore de Merkle do snapshot.
        Qualquer edição deste registro na fonte original produz um hash diferente.
      </p>
      <pre className="bg-bg-soft border border-ink/10 rounded-md p-4 overflow-auto max-h-[520px] text-[0.78rem] leading-relaxed font-mono whitespace-pre-wrap break-words">
        {JSON.stringify(ev.canonical_json, null, 2)}
      </pre>

      <h2 className="text-lg font-semibold mt-8 mb-2">Como verificar</h2>
      <pre className="bg-bg-soft border border-ink/10 rounded-md p-4 overflow-auto text-[0.78rem] font-mono">
        <Terminal className="size-4 inline text-accent mr-2" />
        <span className="text-ink-dim"># 1. baixar a prova</span>
        {"\n"}curl -s "{proofURL}" {">"} proof.json{"\n\n"}
        <span className="text-ink-dim"># 2. verificar offline</span>
        {"\n"}./fepublica-verify proof.json{"\n"}
      </pre>
    </div>
  );
}
