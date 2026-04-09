// Package entes loads and manages the list of Brazilian public entities.
//
// The source list is built from three inputs:
//
//  1. Hardcoded states (27 UFs)
//  2. A curated YAML file of federal entities (db/seeds/entes-federal.yaml)
//  3. IBGE's municipalities API (~5570 rows, fetched over HTTP)
//
// The package exposes a Seeder that loads all three and upserts them into
// the store. It is designed to be idempotent: running it multiple times
// produces the same state, and running it after a YAML edit applies the
// diff without clobbering unrelated rows.
package entes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/gmowses/fepublica/internal/store"
)

// IBGEMunicipio is the minimal shape we consume from the IBGE API.
type ibgeMunicipio struct {
	ID         int    `json:"id"`
	Nome       string `json:"nome"`
	Microrregiao struct {
		Mesorregiao struct {
			UF struct {
				Sigla string `json:"sigla"`
			} `json:"UF"`
		} `json:"mesorregiao"`
	} `json:"microrregiao"`
}

// ibgeEndpoint returns the list of all Brazilian municipalities.
const ibgeEndpoint = "https://servicodados.ibge.gov.br/api/v1/localidades/municipios?orderBy=nome"

// Seeder runs all three sources and upserts into the store.
type Seeder struct {
	store  *store.Store
	logger zerolog.Logger
}

// NewSeeder builds a Seeder.
func NewSeeder(s *store.Store, logger zerolog.Logger) *Seeder {
	return &Seeder{store: s, logger: logger}
}

// Run seeds all three sources in order: UFs → federal YAML → IBGE municipalities.
// federalYAMLPath is the path to entes-federal.yaml (relative or absolute).
func (s *Seeder) Run(ctx context.Context, federalYAMLPath string) error {
	s.logger.Info().Msg("entes: seeding UFs")
	if err := s.seedUFs(ctx); err != nil {
		return fmt.Errorf("seed ufs: %w", err)
	}

	if federalYAMLPath != "" {
		s.logger.Info().Str("path", federalYAMLPath).Msg("entes: seeding federal YAML")
		if err := s.seedFederalYAML(ctx, federalYAMLPath); err != nil {
			return fmt.Errorf("seed federal yaml: %w", err)
		}
	}

	s.logger.Info().Msg("entes: fetching and seeding IBGE municipalities")
	if err := s.seedMunicipalities(ctx); err != nil {
		return fmt.Errorf("seed municipalities: %w", err)
	}

	return nil
}

// seedUFs inserts the 26 states + DF.
func (s *Seeder) seedUFs(ctx context.Context) error {
	ufs := []struct {
		Sigla     string
		Nome      string
		IBGECode  string
	}{
		{"AC", "Acre", "12"},
		{"AL", "Alagoas", "27"},
		{"AP", "Amapá", "16"},
		{"AM", "Amazonas", "13"},
		{"BA", "Bahia", "29"},
		{"CE", "Ceará", "23"},
		{"DF", "Distrito Federal", "53"},
		{"ES", "Espírito Santo", "32"},
		{"GO", "Goiás", "52"},
		{"MA", "Maranhão", "21"},
		{"MT", "Mato Grosso", "51"},
		{"MS", "Mato Grosso do Sul", "50"},
		{"MG", "Minas Gerais", "31"},
		{"PA", "Pará", "15"},
		{"PB", "Paraíba", "25"},
		{"PR", "Paraná", "41"},
		{"PE", "Pernambuco", "26"},
		{"PI", "Piauí", "22"},
		{"RJ", "Rio de Janeiro", "33"},
		{"RN", "Rio Grande do Norte", "24"},
		{"RS", "Rio Grande do Sul", "43"},
		{"RO", "Rondônia", "11"},
		{"RR", "Roraima", "14"},
		{"SC", "Santa Catarina", "42"},
		{"SP", "São Paulo", "35"},
		{"SE", "Sergipe", "28"},
		{"TO", "Tocantins", "17"},
	}
	for _, u := range ufs {
		esfera := "estadual"
		tipo := "uf"
		if u.Sigla == "DF" {
			esfera = "distrital"
		}
		err := s.store.UpsertEnte(ctx, store.UpsertEnteParams{
			ID:        "uf:" + strings.ToLower(u.Sigla),
			Nome:      u.Nome,
			NomeCurto: u.Sigla,
			Esfera:    esfera,
			Tipo:      tipo,
			UF:        u.Sigla,
			IBGECode:  u.IBGECode,
			ParentID:  "fed:uniao",
			Tier:      1,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// seedFederalYAML is a placeholder: it reads the file and uses the simple
// line-based parser below because we don't want to depend on a yaml library
// for this one-off case. For anything more complex we'd add gopkg.in/yaml.v3.
func (s *Seeder) seedFederalYAML(ctx context.Context, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	records, err := parseSimpleYAML(string(data))
	if err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}
	for _, r := range records {
		p := store.UpsertEnteParams{
			ID:         r["id"],
			Nome:       r["nome"],
			NomeCurto:  r["nome_curto"],
			Esfera:     r["esfera"],
			Tipo:       r["tipo"],
			Poder:      r["poder"],
			UF:         strings.ToUpper(r["uf"]),
			IBGECode:   r["ibge_code"],
			CNPJ:       r["cnpj"],
			DomainHint: r["domain_hint"],
			ParentID:   r["parent_id"],
			Tier:       1,
		}
		if t := r["tier"]; t != "" {
			switch t {
			case "1":
				p.Tier = 1
			case "2":
				p.Tier = 2
			case "3":
				p.Tier = 3
			default:
				p.Tier = 4
			}
		}
		if err := s.store.UpsertEnte(ctx, p); err != nil {
			return fmt.Errorf("upsert %s: %w", r["id"], err)
		}
	}
	return nil
}

// seedMunicipalities fetches all 5570 municipalities from IBGE and upserts them.
// Capital municipalities get tier 2; the rest start as tier 4 (the LAI crawler
// will promote them later based on population and political relevance).
func (s *Seeder) seedMunicipalities(ctx context.Context) error {
	httpClient := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ibgeEndpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "fepublica/0.1 (+https://github.com/gmowses/fepublica)")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetch ibge: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ibge status %d", resp.StatusCode)
	}

	var list []ibgeMunicipio
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return fmt.Errorf("decode ibge: %w", err)
	}
	s.logger.Info().Int("count", len(list)).Msg("entes: upserting municipalities")

	for _, m := range list {
		id := fmt.Sprintf("mun:%d", m.ID)
		uf := m.Microrregiao.Mesorregiao.UF.Sigla
		tier := 4
		if _, isCapital := capitals[m.ID]; isCapital {
			tier = 2
		}
		err := s.store.UpsertEnte(ctx, store.UpsertEnteParams{
			ID:       id,
			Nome:     m.Nome,
			Esfera:   "municipal",
			Tipo:     "municipio",
			UF:       uf,
			IBGECode: fmt.Sprintf("%d", m.ID),
			ParentID: "uf:" + strings.ToLower(uf),
			Tier:     tier,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// capitals: IBGE codes of the 27 state capitals (lookup for tier promotion).
var capitals = map[int]bool{
	1200401: true, // Rio Branco
	2704302: true, // Maceió
	1600303: true, // Macapá
	1302603: true, // Manaus
	2927408: true, // Salvador
	2304400: true, // Fortaleza
	5300108: true, // Brasília
	3205309: true, // Vitória
	5208707: true, // Goiânia
	2111300: true, // São Luís
	5103403: true, // Cuiabá
	5002704: true, // Campo Grande
	3106200: true, // Belo Horizonte
	1501402: true, // Belém
	2507507: true, // João Pessoa
	4106902: true, // Curitiba
	2611606: true, // Recife
	2211001: true, // Teresina
	3304557: true, // Rio de Janeiro
	2408102: true, // Natal
	4314902: true, // Porto Alegre
	1100205: true, // Porto Velho
	1400100: true, // Boa Vista
	4205407: true, // Florianópolis
	3550308: true, // São Paulo
	2800308: true, // Aracaju
	1721000: true, // Palmas
}
