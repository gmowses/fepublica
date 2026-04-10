// Cross-source detectors — heurísticas que cruzam dados de múltiplas fontes.
// Este é o core do "encontrar corrupção" — nenhuma fonte isolada revela o
// padrão, mas o JOIN entre elas surfa sinais que seriam invisíveis individualmente.
package store

import (
	"context"
	"fmt"
	"strings"
)

const (
	FindingCEAPSancionado    FindingType = "ceap_sancionado"
	FindingCEAPConcentracao  FindingType = "ceap_concentracao"
	FindingFornecedorDuplo   FindingType = "fornecedor_contrato_ceap"
)

// FindCEAPSancionados cruza CEAP (cota parlamentar) × CEIS/CNEP.
// Detecta: deputado pagou um fornecedor que está na lista de empresas
// sancionadas. Severity: HIGH.
//
// Cruzamento: ceap.fornecedor_cnpj = sancionados.ni (extraído de CEIS/CNEP
// canonical_json -> pessoa -> cnpjFormatado, digits-only).
func (s *Store) FindCEAPSancionados(ctx context.Context, limit int) ([]Finding, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		WITH sancionados AS (
			SELECT DISTINCT regexp_replace(
				COALESCE(canonical_json->'pessoa'->>'cnpjFormatado',
				         canonical_json->'pessoa'->>'cpfFormatado', ''),
				'[^0-9]', '', 'g') AS ni
			FROM events
			WHERE source_id IN ('ceis','cnep')
		)
		SELECT
			c.deputado_nome,
			c.partido,
			c.uf,
			c.fornecedor_cnpj,
			COALESCE(c.fornecedor_nome, ''),
			c.tipo_despesa,
			COUNT(*) AS qtd,
			SUM(c.valor_liquido) AS total
		FROM ceap c
		JOIN sancionados s ON s.ni = c.fornecedor_cnpj
		WHERE c.fornecedor_cnpj IS NOT NULL
		  AND c.fornecedor_cnpj <> ''
		GROUP BY c.deputado_nome, c.partido, c.uf, c.fornecedor_cnpj, c.fornecedor_nome, c.tipo_despesa
		ORDER BY SUM(c.valor_liquido) DESC NULLS LAST
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("forenses: ceap_sancionado: %w", err)
	}
	defer rows.Close()

	var out []Finding
	for rows.Next() {
		var (
			deputado, partido, uf, fornCNPJ, fornNome, tipoDespesa string
			qtd                                                    int64
			total                                                  float64
		)
		if err := rows.Scan(&deputado, &partido, &uf, &fornCNPJ, &fornNome, &tipoDespesa, &qtd, &total); err != nil {
			return nil, err
		}
		v := total
		out = append(out, Finding{
			Type:     FindingCEAPSancionado,
			Severity: SeverityHigh,
			Title:    "Deputado usou cota parlamentar com empresa sancionada",
			Subject:  fmt.Sprintf("%s (%s/%s) → %s", deputado, partido, uf, fallback(fornNome, fornCNPJ)),
			Valor:    &v,
			DedupKey: fmt.Sprintf("ceap_sancionado:%s:%s", deputado, fornCNPJ),
			Evidence: map[string]interface{}{
				"deputado":        deputado,
				"partido":         partido,
				"uf":              uf,
				"fornecedor_cnpj": fornCNPJ,
				"fornecedor_nome": fornNome,
				"tipo_despesa":    tipoDespesa,
				"qtd_notas":       qtd,
				"explanation":     fmt.Sprintf("O deputado %s (%s/%s) pagou R$ %.0f em %d notas fiscais para %s, que está presente no CEIS/CNEP (lista de empresas sancionadas). Uso de cota parlamentar com empresa impedida é irregular.", deputado, partido, uf, total, qtd, fallback(fornNome, fornCNPJ)),
			},
		})
	}
	return out, rows.Err()
}

// FindCEAPConcentracao detecta deputados que concentram ≥50% da cota
// em um único fornecedor. Severity: medium. High se ≥80%.
func (s *Store) FindCEAPConcentracao(ctx context.Context, limit int) ([]Finding, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		WITH dep_totals AS (
			SELECT deputado_nome, SUM(valor_liquido) AS total_dep
			FROM ceap
			WHERE deputado_nome IS NOT NULL AND valor_liquido IS NOT NULL
			GROUP BY deputado_nome
			HAVING SUM(valor_liquido) > 0
		),
		par_totals AS (
			SELECT
				c.deputado_nome,
				c.partido,
				c.uf,
				c.fornecedor_cnpj,
				COALESCE(c.fornecedor_nome, '') AS fornecedor_nome,
				COUNT(*) AS qtd,
				SUM(c.valor_liquido) AS total_par
			FROM ceap c
			WHERE c.deputado_nome IS NOT NULL
			  AND c.fornecedor_cnpj IS NOT NULL
			  AND c.valor_liquido IS NOT NULL
			GROUP BY c.deputado_nome, c.partido, c.uf, c.fornecedor_cnpj, c.fornecedor_nome
		)
		SELECT
			p.deputado_nome,
			p.partido,
			p.uf,
			p.fornecedor_cnpj,
			p.fornecedor_nome,
			p.qtd,
			p.total_par,
			d.total_dep,
			(p.total_par / d.total_dep) AS share
		FROM par_totals p
		JOIN dep_totals d USING (deputado_nome)
		WHERE (p.total_par / d.total_dep) >= 0.3 AND p.total_par >= 10000
		ORDER BY (p.total_par / d.total_dep) DESC, p.total_par DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("forenses: ceap_concentracao: %w", err)
	}
	defer rows.Close()

	var out []Finding
	for rows.Next() {
		var (
			deputado, partido, uf, fornCNPJ, fornNome string
			qtd                                       int64
			totalPar, totalDep, share                 float64
		)
		if err := rows.Scan(&deputado, &partido, &uf, &fornCNPJ, &fornNome, &qtd, &totalPar, &totalDep, &share); err != nil {
			return nil, err
		}
		sev := SeverityMedium
		if share >= 0.5 {
			sev = SeverityHigh
		}
		v := totalPar
		out = append(out, Finding{
			Type:     FindingCEAPConcentracao,
			Severity: sev,
			Title:    "Deputado concentra cota parlamentar em um fornecedor",
			Subject:  fmt.Sprintf("%s (%s/%s) → %s", deputado, partido, uf, fallback(fornNome, fornCNPJ)),
			Valor:    &v,
			DedupKey: fmt.Sprintf("ceap_concentracao:%s:%s", deputado, fornCNPJ),
			Evidence: map[string]interface{}{
				"deputado":          deputado,
				"partido":           partido,
				"uf":                uf,
				"fornecedor_cnpj":   fornCNPJ,
				"fornecedor_nome":   fornNome,
				"qtd_notas":         qtd,
				"total_fornecedor":  totalPar,
				"total_cota":        totalDep,
				"share_pct":         share * 100,
				"explanation":       fmt.Sprintf("O deputado %s (%s/%s) direcionou %.1f%% da cota (R$ %.0f de R$ %.0f total) para %s, em %d notas fiscais. Concentração alta em um único fornecedor pode indicar relação preferencial ou empresa de fachada.", deputado, partido, uf, share*100, totalPar, totalDep, fallback(fornNome, fornCNPJ), qtd),
			},
		})
	}
	return out, rows.Err()
}

// FindFornecedorDuplo cruza CEAP × Contratos PNCP.
// Detecta: empresa que recebe tanto pela cota parlamentar de um deputado
// quanto por contrato público com um órgão. A empresa aparece em dois
// circuitos de dinheiro público ao mesmo tempo — pode ser coincidência,
// pode ser indicativo de articulação. Severity: medium.
func (s *Store) FindFornecedorDuplo(ctx context.Context, limit int) ([]Finding, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT
			ce.fornecedor_cnpj,
			COALESCE(MIN(ce.fornecedor_nome), '') AS forn_nome,
			STRING_AGG(DISTINCT ce.deputado_nome || ' (' || ce.partido || '/' || ce.uf || ')', ', ') AS deputados,
			COUNT(DISTINCT ce.deputado_nome) AS qtd_deputados,
			SUM(ce.valor_liquido) AS total_ceap,
			MIN(co.orgao_razao_social) AS orgao,
			COUNT(DISTINCT co.id) AS qtd_contratos,
			SUM(co.valor_global) AS total_contratos
		FROM ceap ce
		JOIN contratos co ON co.fornecedor_ni = ce.fornecedor_cnpj
		WHERE ce.fornecedor_cnpj IS NOT NULL
		  AND ce.fornecedor_cnpj <> ''
		GROUP BY ce.fornecedor_cnpj
		ORDER BY SUM(ce.valor_liquido) + COALESCE(SUM(co.valor_global), 0) DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("forenses: fornecedor_duplo: %w", err)
	}
	defer rows.Close()

	var out []Finding
	for rows.Next() {
		var (
			fornCNPJ, fornNome, deputados, orgao string
			qtdDeputados, qtdContratos           int64
			totalCEAP, totalContratos            float64
		)
		if err := rows.Scan(&fornCNPJ, &fornNome, &deputados, &qtdDeputados,
			&totalCEAP, &orgao, &qtdContratos, &totalContratos); err != nil {
			return nil, err
		}
		total := totalCEAP + totalContratos
		sev := SeverityMedium
		if total >= 500000 || qtdDeputados >= 3 {
			sev = SeverityHigh
		}
		// Truncate deputados list if too long
		if len(deputados) > 300 {
			deputados = deputados[:300] + "..."
		}
		out = append(out, Finding{
			Type:     FindingFornecedorDuplo,
			Severity: sev,
			Title:    "Fornecedor recebe por cota parlamentar E por contrato público",
			Subject:  fallback(fornNome, fornCNPJ),
			Valor:    &total,
			DedupKey: fmt.Sprintf("fornecedor_duplo:%s", fornCNPJ),
			Evidence: map[string]interface{}{
				"fornecedor_cnpj": fornCNPJ,
				"fornecedor_nome": fornNome,
				"deputados":       deputados,
				"qtd_deputados":   qtdDeputados,
				"total_ceap":      totalCEAP,
				"orgao_contrato":  orgao,
				"qtd_contratos":   qtdContratos,
				"total_contratos": totalContratos,
				"explanation": fmt.Sprintf(
					"%s recebe dinheiro público por dois caminhos: R$ %.0f via cota parlamentar (%d deputado(s): %s) e R$ %.0f em %d contrato(s) público(s) com %s. Não é necessariamente irregular, mas uma empresa que aparece em dois circuitos de repasse simultâneos merece investigação sobre articulação ou conflito de interesse.",
					fallback(fornNome, fornCNPJ), totalCEAP, qtdDeputados, truncateStr(deputados, 150), totalContratos, qtdContratos, orgao),
			},
			Link: fmt.Sprintf("/gastos/fornecedores/%s", fornCNPJ),
		})
	}
	return out, rows.Err()
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// Ensure strings import is used (containsFold is in forenses.go)
var _ = strings.Contains
