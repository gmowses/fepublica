// Forenses queries — heurísticas para detectar padrões suspeitos sobre os
// dados já materializados (contratos, cartoes, eventos CEIS/CNEP).
//
// Cada finder devolve uma lista de Findings tipados, com a evidência mínima
// pra que o consumidor (UI ou alerta) possa explicar ao usuário POR QUE
// aquele caso é suspeito. Não classifica fraude — só sinaliza para revisão.
package store

import (
	"context"
	"fmt"
)

// Severity classifies a finding's confidence/impact.
type Severity string

const (
	SeverityHigh   Severity = "high"
	SeverityMedium Severity = "medium"
	SeverityLow    Severity = "low"
)

// FindingType is the detector that produced the finding.
type FindingType string

const (
	FindingSancionadoContratado FindingType = "sancionado_contratado"
	FindingConcentracaoOrgao    FindingType = "concentracao_orgao"
	FindingValorOutlier         FindingType = "valor_outlier"
	FindingCPGFAlto             FindingType = "cpgf_alto"
	FindingCPGFEstabOpaco       FindingType = "cpgf_estab_opaco"
)

// Finding is a single suspicious pattern surfaced by a detector.
type Finding struct {
	Type     FindingType            `json:"type"`
	Severity Severity               `json:"severity"`
	Title    string                 `json:"title"`
	Subject  string                 `json:"subject"`
	Valor    *float64               `json:"valor,omitempty"`
	Evidence map[string]interface{} `json:"evidence"`
	Link     string                 `json:"link,omitempty"`
}

// FindSancionadosContratados returns contratos onde o fornecedor aparece
// em CEIS ou CNEP. Severity: high.
func (s *Store) FindSancionadosContratados(ctx context.Context, limit int) ([]Finding, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT
			c.fornecedor_ni,
			COALESCE(c.fornecedor_nome, ''),
			COALESCE(c.orgao_razao_social, c.orgao_cnpj, ''),
			c.valor_global,
			c.objeto_contrato,
			c.id
		FROM contratos c
		WHERE c.fornecedor_ni IS NOT NULL
		  AND c.fornecedor_ni <> ''
		  AND EXISTS (
		    SELECT 1 FROM events e
		    WHERE e.source_id IN ('ceis', 'cnep')
		      AND e.canonical_json::text ILIKE '%' || c.fornecedor_ni || '%'
		  )
		ORDER BY c.valor_global DESC NULLS LAST
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("forenses: sancionados: %w", err)
	}
	defer rows.Close()

	var out []Finding
	for rows.Next() {
		var (
			ni, nome, orgao, objeto string
			valor                   *float64
			id                      int64
		)
		if err := rows.Scan(&ni, &nome, &orgao, &valor, &objeto, &id); err != nil {
			return nil, err
		}
		f := Finding{
			Type:     FindingSancionadoContratado,
			Severity: SeverityHigh,
			Title:    "Empresa sancionada com contrato público",
			Subject:  fallback(nome, ni),
			Valor:    valor,
			Evidence: map[string]interface{}{
				"fornecedor_ni":   ni,
				"fornecedor_nome": nome,
				"orgao":           orgao,
				"objeto":          truncateString(objeto, 200),
				"explanation":     "Fornecedor presente em CEIS/CNEP. Contratar empresas sancionadas viola o art. 87 da Lei 8.666/93 / art. 156 da Lei 14.133/21.",
			},
			Link: fmt.Sprintf("/gastos/fornecedores/%s", ni),
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// FindConcentracaoOrgao returns (órgão, fornecedor) pairs onde o fornecedor
// soma >50% do gasto total do órgão OU tem ≥3 contratos.
// Severity: medium.
func (s *Store) FindConcentracaoOrgao(ctx context.Context, limit int) ([]Finding, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		WITH orgao_totals AS (
			SELECT orgao_cnpj, SUM(valor_global) AS total_orgao, COUNT(*) AS qtd_orgao
			FROM contratos
			WHERE orgao_cnpj IS NOT NULL AND valor_global IS NOT NULL
			GROUP BY orgao_cnpj
			HAVING SUM(valor_global) > 0
		),
		par_totals AS (
			SELECT
				c.orgao_cnpj,
				MIN(c.orgao_razao_social) AS orgao_nome,
				c.fornecedor_ni,
				MIN(c.fornecedor_nome) AS fornecedor_nome,
				COUNT(*) AS qtd,
				SUM(c.valor_global) AS total_par
			FROM contratos c
			WHERE c.orgao_cnpj IS NOT NULL
			  AND c.fornecedor_ni IS NOT NULL
			  AND c.valor_global IS NOT NULL
			GROUP BY c.orgao_cnpj, c.fornecedor_ni
		)
		SELECT
			p.orgao_cnpj,
			p.orgao_nome,
			p.fornecedor_ni,
			p.fornecedor_nome,
			p.qtd,
			p.total_par,
			t.total_orgao,
			(p.total_par / t.total_orgao) AS share
		FROM par_totals p
		JOIN orgao_totals t USING (orgao_cnpj)
		WHERE (p.total_par / t.total_orgao) >= 0.5 OR p.qtd >= 3
		ORDER BY (p.total_par / t.total_orgao) DESC, p.total_par DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("forenses: concentracao: %w", err)
	}
	defer rows.Close()

	var out []Finding
	for rows.Next() {
		var (
			orgaoCNPJ, orgaoNome, fornNi, fornNome string
			qtd                                    int64
			totalPar, totalOrgao, share            float64
		)
		if err := rows.Scan(&orgaoCNPJ, &orgaoNome, &fornNi, &fornNome,
			&qtd, &totalPar, &totalOrgao, &share); err != nil {
			return nil, err
		}
		sev := SeverityMedium
		if share >= 0.8 {
			sev = SeverityHigh
		}
		v := totalPar
		f := Finding{
			Type:     FindingConcentracaoOrgao,
			Severity: sev,
			Title:    "Concentração de fornecedor em um órgão",
			Subject:  fmt.Sprintf("%s ← %s", fallback(orgaoNome, orgaoCNPJ), fallback(fornNome, fornNi)),
			Valor:    &v,
			Evidence: map[string]interface{}{
				"orgao":             fallback(orgaoNome, orgaoCNPJ),
				"orgao_cnpj":        orgaoCNPJ,
				"fornecedor":        fallback(fornNome, fornNi),
				"fornecedor_ni":     fornNi,
				"contratos":         qtd,
				"total_fornecedor":  totalPar,
				"total_orgao":       totalOrgao,
				"share_pct":         share * 100,
				"explanation":       fmt.Sprintf("Este fornecedor concentra %.1f%% do gasto deste órgão (%d contratos). Concentrações altas podem indicar direcionamento.", share*100, qtd),
			},
			Link: fmt.Sprintf("/gastos/fornecedores/%s", fornNi),
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// FindValorOutliers returns contratos cujo valor é ≥5× a mediana (estimada
// pelo percentile_cont 0.5) do mesmo órgão. Severity: medium.
func (s *Store) FindValorOutliers(ctx context.Context, limit int) ([]Finding, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		WITH stats AS (
			SELECT
				orgao_cnpj,
				PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY valor_global) AS mediana,
				COUNT(*) AS n
			FROM contratos
			WHERE orgao_cnpj IS NOT NULL AND valor_global IS NOT NULL
			GROUP BY orgao_cnpj
			HAVING COUNT(*) >= 3
		)
		SELECT
			c.id,
			COALESCE(c.fornecedor_nome, c.fornecedor_ni, ''),
			c.fornecedor_ni,
			COALESCE(c.orgao_razao_social, c.orgao_cnpj, ''),
			c.valor_global,
			c.objeto_contrato,
			s.mediana,
			(c.valor_global / s.mediana) AS ratio
		FROM contratos c
		JOIN stats s USING (orgao_cnpj)
		WHERE c.valor_global >= 5 * s.mediana
		  AND s.mediana > 0
		ORDER BY (c.valor_global / s.mediana) DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("forenses: outliers: %w", err)
	}
	defer rows.Close()

	var out []Finding
	for rows.Next() {
		var (
			id                int64
			fornNome, fornNi  string
			orgao, objeto     string
			valor, mediana    float64
			ratio             float64
		)
		if err := rows.Scan(&id, &fornNome, &fornNi, &orgao, &valor, &objeto, &mediana, &ratio); err != nil {
			return nil, err
		}
		sev := SeverityMedium
		if ratio >= 20 {
			sev = SeverityHigh
		}
		v := valor
		f := Finding{
			Type:     FindingValorOutlier,
			Severity: sev,
			Title:    "Contrato muito acima da mediana do órgão",
			Subject:  fornNome,
			Valor:    &v,
			Evidence: map[string]interface{}{
				"orgao":         orgao,
				"fornecedor":    fornNome,
				"fornecedor_ni": fornNi,
				"objeto":        truncateString(objeto, 200),
				"mediana_orgao": mediana,
				"ratio":         ratio,
				"explanation":   fmt.Sprintf("Este contrato vale R$ %.0f, %.1fx a mediana de contratos do mesmo órgão (R$ %.0f). Outliers extremos merecem revisão.", valor, ratio, mediana),
			},
			Link: fmt.Sprintf("/gastos/fornecedores/%s", fornNi),
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// FindCPGFAltoValor returns CPGF transações com valor único > R$ 10.000.
// Severity: medium para >10k, high para >50k.
func (s *Store) FindCPGFAltoValor(ctx context.Context, limit int) ([]Finding, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT
			id,
			COALESCE(portador_nome, ''),
			COALESCE(portador_cpf, ''),
			COALESCE(estab_nome, ''),
			COALESCE(estab_cnpj, ''),
			COALESCE(orgao_max_nome, ''),
			COALESCE(unidade_nome, ''),
			data_transacao,
			valor_transacao
		FROM cartoes
		WHERE valor_transacao >= 10000
		ORDER BY valor_transacao DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("forenses: cpgf_alto: %w", err)
	}
	defer rows.Close()

	var out []Finding
	for rows.Next() {
		var (
			id                                                    int64
			portador, cpf, estab, estabCNPJ, orgao, unidade       string
			data                                                  *string
			valor                                                 *float64
		)
		if err := rows.Scan(&id, &portador, &cpf, &estab, &estabCNPJ, &orgao, &unidade, &data, &valor); err != nil {
			return nil, err
		}
		sev := SeverityMedium
		if valor != nil && *valor >= 50000 {
			sev = SeverityHigh
		}
		f := Finding{
			Type:     FindingCPGFAlto,
			Severity: sev,
			Title:    "Transação CPGF de valor elevado",
			Subject:  fallback(portador, "?"),
			Valor:    valor,
			Evidence: map[string]interface{}{
				"portador":      portador,
				"portador_cpf":  cpf,
				"estabelecimento": estab,
				"estab_cnpj":    estabCNPJ,
				"orgao":         orgao,
				"unidade":       unidade,
				"explanation":   "Transações únicas de valor alto no cartão corporativo são raras e merecem checagem com a finalidade declarada.",
			},
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// FindCPGFEstabOpaco returns CPGF transações em estabelecimentos sem
// informação cadastral. Severity: low individualmente, mas o agregado por
// portador pode revelar abusos.
func (s *Store) FindCPGFEstabOpaco(ctx context.Context, limit int) ([]Finding, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT
			COALESCE(portador_nome, ''),
			COALESCE(portador_cpf, ''),
			COALESCE(orgao_max_nome, ''),
			COUNT(*) AS qtd,
			SUM(valor_transacao) AS total
		FROM cartoes
		WHERE estab_nome ILIKE '%SEM INFORMACAO%' OR estab_nome IS NULL OR estab_nome = ''
		GROUP BY portador_cpf, portador_nome, orgao_max_nome
		HAVING SUM(valor_transacao) > 0
		ORDER BY SUM(valor_transacao) DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("forenses: cpgf_opaco: %w", err)
	}
	defer rows.Close()

	var out []Finding
	for rows.Next() {
		var (
			portador, cpf, orgao string
			qtd                  int64
			total                float64
		)
		if err := rows.Scan(&portador, &cpf, &orgao, &qtd, &total); err != nil {
			return nil, err
		}
		sev := SeverityLow
		if total >= 5000 {
			sev = SeverityMedium
		}
		v := total
		f := Finding{
			Type:     FindingCPGFEstabOpaco,
			Severity: sev,
			Title:    "Gastos CPGF em estabelecimento sem identificação",
			Subject:  fallback(portador, "?"),
			Valor:    &v,
			Evidence: map[string]interface{}{
				"portador":     portador,
				"portador_cpf": cpf,
				"orgao":        orgao,
				"transacoes":   qtd,
				"explanation":  "Estabelecimento marcado como 'SEM INFORMAÇÃO' impossibilita verificar o que foi pago. O total agregado por portador é o sinal.",
			},
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// ForensesSummary é o agregado para a página /forenses.
type ForensesSummary struct {
	Sancionados   int64 `json:"sancionados_contratados"`
	Concentracoes int64 `json:"concentracoes_orgao"`
	Outliers      int64 `json:"valor_outliers"`
	CPGFAlto      int64 `json:"cpgf_alto"`
	CPGFOpaco     int64 `json:"cpgf_opaco"`
}

// GetForensesSummary returns counts for each detector. Cheap for the
// dashboard top section. Each detector reuses its own query semantics.
func (s *Store) GetForensesSummary(ctx context.Context) (*ForensesSummary, error) {
	var sum ForensesSummary
	err := s.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM contratos c
			 WHERE c.fornecedor_ni IS NOT NULL AND c.fornecedor_ni <> ''
			   AND EXISTS (SELECT 1 FROM events e
			               WHERE e.source_id IN ('ceis','cnep')
			                 AND e.canonical_json::text ILIKE '%' || c.fornecedor_ni || '%')),
			(SELECT COUNT(*) FROM (
				WITH orgao_totals AS (
					SELECT orgao_cnpj, SUM(valor_global) AS t FROM contratos
					WHERE orgao_cnpj IS NOT NULL AND valor_global IS NOT NULL
					GROUP BY orgao_cnpj HAVING SUM(valor_global) > 0
				)
				SELECT 1 FROM contratos c
				JOIN orgao_totals t USING (orgao_cnpj)
				WHERE c.fornecedor_ni IS NOT NULL AND c.valor_global IS NOT NULL
				GROUP BY c.orgao_cnpj, c.fornecedor_ni, t.t
				HAVING SUM(c.valor_global)/t.t >= 0.5 OR COUNT(*) >= 3
			) x),
			(SELECT COUNT(*) FROM contratos c
			 JOIN (
				SELECT orgao_cnpj, PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY valor_global) AS mediana
				FROM contratos
				WHERE orgao_cnpj IS NOT NULL AND valor_global IS NOT NULL
				GROUP BY orgao_cnpj HAVING COUNT(*) >= 3
			 ) s USING (orgao_cnpj)
			 WHERE c.valor_global >= 5 * s.mediana AND s.mediana > 0),
			(SELECT COUNT(*) FROM cartoes WHERE valor_transacao >= 10000),
			(SELECT COUNT(*) FROM (
				SELECT 1 FROM cartoes
				WHERE estab_nome ILIKE '%SEM INFORMACAO%' OR estab_nome IS NULL OR estab_nome = ''
				GROUP BY portador_cpf, portador_nome, orgao_max_nome
				HAVING SUM(valor_transacao) > 0
			) y)
	`).Scan(&sum.Sancionados, &sum.Concentracoes, &sum.Outliers, &sum.CPGFAlto, &sum.CPGFOpaco)
	if err != nil {
		return nil, fmt.Errorf("forenses: summary: %w", err)
	}
	return &sum, nil
}

func fallback(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func truncateString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
