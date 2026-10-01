-- Append-only record of which legal document version a parent accepted, and
-- when (UU PDP Art. 24: the controller must be able to prove consent). One
-- row per document per acceptance; the latest row per document is current.
--
-- Not ON DELETE CASCADE: account deletion anonymises the parents row instead
-- of removing it, so AccountDeletionService deletes these rows explicitly.
CREATE TABLE IF NOT EXISTS user_consents (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL REFERENCES parents(id),
    document    VARCHAR(20) NOT NULL CHECK (document IN ('TERMS', 'PRIVACY', 'PARENTAL')),
    version     VARCHAR(20) NOT NULL,
    accepted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ip_address  VARCHAR(64),
    user_agent  VARCHAR(512)
);

CREATE INDEX IF NOT EXISTS idx_user_consents_user_doc
    ON user_consents (user_id, document, accepted_at DESC);
