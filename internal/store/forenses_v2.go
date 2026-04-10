// Sprint B detectors — 5 novas heurísticas SQL puras sobre projections
// existentes (contratos + cartoes). Chamadas pelo forenses-runner.
package store

import (
	"context"
	"fmt"
	"math"
	"time"
)

// FindMesmoDiaMesmoOrgao detecta órgãos que assinam ≥5 contratos no mesmo
// dia — corrida de fim de exercício / contratação atropelada. Severity:
// medium individualmente, high se o pacote do dia ultrapassa R$ 1M.
func (s *Store) FindMesmoDiaMesmoOrgao(ctx context.Context, limit int) ([]Finding, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT
			orgao_cnpj,
			COALESCE(MIN(orgao_razao_social), ''),
			data_assinatura,
			COUNT(*) AS qtd,
			SUM(valor_global) AS total
		FROM contratos
		WHERE orgao_cnpj IS NOT NULL
		  AND data_assinatura IS NOT NULL
		  AND valor_global IS NOT NULL
		GROUP BY orgao_cnpj, data_assinatura
		HAVING COUNT(*) >= 5
		ORDER BY SUM(valor_global) DESC NULLS LAST
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("forenses: mesmo_dia: %w", err)
	}
	defer rows.Close()

	var out []Finding
	for rows.Next() {
		var (
			orgaoCNPJ, orgaoNome string
			data                 time.Time
			qtd                  int64
			total                float64
		)
		if err := rows.Scan(&orgaoCNPJ, &orgaoNome, &data, &qtd, &total); err != nil {
			return nil, fmt.Errorf("forenses: mesmo_dia scan: %w", err)
		}
		dataStr := data.Format("2006-01-02")
		sev := SeverityMedium
		if total >= 1_000_000 {
			sev = SeverityHigh
		}
		v := total
		out = append(out, Finding{
			Type:     FindingMesmoDiaOrgao,
			Severity: sev,
			Title:    "Vários contratos assinados no mesmo dia pelo mesmo órgão",
			Subject:  fmt.Sprintf("%s · %s", fallback(orgaoNome, orgaoCNPJ), dataStr),
			Valor:    &v,
			DedupKey: fmt.Sprintf("mesmo_dia_orgao:%s:%s", orgaoCNPJ, dataStr),
			Evidence: map[string]interface{}{
				"orgao":          fallback(orgaoNome, orgaoCNPJ),
				"orgao_cnpj":     orgaoCNPJ,
				"data":           dataStr,
				"qtd_contratos":  qtd,
				"total_pacote":   total,
				"explanation":    fmt.Sprintf("Este órgão assinou %d contratos em um único dia (%s) somando R$ %.0f. Pacotes desse tamanho num único dia frequentemente refletem corrida de fim de exercício, com revisão mais fraca.", qtd, dataStr, total),
			},
		})
	}
	return out, rows.Err()
}

// FindValoresRedondos detecta contratos cujo valor é exatamente um múltiplo
// "redondo" significativo — R$ 100k, 250k, 500k, 1M, 5M, 10M. Valores
// exatos são raros em contratação real (que tem itens unitários, taxas,
// frete) e podem indicar valor "negociado" pra caber em uma faixa de
// dispensa de licitação.
func (s *Store) FindValoresRedondos(ctx context.Context, limit int) ([]Finding, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT
			c.id,
			COALESCE(c.fornecedor_nome, c.fornecedor_ni, ''),
			c.fornecedor_ni,
			COALESCE(c.orgao_razao_social, c.orgao_cnpj, ''),
			c.valor_global,
			c.objeto_contrato
		FROM contratos c
		WHERE c.valor_global IS NOT NULL
		  AND c.valor_global >= 50000
		  AND (
			c.valor_global = 50000 OR c.valor_global = 100000 OR c.valor_global = 150000 OR
			c.valor_global = 200000 OR c.valor_global = 250000 OR c.valor_global = 300000 OR
			c.valor_global = 500000 OR c.valor_global = 750000 OR c.valor_global = 1000000 OR
			c.valor_global = 1500000 OR c.valor_global = 2000000 OR c.valor_global = 2500000 OR
			c.valor_global = 5000000 OR c.valor_global = 10000000
		  )
		ORDER BY c.valor_global DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("forenses: valores_redondos: %w", err)
	}
	defer rows.Close()

	var out []Finding
	for rows.Next() {
		var (
			id                int64
			fornNome, fornNi  string
			orgao, objeto     string
			valor             float64
		)
		if err := rows.Scan(&id, &fornNome, &fornNi, &orgao, &valor, &objeto); err != nil {
			return nil, err
		}
		sev := SeverityLow
		if valor >= 500000 {
			sev = SeverityMedium
		}
		v := valor
		out = append(out, Finding{
			Type:     FindingValorRedondo,
			Severity: sev,
			Title:    "Contrato com valor exatamente redondo",
			Subject:  fornNome,
			Valor:    &v,
			DedupKey: fmt.Sprintf("valor_redondo:%d", id),
			Evidence: map[string]interface{}{
				"orgao":         orgao,
				"fornecedor":    fornNome,
				"fornecedor_ni": fornNi,
				"objeto":        truncateString(objeto, 200),
				"explanation":   "Valor exatamente igual a um múltiplo redondo (R$ 100k, 500k, 1M, etc) é incomum em contratação real (que tem itens unitários, taxas e frete). Frequentemente indica valor 'negociado' pra caber em uma faixa de dispensa.",
			},
			Link: fmt.Sprintf("/gastos/fornecedores/%s", fornNi),
		})
	}
	return out, rows.Err()
}

// FindFornecedorMultiUF detecta fornecedores que aparecem em ≥3 UFs
// distintas. Pode ser operação nacional legítima (consultoria, software)
// ou laranja distribuído por estados.
func (s *Store) FindFornecedorMultiUF(ctx context.Context, limit int) ([]Finding, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT
			fornecedor_ni,
			COALESCE(MIN(fornecedor_nome), ''),
			COUNT(DISTINCT uf) AS uf_count,
			ARRAY_AGG(DISTINCT uf ORDER BY uf) AS ufs,
			COUNT(*) AS qtd_contratos,
			SUM(valor_global) AS total
		FROM contratos
		WHERE fornecedor_ni IS NOT NULL
		  AND fornecedor_ni <> ''
		  AND uf IS NOT NULL
		  AND valor_global IS NOT NULL
		GROUP BY fornecedor_ni
		HAVING COUNT(DISTINCT uf) >= 3
		ORDER BY SUM(valor_global) DESC NULLS LAST
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("forenses: multi_uf: %w", err)
	}
	defer rows.Close()

	var out []Finding
	for rows.Next() {
		var (
			fornNi, fornNome     string
			ufCount, qtdContratos int64
			ufs                  []string
			total                float64
		)
		if err := rows.Scan(&fornNi, &fornNome, &ufCount, &ufs, &qtdContratos, &total); err != nil {
			return nil, err
		}
		sev := SeverityLow
		if ufCount >= 5 {
			sev = SeverityMedium
		}
		if ufCount >= 8 {
			sev = SeverityHigh
		}
		v := total
		out = append(out, Finding{
			Type:     FindingFornecedorMultiUF,
			Severity: sev,
			Title:    "Fornecedor com contratos em múltiplas UFs",
			Subject:  fallback(fornNome, fornNi),
			Valor:    &v,
			DedupKey: fmt.Sprintf("fornecedor_multi_uf:%s", fornNi),
			Evidence: map[string]interface{}{
				"fornecedor":     fallback(fornNome, fornNi),
				"fornecedor_ni":  fornNi,
				"uf_count":       ufCount,
				"ufs":            ufs,
				"qtd_contratos":  qtdContratos,
				"explanation":    fmt.Sprintf("Este fornecedor aparece em %d UFs distintas (%v). Pode ser operação nacional legítima ou um laranja distribuído por estados.", ufCount, ufs),
			},
			Link: fmt.Sprintf("/gastos/fornecedores/%s", fornNi),
		})
	}
	return out, rows.Err()
}

// FindCPGFConcentradoNoMes detecta portadores cujo gasto total no CPGF em
// um único mês ultrapassa R$ 50k. CPGF é desenhado pra despesa operacional
// pequena; concentração mensal alta merece checagem.
func (s *Store) FindCPGFConcentradoNoMes(ctx context.Context, limit int) ([]Finding, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT
			COALESCE(portador_cpf, ''),
			COALESCE(MIN(portador_nome), ''),
			COALESCE(MIN(orgao_max_nome), ''),
			mes_extrato,
			COUNT(*) AS qtd,
			SUM(valor_transacao) AS total
		FROM cartoes
		WHERE portador_cpf IS NOT NULL
		  AND portador_cpf <> ''
		  AND mes_extrato IS NOT NULL
		GROUP BY portador_cpf, mes_extrato
		HAVING SUM(valor_transacao) >= 50000
		ORDER BY SUM(valor_transacao) DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("forenses: cpgf_concentrado: %w", err)
	}
	defer rows.Close()

	var out []Finding
	for rows.Next() {
		var (
			cpf, nome, orgao, mes string
			qtd                   int64
			total                 float64
		)
		if err := rows.Scan(&cpf, &nome, &orgao, &mes, &qtd, &total); err != nil {
			return nil, err
		}
		sev := SeverityMedium
		if total >= 200000 {
			sev = SeverityHigh
		}
		v := total
		out = append(out, Finding{
			Type:     FindingCPGFConcentradoMes,
			Severity: sev,
			Title:    "Portador CPGF com gasto concentrado em um único mês",
			Subject:  fmt.Sprintf("%s · %s", fallback(nome, "?"), mes),
			Valor:    &v,
			DedupKey: fmt.Sprintf("cpgf_concentrado_mes:%s:%s", cpf, mes),
			Evidence: map[string]interface{}{
				"portador":     nome,
				"portador_cpf": cpf,
				"orgao":        orgao,
				"mes_extrato":  mes,
				"qtd_transacoes": qtd,
				"explanation":  fmt.Sprintf("Este portador acumulou R$ %.0f no CPGF no mês %s, em %d transações. CPGF é destinado a despesas operacionais pequenas; concentrações mensais altas merecem checagem.", total, mes, qtd),
			},
		})
	}
	return out, rows.Err()
}

// FindValorCrescimentoGeometrico detecta fornecedores cujos contratos com
// um mesmo órgão têm valores que crescem >2× entre contratos sequenciais.
// Pode indicar relação que vai escalando — fornecedor "preferido" recebendo
// contratos cada vez maiores.
func (s *Store) FindValorCrescimentoGeometrico(ctx context.Context, limit int) ([]Finding, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		WITH par AS (
			SELECT
				orgao_cnpj,
				MIN(orgao_razao_social) AS orgao_nome,
				fornecedor_ni,
				MIN(fornecedor_nome) AS fornecedor_nome,
				ARRAY_AGG(valor_global ORDER BY data_assinatura) AS valores,
				COUNT(*) AS qtd,
				MIN(valor_global) AS minv,
				MAX(valor_global) AS maxv,
				SUM(valor_global) AS total
			FROM contratos
			WHERE orgao_cnpj IS NOT NULL
			  AND fornecedor_ni IS NOT NULL
			  AND data_assinatura IS NOT NULL
			  AND valor_global IS NOT NULL
			  AND valor_global > 0
			GROUP BY orgao_cnpj, fornecedor_ni
			HAVING COUNT(*) >= 3
			   AND MAX(valor_global) >= 2 * MIN(valor_global)
		)
		SELECT orgao_cnpj, orgao_nome, fornecedor_ni, fornecedor_nome,
		       valores, qtd, minv, maxv, total
		FROM par
		ORDER BY maxv DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("forenses: crescimento: %w", err)
	}
	defer rows.Close()

	var out []Finding
	for rows.Next() {
		var (
			orgaoCNPJ, orgaoNome, fornNi, fornNome string
			valores                                []float64
			qtd                                    int64
			minv, maxv, total                      float64
		)
		if err := rows.Scan(&orgaoCNPJ, &orgaoNome, &fornNi, &fornNome,
			&valores, &qtd, &minv, &maxv, &total); err != nil {
			return nil, err
		}
		ratio := maxv / math.Max(minv, 1)
		sev := SeverityMedium
		if ratio >= 10 {
			sev = SeverityHigh
		}
		v := total
		out = append(out, Finding{
			Type:     FindingValorCrescimento,
			Severity: sev,
			Title:    "Contratos crescentes entre o mesmo órgão e fornecedor",
			Subject:  fmt.Sprintf("%s ← %s", fallback(orgaoNome, orgaoCNPJ), fallback(fornNome, fornNi)),
			Valor:    &v,
			DedupKey: fmt.Sprintf("valor_crescimento:%s:%s", orgaoCNPJ, fornNi),
			Evidence: map[string]interface{}{
				"orgao":         fallback(orgaoNome, orgaoCNPJ),
				"fornecedor":    fallback(fornNome, fornNi),
				"fornecedor_ni": fornNi,
				"qtd_contratos": qtd,
				"valor_min":     minv,
				"valor_max":     maxv,
				"ratio":         ratio,
				"valores":       valores,
				"explanation":   fmt.Sprintf("Este fornecedor tem %d contratos com o mesmo órgão, com valores indo de R$ %.0f a R$ %.0f (ratio %.1fx). Padrões crescentes podem indicar relação preferencial.", qtd, minv, maxv, ratio),
			},
			Link: fmt.Sprintf("/gastos/fornecedores/%s", fornNi),
		})
	}
	return out, rows.Err()
}
