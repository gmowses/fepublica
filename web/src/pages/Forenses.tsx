import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { AlertTriangle, AlertCircle, AlertOctagon, Search, TrendingUp, CreditCard, EyeOff } from "lucide-react";

type Severity = "high" | "medium" | "low";

interface Finding {
  type: string;
  severity: Severity;
  title: string;
  subject: string;
  valor?: number;
  evidence: Record<string, any>;
  link?: string;
}

interface Summary {
  sancionados_contratados: number;
  concentracoes_orgao: number;
  valor_outliers: number;
  cpgf_alto: number;
  cpgf_opaco: number;
}

function formatBRL(n?: number): string {
  if (n == null) return "—";
  return n.toLocaleString("pt-BR", { style: "currency", currency: "BRL", maximumFractionDigits: 0 });
}

function severityClass(s: Severity): string {
  if (s === "high") return "border-danger/40 bg-danger/10 text-danger";
  if (s === "medium") return "border-warn/40 bg-warn/10 text-warn";
  return "border-ink/20 bg-bg-card text-ink-dim";
}

function severityLabel(s: Severity): string {
  if (s === "high") return "ALTA";
  if (s === "medium") return "MÉDIA";
  return "BAIXA";
}

export function Forenses() {
  const summaryQ = useQuery({
    queryKey: ["forenses-summary"],
    queryFn: () => fetch("/api/forenses/summary").then((r) => r.json()) as Promise<Summary>,
  });
  const sancionadosQ = useQuery({
    queryKey: ["forenses-sancionados"],
    queryFn: () => fetch("/api/forenses/sancionados?limit=20").then((r) => r.json()) as Promise<{ findings: Finding[] }>,
  });
  const concentracaoQ = useQuery({
    queryKey: ["forenses-concentracao"],
    queryFn: () => fetch("/api/forenses/concentracao?limit=20").then((r) => r.json()) as Promise<{ findings: Finding[] }>,
  });
  const outliersQ = useQuery({
    queryKey: ["forenses-outliers"],
    queryFn: () => fetch("/api/forenses/outliers?limit=20").then((r) => r.json()) as Promise<{ findings: Finding[] }>,
  });
  const cpgfAltoQ = useQuery({
    queryKey: ["forenses-cpgf-alto"],
    queryFn: () => fetch("/api/forenses/cpgf-alto?limit=20").then((r) => r.json()) as Promise<{ findings: Finding[] }>,
  });
  const cpgfOpacoQ = useQuery({
    queryKey: ["forenses-cpgf-opaco"],
    queryFn: () => fetch("/api/forenses/cpgf-opaco?limit=20").then((r) => r.json()) as Promise<{ findings: Finding[] }>,
  });

  const summary = summaryQ.data;

  return (
    <div className="container-app py-6 md:py-10">
      <nav className="text-xs font-mono text-ink-dim mb-3">
        <Link to="/" className="hover:text-accent">início</Link> / forenses
      </nav>

      <div className="text-[0.72rem] font-mono uppercase tracking-widest text-accent mb-3">
        Forense — detector de padrões suspeitos
      </div>
      <h1 className="text-3xl md:text-5xl font-bold leading-tight tracking-tight max-w-4xl">
        Onde os números não fecham.
      </h1>
      <p className="mt-4 text-lg text-ink-dim max-w-3xl">
        Heurísticas automáticas que cruzam contratos públicos, cartões corporativos
        e listas de empresas sancionadas pra surfar padrões merecedores de revisão.
        <strong className="text-ink"> Isto não é acusação de fraude</strong> — é
        uma fila de casos pra um humano olhar com calma.
      </p>

      {/* Summary cards */}
      <div className="grid grid-cols-2 md:grid-cols-5 gap-3 mt-8">
        <SummaryCard
          k="Sancionados contratados"
          v={summary?.sancionados_contratados ?? 0}
          Icon={AlertOctagon}
          severity="high"
        />
        <SummaryCard
          k="Concentração de fornecedor"
          v={summary?.concentracoes_orgao ?? 0}
          Icon={TrendingUp}
          severity="medium"
        />
        <SummaryCard
          k="Valor outlier"
          v={summary?.valor_outliers ?? 0}
          Icon={AlertCircle}
          severity="medium"
        />
        <SummaryCard
          k="CPGF — valor alto"
          v={summary?.cpgf_alto ?? 0}
          Icon={CreditCard}
          severity="medium"
        />
        <SummaryCard
          k="CPGF — estab. opaco"
          v={summary?.cpgf_opaco ?? 0}
          Icon={EyeOff}
          severity="low"
        />
      </div>

      <Section
        title="Sancionados contratados"
        explanation="Empresas presentes em CEIS ou CNEP que ainda assim aparecem em contratos públicos. Contratar empresas sancionadas viola o art. 87 da Lei 8.666/93 / art. 156 da Lei 14.133/21."
        Icon={AlertOctagon}
      >
        <FindingList findings={sancionadosQ.data?.findings ?? []} />
      </Section>

      <Section
        title="Concentração de fornecedor por órgão"
        explanation="Pares (órgão, fornecedor) onde o fornecedor concentra ≥50% do valor total contratado pelo órgão, ou tem ≥3 contratos. Concentrações altas podem indicar direcionamento."
        Icon={TrendingUp}
      >
        <FindingList findings={concentracaoQ.data?.findings ?? []} />
      </Section>

      <Section
        title="Contratos com valor outlier"
        explanation="Contratos cujo valor é ≥5× a mediana de contratos do mesmo órgão. Ratios extremos não significam fraude — significam que esse contrato é qualitativamente diferente do que o órgão costuma assinar."
        Icon={AlertCircle}
      >
        <FindingList findings={outliersQ.data?.findings ?? []} />
      </Section>

      <Section
        title="CPGF — transações de valor alto"
        explanation="Transações únicas no Cartão de Pagamento do Governo Federal acima de R$ 10.000. CPGF é desenhado pra despesas operacionais menores; valores altos exigem checagem."
        Icon={CreditCard}
      >
        <FindingList findings={cpgfAltoQ.data?.findings ?? []} />
      </Section>

      <Section
        title="CPGF — gastos em estabelecimento sem identificação"
        explanation="Quando o estabelecimento aparece como 'SEM INFORMAÇÃO', é impossível verificar o que foi pago. O agregado por portador é o sinal — valores altos somados em estabelecimentos opacos merecem atenção."
        Icon={EyeOff}
      >
        <FindingList findings={cpgfOpacoQ.data?.findings ?? []} />
      </Section>
    </div>
  );
}

function SummaryCard({
  k,
  v,
  Icon,
  severity,
}: {
  k: string;
  v: number;
  Icon: any;
  severity: Severity;
}) {
  return (
    <div className={`card border ${severityClass(severity)}`}>
      <div className="flex items-center justify-between">
        <div className="text-xs uppercase tracking-wider opacity-80">{k}</div>
        <Icon className="size-4" />
      </div>
      <div className="text-3xl font-bold mt-2">{v.toLocaleString("pt-BR")}</div>
      <div className="text-xs opacity-70 mt-1">casos detectados</div>
    </div>
  );
}

function Section({
  title,
  explanation,
  Icon,
  children,
}: {
  title: string;
  explanation: string;
  Icon: any;
  children: React.ReactNode;
}) {
  return (
    <section className="mt-10">
      <div className="flex items-start gap-3">
        <Icon className="size-6 text-accent mt-1 shrink-0" />
        <div>
          <h2 className="text-2xl font-bold tracking-tight">{title}</h2>
          <p className="text-sm text-ink-dim mt-1 max-w-3xl">{explanation}</p>
        </div>
      </div>
      <div className="mt-4 space-y-3">{children}</div>
    </section>
  );
}

function FindingList({ findings }: { findings: Finding[] }) {
  if (findings.length === 0) {
    return (
      <div className="card text-center text-ink-dim py-6">
        Nenhum caso detectado neste momento.
      </div>
    );
  }
  return (
    <>
      {findings.map((f, i) => (
        <FindingCard key={i} f={f} />
      ))}
    </>
  );
}

function FindingCard({ f }: { f: Finding }) {
  return (
    <div className={`card border ${severityClass(f.severity)}`}>
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2 mb-1">
            <span className="text-[0.65rem] font-mono uppercase tracking-wider px-1.5 py-0.5 rounded border border-current">
              {severityLabel(f.severity)}
            </span>
            <span className="text-xs font-mono opacity-70">{f.type}</span>
          </div>
          <div className="font-semibold text-ink line-clamp-2">{f.subject}</div>
          <div className="text-sm text-ink-dim mt-2">
            {f.evidence.explanation}
          </div>
          <Evidence evidence={f.evidence} />
        </div>
        <div className="text-right shrink-0">
          {f.valor != null && (
            <div className="text-2xl font-bold">{formatBRL(f.valor)}</div>
          )}
          {f.link && (
            <Link to={f.link} className="text-xs underline opacity-80 hover:opacity-100 mt-1 inline-flex items-center gap-1">
              <Search className="size-3" /> investigar
            </Link>
          )}
        </div>
      </div>
    </div>
  );
}

function Evidence({ evidence }: { evidence: Record<string, any> }) {
  const visible = Object.entries(evidence).filter(
    ([k]) => k !== "explanation"
  );
  if (visible.length === 0) return null;
  return (
    <div className="mt-3 grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-1 text-xs font-mono text-ink-dim">
      {visible.map(([k, v]) => (
        <div key={k} className="truncate">
          <span className="opacity-70">{k}:</span>{" "}
          <span className="text-ink">{formatEvidenceValue(v)}</span>
        </div>
      ))}
    </div>
  );
}

function formatEvidenceValue(v: any): string {
  if (v == null) return "—";
  if (typeof v === "number") {
    if (Number.isInteger(v)) return v.toLocaleString("pt-BR");
    if (v < 1) return v.toFixed(3);
    return v.toLocaleString("pt-BR", { maximumFractionDigits: 1 });
  }
  return String(v);
}
