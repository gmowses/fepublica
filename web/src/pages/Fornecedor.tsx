import { Link, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { AlertTriangle, Wallet, FileText } from "lucide-react";

interface Contrato {
  id: number;
  external_id: string;
  orgao_razao_social?: string;
  orgao_cnpj?: string;
  uf?: string;
  valor_global?: number;
  data_assinatura?: string;
  objeto_contrato?: string;
}

interface FornecedorData {
  fornecedor: {
    ni: string;
    nome: string;
    total_contratos: number;
    valor_total: number;
    sancionado: boolean;
  };
  contratos: Contrato[];
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
  return s;
}

export function Fornecedor() {
  const { ni } = useParams();
  const q = useQuery({
    queryKey: ["fornecedor", ni],
    queryFn: () => fetch(`/api/gastos/fornecedores/${encodeURIComponent(ni ?? "")}`).then((r) => r.json()) as Promise<FornecedorData>,
    enabled: !!ni,
  });

  if (q.isLoading) return <div className="container-app py-16 text-ink-dim">carregando…</div>;
  if (q.isError || !q.data)
    return (
      <div className="container-app py-16 text-ink-dim">
        fornecedor não encontrado. <Link to="/gastos" className="text-accent hover:underline">voltar</Link>
      </div>
    );

  const { fornecedor, contratos } = q.data;

  return (
    <div className="container-app py-6 md:py-10">
      <nav className="text-xs font-mono text-ink-dim mb-3">
        <Link to="/" className="hover:text-accent">início</Link> /{" "}
        <Link to="/gastos" className="hover:text-accent">gastos</Link> / fornecedores /{" "}
        <strong className="text-ink">{formatCNPJ(fornecedor.ni)}</strong>
      </nav>

      {fornecedor.sancionado && (
        <div className="card border-danger/40 bg-danger/10 mb-4">
          <div className="flex items-center gap-3">
            <AlertTriangle className="size-5 text-danger" />
            <div>
              <div className="font-semibold text-danger">Empresa em lista de sanções</div>
              <div className="text-sm text-ink-dim">
                Este CNPJ aparece no CEIS (Cadastro de Empresas Inidôneas e Suspensas) ou CNEP
                (Cadastro Nacional de Empresas Punidas).
              </div>
            </div>
          </div>
        </div>
      )}

      <h1 className="text-2xl md:text-3xl font-bold tracking-tight">
        {fornecedor.nome || formatCNPJ(fornecedor.ni)}
      </h1>
      <div className="text-ink-dim font-mono text-sm mt-1">CNPJ/CPF: {formatCNPJ(fornecedor.ni)}</div>

      <div className="grid grid-cols-2 gap-3 mt-6">
        <div className="card">
          <div className="flex items-center justify-between">
            <div className="stat-k">Valor total recebido</div>
            <Wallet className="size-4 text-accent" />
          </div>
          <div className="stat-v">{formatBRL(fornecedor.valor_total)}</div>
          <div className="text-xs text-ink-dim mt-1">em contratos arquivados</div>
        </div>
        <div className="card">
          <div className="flex items-center justify-between">
            <div className="stat-k">Contratos</div>
            <FileText className="size-4 text-accent" />
          </div>
          <div className="stat-v">{fornecedor.total_contratos.toLocaleString("pt-BR")}</div>
          <div className="text-xs text-ink-dim mt-1">celebrados com entes públicos</div>
        </div>
      </div>

      <h2 className="text-lg font-semibold mt-8 mb-3">Contratos arquivados</h2>
      <div className="space-y-3">
        {contratos.map((c) => (
          <div key={c.id} className="card">
            <div className="flex flex-wrap gap-3 items-start justify-between">
              <div className="min-w-0 flex-1">
                <div className="text-xs text-ink-dim font-mono">
                  {c.data_assinatura} {c.uf ? "· " + c.uf : ""}
                </div>
                <div className="mt-1 font-semibold line-clamp-2">
                  {c.objeto_contrato || "Sem objeto informado"}
                </div>
                <div className="mt-2 text-sm">
                  <span className="text-ink-dim">Contratado por </span>
                  <span className="font-medium">{c.orgao_razao_social || c.orgao_cnpj || "—"}</span>
                </div>
              </div>
              <div className="text-right shrink-0">
                <div className="text-xl font-bold text-accent">{formatBRL(c.valor_global)}</div>
              </div>
            </div>
          </div>
        ))}
        {contratos.length === 0 && (
          <div className="card text-center text-ink-dim py-8">
            Nenhum contrato encontrado para este fornecedor.
          </div>
        )}
      </div>
    </div>
  );
}
