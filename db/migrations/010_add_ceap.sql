-- Fé Pública migration 010 — R6: CEAP (Câmara) projection.
-- Cota para o Exercício da Atividade Parlamentar: cada nota fiscal de
-- cada deputado, com CNPJ do fornecedor, é pública pela Lei de Acesso.

BEGIN;

CREATE TABLE IF NOT EXISTS ceap (
    id              BIGSERIAL PRIMARY KEY,
    event_id        BIGINT NOT NULL REFERENCES events(id),
    snapshot_id     BIGINT NOT NULL REFERENCES snapshots(id),
    external_id     TEXT NOT NULL,

    deputado_id     INTEGER,
    deputado_nome   TEXT,
    partido         TEXT,
    uf              CHAR(2),

    ano             INTEGER,
    mes             INTEGER,
    data_documento  DATE,

    tipo_despesa    TEXT,
    fornecedor_cnpj TEXT,            -- já em digits-only
    fornecedor_nome TEXT,

    valor_documento NUMERIC(18, 2),
    valor_liquido   NUMERIC(18, 2),
    valor_glosa     NUMERIC(18, 2),
    url_documento   TEXT,

    collected_at    TIMESTAMPTZ NOT NULL,
    indexed_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ceap_deputado ON ceap (deputado_id) WHERE deputado_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_ceap_fornecedor ON ceap (fornecedor_cnpj) WHERE fornecedor_cnpj IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_ceap_data ON ceap (data_documento DESC NULLS LAST);
CREATE INDEX IF NOT EXISTS idx_ceap_valor ON ceap (valor_liquido DESC NULLS LAST);
CREATE INDEX IF NOT EXISTS idx_ceap_event_id ON ceap (event_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_ceap_external_snap ON ceap (snapshot_id, external_id);
CREATE INDEX IF NOT EXISTS idx_ceap_fornecedor_trgm ON ceap USING gin (fornecedor_nome gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_ceap_deputado_trgm ON ceap USING gin (deputado_nome gin_trgm_ops);

INSERT INTO sources (id, name, base_url, description) VALUES
  ('camara-ceap',
   'CEAP — Câmara dos Deputados',
   'https://dadosabertos.camara.leg.br/api/v2',
   'Cota para Exercício da Atividade Parlamentar — notas fiscais por deputado.')
ON CONFLICT (id) DO NOTHING;

COMMIT;
