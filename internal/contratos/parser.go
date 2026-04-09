// Package contratos parses PNCP canonical JSON into a typed contract row
// ready to be inserted into the contratos projection table.
//
// The shape of the input JSON is determined by the PNCP consulta v1 API
// (/api/consulta/v1/contratos). We only extract fields we plan to query on;
// the full canonical_json stays in events, so anything we need later can
// be recovered by re-parsing.
package contratos

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Row is the projected shape of a PNCP contract.
type Row struct {
	ExternalID          string
	NumeroControlePNCP  string
	OrgaoCNPJ           string
	OrgaoRazaoSocial    string
	OrgaoPoderID        string
	OrgaoEsferaID       string
	UF                  string
	FornecedorNI        string
	FornecedorNome      string
	FornecedorTipo      string
	ValorInicial        *float64
	ValorGlobal         *float64
	ValorAcumulado      *float64
	DataAssinatura      *time.Time
	DataVigenciaInicio  *time.Time
	DataVigenciaFim     *time.Time
	DataPublicacaoPNCP  *time.Time
	ObjetoContrato      string
	TipoContrato        string
	CategoriaProcesso   string
}

// pncpShape captures the fields we read from PNCP canonical_json.
type pncpShape struct {
	NumeroControlePncpCompra string  `json:"numeroControlePncpCompra"`
	DataAtualizacao          string  `json:"dataAtualizacao"`
	DataAssinatura           string  `json:"dataAssinatura"`
	DataVigenciaInicio       string  `json:"dataVigenciaInicio"`
	DataVigenciaFim          string  `json:"dataVigenciaFim"`
	NiFornecedor             string  `json:"niFornecedor"`
	NomeRazaoSocialFornecedor string `json:"nomeRazaoSocialFornecedor"`
	TipoPessoa               string  `json:"tipoPessoa"`
	ObjetoContrato           string  `json:"objetoContrato"`
	TipoContrato             *struct {
		Nome string `json:"nome"`
	} `json:"tipoContrato"`
	CategoriaProcesso *struct {
		Nome string `json:"nome"`
	} `json:"categoriaProcesso"`
	ValorInicial   *float64 `json:"valorInicial"`
	ValorGlobal    *float64 `json:"valorGlobal"`
	ValorAcumulado *float64 `json:"valorAcumulado"`
	OrgaoEntidade  *struct {
		CNPJ         string `json:"cnpj"`
		RazaoSocial  string `json:"razaoSocial"`
		PoderID      string `json:"poderId"`
		EsferaID     string `json:"esferaId"`
	} `json:"orgaoEntidade"`
	UnidadeOrgao *struct {
		UFSigla string `json:"ufSigla"`
	} `json:"unidadeOrgao"`
}

// Parse extracts a Row from raw PNCP JSON.
func Parse(raw []byte, externalID string) (*Row, error) {
	var src pncpShape
	if err := json.Unmarshal(raw, &src); err != nil {
		return nil, fmt.Errorf("contratos: parse pncp: %w", err)
	}
	r := &Row{
		ExternalID:          externalID,
		NumeroControlePNCP:  src.NumeroControlePncpCompra,
		FornecedorNI:        src.NiFornecedor,
		FornecedorNome:      src.NomeRazaoSocialFornecedor,
		FornecedorTipo:      src.TipoPessoa,
		ObjetoContrato:      src.ObjetoContrato,
		ValorInicial:        src.ValorInicial,
		ValorGlobal:         src.ValorGlobal,
		ValorAcumulado:      src.ValorAcumulado,
	}
	if src.OrgaoEntidade != nil {
		r.OrgaoCNPJ = src.OrgaoEntidade.CNPJ
		r.OrgaoRazaoSocial = src.OrgaoEntidade.RazaoSocial
		r.OrgaoPoderID = src.OrgaoEntidade.PoderID
		r.OrgaoEsferaID = src.OrgaoEntidade.EsferaID
	}
	if src.UnidadeOrgao != nil {
		r.UF = strings.ToUpper(src.UnidadeOrgao.UFSigla)
	}
	if src.TipoContrato != nil {
		r.TipoContrato = src.TipoContrato.Nome
	}
	if src.CategoriaProcesso != nil {
		r.CategoriaProcesso = src.CategoriaProcesso.Nome
	}

	// Dates: PNCP uses ISO date and sometimes ISO datetime.
	r.DataAssinatura = parseFlexibleDate(src.DataAssinatura)
	r.DataVigenciaInicio = parseFlexibleDate(src.DataVigenciaInicio)
	r.DataVigenciaFim = parseFlexibleDate(src.DataVigenciaFim)
	r.DataPublicacaoPNCP = parseFlexibleDate(src.DataAtualizacao)

	return r, nil
}

func parseFlexibleDate(s string) *time.Time {
	if s == "" {
		return nil
	}
	layouts := []string{
		"2006-01-02",
		"2006-01-02T15:04:05",
		time.RFC3339,
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return &t
		}
	}
	return nil
}
