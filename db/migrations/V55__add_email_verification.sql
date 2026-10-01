-- Email verification: prove a registrant controls the address on their
-- account, without ever blocking registration on mail delivery.

-- Add the column and grandfather existing accounts as one atomic step.
--
-- The grandfathering must happen at exactly the moment the column is created,
-- and never again. A bare "UPDATE parents SET email_verified = true" outside
-- this guard would, on any re-run, silently mark every genuinely unverified
-- account as verified — handing password recovery to addresses nobody has
-- proven they control. Flyway does not re-run versioned migrations, but this
-- must not depend on that: test harnesses and manual recovery steps do
-- re-apply migrations.
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
     WHERE table_name = 'parents' AND column_name = 'email_verified'
  ) THEN
    ALTER TABLE parents
      ADD COLUMN email_verified BOOLEAN NOT NULL DEFAULT false;

    -- Every account that predates this feature keeps password recovery.
    -- Letting the false default stand would instead withdraw account recovery
    -- from the entire live user base the moment the reset gate ships —
    -- locking current users out to solve a problem they do not have.
    --
    -- The deliberate consequence: these addresses are not actually verified.
    -- The guarantee is forward-looking, not universal. Any retrofit must be
    -- an opt-in campaign, never a migration default.
    UPDATE parents SET email_verified = true;
  END IF;
END $$;

-- Mirrors password_reset_tokens: only a SHA-256 hash of the emailed token is
-- stored, so reading this table alone never yields a usable verification
-- link. Tokens are single-use (deleted on consumption) and superseded on
-- reissue (prior rows deleted for that user).
CREATE TABLE IF NOT EXISTS email_verification_tokens (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    UUID NOT NULL REFERENCES parents(id) ON DELETE CASCADE,
  token      VARCHAR(64) NOT NULL UNIQUE,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  is_deleted BOOLEAN NOT NULL DEFAULT false
);

CREATE INDEX IF NOT EXISTS idx_email_verification_tokens_user_id
  ON email_verification_tokens(user_id);
