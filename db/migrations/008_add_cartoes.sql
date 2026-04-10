-- Fé Pública migration 008 — R2: cartoes-cpgf projection.
-- A denormalized projection of cartoes-cpgf events so we can aggregate
-- (top portadores, top órgãos, top estabelecimentos, time series)
-- without parsing JSONB on every query.
--
-- The table is populated by cmd/cartoes-indexer reading unindexed events
-- where source_id='cartoes-cpgf'. Like contratos, it's a projection — rebuild
-- is always possible from events.canonical_json.

BEGIN;

CREATE TABLE IF NOT EXISTS cartoes (
    -- Identity
    id                BIGSERIAL PRIMARY KEY,
    event_id          BIGINT NOT NULL REFERENCES events(id),
    snapshot_id       BIGINT NOT NULL REFERENCES snapshots(id),
    external_id       TEXT NOT NULL,

    -- Tipo de cartão (CPGF, CPGF-Compras Centralizadas, etc)
    tipo_cartao       TEXT,

    -- Transação
    mes_extrato       TEXT,           -- "06/2024"
    data_transacao    DATE,
    valor_transacao   NUMERIC(18, 2),

    -- Estabelecimento (quem recebeu)
    estab_cnpj        TEXT,
    estab_nome        TEXT,
    estab_tipo        TEXT,

    -- Portador (servidor que usou)
    portador_cpf      TEXT,           -- mascarado, pela LGPD
    portador_nome     TEXT,

    -- Órgão (quem pagou)
    orgao_codigo      TEXT,
    orgao_sigla       TEXT,
    orgao_nome        TEXT,
    orgao_max_codigo  TEXT,
    orgao_max_sigla   TEXT,
    orgao_max_nome    TEXT,
    unidade_codigo    TEXT,
    unidade_nome      TEXT,

    -- Bookkeeping
    collected_at      TIMESTAMPTZ NOT NULL,
    indexed_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for the common queries.
CREATE INDEX IF NOT EXISTS idx_cartoes_portador_cpf ON cartoes (portador_cpf) WHERE portador_cpf IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_cartoes_estab_cnpj ON cartoes (estab_cnpj) WHERE estab_cnpj IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_cartoes_orgao_max ON cartoes (orgao_max_codigo) WHERE orgao_max_codigo IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_cartoes_data ON cartoes (data_transacao DESC NULLS LAST);
CREATE INDEX IF NOT EXISTS idx_cartoes_valor ON cartoes (valor_transacao DESC NULLS LAST);
CREATE INDEX IF NOT EXISTS idx_cartoes_event_id ON cartoes (event_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_cartoes_external_snap ON cartoes (snapshot_id, external_id);

-- Trigram search on portador/estabelecimento names.
CREATE INDEX IF NOT EXISTS idx_cartoes_portador_trgm ON cartoes USING gin (portador_nome gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_cartoes_estab_trgm ON cartoes USING gin (estab_nome gin_trgm_ops);

INSERT INTO sources (id, name, base_url, description) VALUES
  ('cartoes-cpgf',
   'CPGF — Cartão de Pagamento do Governo Federal',
   'https://api.portaldatransparencia.gov.br/api-de-dados/cartoes',
   'Transações nos cartões corporativos da União, por mês de extrato.')
ON CONFLICT (id) DO NOTHING;

COMMIT;
