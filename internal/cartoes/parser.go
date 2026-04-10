// Package cartoes parses Portal da Transparência CPGF JSON into a typed
// row ready for the cartoes projection table.
package cartoes

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Row is the projected shape of a CPGF transaction.
type Row struct {
	ExternalID      string
	TipoCartao      string
	MesExtrato      string
	DataTransacao   *time.Time
	ValorTransacao  *float64
	EstabCNPJ       string
	EstabNome       string
	EstabTipo       string
	PortadorCPF     string
	PortadorNome    string
	OrgaoCodigo     string
	OrgaoSigla      string
	OrgaoNome       string
	OrgaoMaxCodigo  string
	OrgaoMaxSigla   string
	OrgaoMaxNome    string
	UnidadeCodigo   string
	UnidadeNome     string
}

// cpgfShape captures the fields we read from the CPGF response.
type cpgfShape struct {
	ID             int64  `json:"id"`
	MesExtrato     string `json:"mesExtrato"`
	DataTransacao  string `json:"dataTransacao"`
	ValorTransacao string `json:"valorTransacao"`
	TipoCartao     *struct {
		Descricao string `json:"descricao"`
	} `json:"tipoCartao"`
	Estabelecimento *struct {
		CNPJFormatado string `json:"cnpjFormatado"`
		Nome          string `json:"nome"`
		Tipo          string `json:"tipo"`
	} `json:"estabelecimento"`
	Portador *struct {
		CPFFormatado string `json:"cpfFormatado"`
		Nome         string `json:"nome"`
	} `json:"portador"`
	UnidadeGestora *struct {
		Codigo        string `json:"codigo"`
		Nome          string `json:"nome"`
		OrgaoVinculado *struct {
			CodigoSIAFI string `json:"codigoSIAFI"`
			Sigla       string `json:"sigla"`
			Nome        string `json:"nome"`
		} `json:"orgaoVinculado"`
		OrgaoMaximo *struct {
			Codigo string `json:"codigo"`
			Sigla  string `json:"sigla"`
			Nome   string `json:"nome"`
		} `json:"orgaoMaximo"`
	} `json:"unidadeGestora"`
}

// Parse extracts a Row from raw CPGF JSON.
func Parse(raw []byte, externalID string) (*Row, error) {
	var src cpgfShape
	if err := json.Unmarshal(raw, &src); err != nil {
		return nil, fmt.Errorf("cartoes: parse cpgf: %w", err)
	}
	r := &Row{
		ExternalID: externalID,
		MesExtrato: src.MesExtrato,
	}
	if src.TipoCartao != nil {
		r.TipoCartao = src.TipoCartao.Descricao
	}
	if src.Estabelecimento != nil {
		r.EstabCNPJ = src.Estabelecimento.CNPJFormatado
		r.EstabNome = src.Estabelecimento.Nome
		r.EstabTipo = src.Estabelecimento.Tipo
	}
	if src.Portador != nil {
		r.PortadorCPF = src.Portador.CPFFormatado
		r.PortadorNome = src.Portador.Nome
	}
	if src.UnidadeGestora != nil {
		r.UnidadeCodigo = src.UnidadeGestora.Codigo
		r.UnidadeNome = src.UnidadeGestora.Nome
		if src.UnidadeGestora.OrgaoVinculado != nil {
			r.OrgaoCodigo = src.UnidadeGestora.OrgaoVinculado.CodigoSIAFI
			r.OrgaoSigla = src.UnidadeGestora.OrgaoVinculado.Sigla
			r.OrgaoNome = src.UnidadeGestora.OrgaoVinculado.Nome
		}
		if src.UnidadeGestora.OrgaoMaximo != nil {
			r.OrgaoMaxCodigo = src.UnidadeGestora.OrgaoMaximo.Codigo
			r.OrgaoMaxSigla = src.UnidadeGestora.OrgaoMaximo.Sigla
			r.OrgaoMaxNome = src.UnidadeGestora.OrgaoMaximo.Nome
		}
	}
	r.DataTransacao = parseDateBR(src.DataTransacao)
	r.ValorTransacao = parseValorBR(src.ValorTransacao)
	return r, nil
}

func parseDateBR(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse("02/01/2006", s)
	if err != nil {
		return nil
	}
	return &t
}

// parseValorBR parses a Brazilian-formatted decimal: "1.019,94" → 1019.94.
func parseValorBR(s string) *float64 {
	if s == "" {
		return nil
	}
	clean := strings.ReplaceAll(s, ".", "")
	clean = strings.ReplaceAll(clean, ",", ".")
	v, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		return nil
	}
	return &v
}
