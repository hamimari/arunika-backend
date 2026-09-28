-- Every refund of a Google Play order: issued by an admin from the
-- backoffice (source ADMIN), or reported by Google on its own via the
-- voided-purchases API (GOOGLE_VOIDED) or an RTDN revocation (GOOGLE_RTDN).
-- Google's refund calls return nothing, so the amounts come from a follow-up
-- orders.get and may be filled in later.
CREATE TABLE IF NOT EXISTS order_refunds (
    id                 UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id           UUID        NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    provider           VARCHAR(20) NOT NULL DEFAULT 'google_play' CHECK (provider IN ('google_play')),
    source             VARCHAR(20) NOT NULL CHECK (source IN ('ADMIN', 'GOOGLE_VOIDED', 'GOOGLE_RTDN')),
    refund_type        VARCHAR(10) NOT NULL DEFAULT 'FULL' CHECK (refund_type IN ('FULL', 'PRORATED')),
    revoked            BOOLEAN     NOT NULL DEFAULT TRUE,
    reason             TEXT,
    admin_id           UUID,
    play_order_id      TEXT,
    purchase_token     TEXT,
    order_amount_idr   BIGINT      NOT NULL,
    refunded_total     NUMERIC(18, 2),
    refunded_tax       NUMERIC(18, 2),
    currency           VARCHAR(3),
    play_order_state   VARCHAR(30),
    play_refund_reason VARCHAR(30),
    voided_source      INTEGER,
    voided_reason      INTEGER,
    status             VARCHAR(10) NOT NULL CHECK (status IN ('REQUESTED', 'SUCCEEDED', 'FAILED')),
    error              TEXT,
    raw_response       JSONB,
    requested_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at       TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT order_refunds_admin_reason CHECK (source <> 'ADMIN' OR (reason IS NOT NULL AND admin_id IS NOT NULL))
);

CREATE INDEX IF NOT EXISTS idx_order_refunds_order_id ON order_refunds (order_id);

-- At most one refund in progress or completed per order; FAILED attempts
-- don't count, so an admin can retry.
CREATE UNIQUE INDEX IF NOT EXISTS uq_order_refunds_active
    ON order_refunds (order_id) WHERE status IN ('REQUESTED', 'SUCCEEDED');
