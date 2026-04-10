// Package ceap parses CEAP (Câmara) JSON into a typed row.
//
// Note: the Câmara API doesn't return the deputado nome/partido/uf in the
// despesa payload — only the deputado ID. The collector embeds the deputado
// info into the canonical_json by wrapping each record with metadata. We
// keep the parser tolerant: if those fields are missing the projection
// just stores the IDs.
package ceap

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"
)

type Row struct {
	ExternalID     string
	DeputadoID     int
	DeputadoNome   string
	Partido        string
	UF             string
	Ano            int
	Mes            int
	DataDocumento  *time.Time
	TipoDespesa    string
	FornecedorCNPJ string
	FornecedorNome string
	ValorDocumento *float64
	ValorLiquido   *float64
	ValorGlosa     *float64
	URLDocumento   string
}

type ceapShape struct {
	Ano               int             `json:"ano"`
	Mes               int             `json:"mes"`
	DataDocumento     string          `json:"dataDocumento"`
	TipoDespesa       string          `json:"tipoDespesa"`
	NomeFornecedor    string          `json:"nomeFornecedor"`
	CnpjCpfFornecedor string          `json:"cnpjCpfFornecedor"`
	ValorDocumento    *float64        `json:"valorDocumento"`
	ValorLiquido      *float64        `json:"valorLiquido"`
	ValorGlosa        *float64        `json:"valorGlosa"`
	URLDocumento      string          `json:"urlDocumento"`
	CodDocumento      json.RawMessage `json:"codDocumento"`
	// Custom fields injected by the collector wrapper:
	DeputadoID    int    `json:"_deputadoId,omitempty"`
	DeputadoNome  string `json:"_deputadoNome,omitempty"`
	DeputadoSigla string `json:"_deputadoPartido,omitempty"`
	DeputadoUF    string `json:"_deputadoUf,omitempty"`
}

var nonDigit = regexp.MustCompile(`[^0-9]`)

func Parse(raw []byte, externalID string) (*Row, error) {
	var src ceapShape
	if err := json.Unmarshal(raw, &src); err != nil {
		return nil, fmt.Errorf("ceap: parse: %w", err)
	}
	r := &Row{
		ExternalID:     externalID,
		DeputadoID:     src.DeputadoID,
		DeputadoNome:   src.DeputadoNome,
		Partido:        src.DeputadoSigla,
		UF:             src.DeputadoUF,
		Ano:            src.Ano,
		Mes:            src.Mes,
		TipoDespesa:    src.TipoDespesa,
		FornecedorNome: src.NomeFornecedor,
		FornecedorCNPJ: nonDigit.ReplaceAllString(src.CnpjCpfFornecedor, ""),
		ValorDocumento: src.ValorDocumento,
		ValorLiquido:   src.ValorLiquido,
		ValorGlosa:     src.ValorGlosa,
		URLDocumento:   src.URLDocumento,
	}
	if src.DataDocumento != "" {
		// "2025-01-08T00:00:00"
		if t, err := time.Parse("2006-01-02T15:04:05", src.DataDocumento); err == nil {
			r.DataDocumento = &t
		} else if t, err := time.Parse("2006-01-02", src.DataDocumento); err == nil {
			r.DataDocumento = &t
		}
	}
	return r, nil
}
