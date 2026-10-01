-- Push/email promo campaigns sent from the backoffice. Dispatch runs in the
-- background, so the row doubles as a progress/history record: status moves
-- SENDING -> COMPLETED (or FAILED) and sent/failed counters are updated as
-- delivery proceeds.
CREATE TABLE IF NOT EXISTS campaigns (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    title        VARCHAR(200) NOT NULL,
    body         TEXT         NOT NULL,
    image_url    TEXT         NOT NULL DEFAULT '',
    channel      VARCHAR(10)  NOT NULL CHECK (channel IN ('push', 'email', 'both')),
    segment      VARCHAR(20)  NOT NULL CHECK (segment IN ('all_devices', 'all', 'subscribers')),
    link_type    VARCHAR(20)  NOT NULL DEFAULT 'none' CHECK (link_type IN ('none', 'ar_card', 'dongeng')),
    link_id      VARCHAR(100) NOT NULL DEFAULT '',
    status       VARCHAR(20)  NOT NULL DEFAULT 'SENDING' CHECK (status IN ('SENDING', 'COMPLETED', 'FAILED')),
    sent         INT          NOT NULL DEFAULT 0,
    failed       INT          NOT NULL DEFAULT 0,
    error        TEXT         NOT NULL DEFAULT '',
    created_by   UUID         NULL,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ  NULL
);

CREATE INDEX IF NOT EXISTS idx_campaigns_created_at ON campaigns(created_at DESC);
