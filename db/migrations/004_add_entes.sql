-- Fé Pública migration 004 — Observatório M3: entes públicos brasileiros.
-- Adds the entes table + FK from change_events.ente_id to entes.id.

BEGIN;

CREATE TABLE IF NOT EXISTS entes (
    id              TEXT PRIMARY KEY,
    nome            TEXT NOT NULL,
    nome_curto      TEXT,
    esfera          TEXT NOT NULL CHECK (esfera IN ('federal','estadual','municipal','distrital')),
    tipo            TEXT NOT NULL CHECK (tipo IN ('uniao','poder','ministerio','autarquia','empresa','uf','municipio','camara','tribunal','mp','outro')),
    poder           TEXT CHECK (poder IN ('executivo','legislativo','judiciario','mp','tc','defensoria')),
    uf              CHAR(2),
    ibge_code       TEXT,
    cnpj            TEXT,
    populacao       INT,
    domain_hint     TEXT,
    parent_id       TEXT REFERENCES entes(id),
    tier            INT NOT NULL DEFAULT 4,
    active          BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_entes_esfera_uf ON entes (esfera, uf);
CREATE INDEX IF NOT EXISTS idx_entes_tier_active ON entes (tier) WHERE active = TRUE;
CREATE INDEX IF NOT EXISTS idx_entes_ibge_code ON entes (ibge_code);

-- Add FK from change_events.ente_id now that entes table exists.
-- Pre-existing rows have NULL ente_id which is allowed.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE constraint_name = 'change_events_ente_id_fkey'
        AND table_name = 'change_events'
    ) THEN
        ALTER TABLE change_events
            ADD CONSTRAINT change_events_ente_id_fkey
            FOREIGN KEY (ente_id) REFERENCES entes(id);
    END IF;
END$$;

COMMIT;
