-- Fé Pública migration 007 — R1: contratos projection.
-- A denormalized projection of pncp-contratos events so we can aggregate
-- (top fornecedores, top órgãos, time series of values, search by CNPJ)
-- without parsing JSONB on every query.
--
-- The table is populated by cmd/contratos-indexer reading unindexed events
-- where source_id='pncp-contratos'. It's a projection, not a source of truth
-- — rebuild is always possible from events.canonical_json.

BEGIN;

CREATE TABLE IF NOT EXISTS contratos (
    -- Identity
    id                    BIGSERIAL PRIMARY KEY,
    event_id              BIGINT NOT NULL REFERENCES events(id),
    snapshot_id           BIGINT NOT NULL REFERENCES snapshots(id),
    external_id           TEXT NOT NULL,
    numero_controle_pncp  TEXT,

    -- Órgão comprador
    orgao_cnpj            TEXT,
    orgao_razao_social    TEXT,
    orgao_poder_id        TEXT,
    orgao_esfera_id       TEXT,
    uf                    CHAR(2),

    -- Fornecedor
    fornecedor_ni         TEXT,           -- CPF/CNPJ
    fornecedor_nome       TEXT,
    fornecedor_tipo       TEXT,           -- PJ/PF

    -- Valor e vigência
    valor_inicial         NUMERIC(18, 2),
    valor_global          NUMERIC(18, 2),
    valor_acumulado       NUMERIC(18, 2),
    data_assinatura       DATE,
    data_vigencia_inicio  DATE,
    data_vigencia_fim     DATE,
    data_publicacao_pncp  TIMESTAMPTZ,

    -- Objeto
    objeto_contrato       TEXT,
    tipo_contrato         TEXT,
    categoria_processo    TEXT,

    -- Bookkeeping
    collected_at          TIMESTAMPTZ NOT NULL,
    indexed_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for the common queries.
CREATE INDEX IF NOT EXISTS idx_contratos_fornecedor_ni ON contratos (fornecedor_ni) WHERE fornecedor_ni IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_contratos_orgao_cnpj ON contratos (orgao_cnpj) WHERE orgao_cnpj IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_contratos_uf ON contratos (uf) WHERE uf IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_contratos_data_assinatura ON contratos (data_assinatura DESC NULLS LAST);
CREATE INDEX IF NOT EXISTS idx_contratos_valor_global ON contratos (valor_global DESC NULLS LAST);
CREATE INDEX IF NOT EXISTS idx_contratos_event_id ON contratos (event_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_contratos_external_snap ON contratos (snapshot_id, external_id);

-- Trigram search on fornecedor/órgão names for "buscar por nome".
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX IF NOT EXISTS idx_contratos_fornecedor_nome_trgm ON contratos USING gin (fornecedor_nome gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_contratos_orgao_nome_trgm ON contratos USING gin (orgao_razao_social gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_contratos_objeto_trgm ON contratos USING gin (objeto_contrato gin_trgm_ops);

COMMIT;
