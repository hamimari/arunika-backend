-- V62: Tumbuh Kembang growth tracking.
--
-- Replaces growth_records (V12): it had no position, no classification and no
-- delete, and no client ever called its /growth routes. Production held no
-- rows when this shipped.
DROP TABLE IF EXISTS growth_records;

-- One row per height/weight measurement. Age, z-scores and categories are
-- computed by the server (package growth) on every write, and recomputed when
-- the child's birth date or sex changes, so history never shifts silently.
CREATE TABLE growth_measurements (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    child_id      UUID         NOT NULL REFERENCES children(id) ON DELETE CASCADE,
    measured_on   DATE         NOT NULL,
    height_cm     NUMERIC(4,1) CHECK (height_cm IS NULL OR height_cm > 0),
    weight_kg     NUMERIC(4,1) CHECK (weight_kg IS NULL OR weight_kg > 0),
    position      TEXT         NOT NULL CHECK (position IN ('standing', 'recumbent')),
    age_days      INTEGER      NOT NULL,
    hfa_z         NUMERIC(5,2),
    wfa_z         NUMERIC(5,2),
    hfa_category  TEXT,
    wfa_category  TEXT,
    -- Outside WHO flag limits; saved only after the parent confirmed it.
    flagged       BOOLEAN      NOT NULL DEFAULT FALSE,
    who_version   TEXT         NOT NULL DEFAULT 'who2006',
    -- Idempotency key for retried (and, later, offline-queued) creates.
    client_id     UUID UNIQUE,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    -- Soft delete backs "Urungkan"; rows are purged 30 days later.
    deleted_at    TIMESTAMPTZ,
    CHECK (height_cm IS NOT NULL OR weight_kg IS NOT NULL)
);

CREATE INDEX idx_growth_measurements_child
    ON growth_measurements (child_id, measured_on DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_growth_measurements_deleted
    ON growth_measurements (deleted_at)
    WHERE deleted_at IS NOT NULL;

INSERT INTO app_feature_flags (key, name, description, is_enabled) VALUES
    ('growth_tracking', 'Tumbuh Kembang', 'Tumbuh tab and Beranda growth card.', FALSE)
ON CONFLICT (key) DO NOTHING;
