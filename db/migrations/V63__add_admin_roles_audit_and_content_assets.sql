-- V63: content roles for admins, an admin audit log, and uploaded media assets.

-- Existing admins become publishers so nobody loses access.
ALTER TABLE admin_users
    ADD COLUMN role TEXT NOT NULL DEFAULT 'publisher'
        CHECK (role IN ('editor', 'publisher'));

CREATE TABLE admin_audit_log (
    id          BIGSERIAL PRIMARY KEY,
    admin_id    UUID        NOT NULL REFERENCES admin_users(id),
    action      TEXT        NOT NULL,
    entity_type TEXT        NOT NULL,
    entity_id   TEXT        NOT NULL,
    before      JSONB,
    after       JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_admin_audit_log_entity
    ON admin_audit_log (entity_type, entity_id, created_at DESC);

-- Files uploaded through POST /admin/assets, stored content-addressed by sha256.
CREATE TABLE content_assets (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    kind          TEXT        NOT NULL CHECK (kind IN ('image', 'audio')),
    storage_key   TEXT        NOT NULL,
    sha256        TEXT        NOT NULL UNIQUE,
    bytes         INTEGER     NOT NULL,
    mime          TEXT        NOT NULL,
    width         INTEGER,
    height        INTEGER,
    duration_ms   INTEGER,
    original_name TEXT        NOT NULL DEFAULT '',
    created_by    UUID        NOT NULL REFERENCES admin_users(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
