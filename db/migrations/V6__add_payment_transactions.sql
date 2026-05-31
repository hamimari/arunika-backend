-- Midtrans payment transactions table
CREATE TABLE IF NOT EXISTS payment_transactions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID         NOT NULL,
    order_id        VARCHAR(100) NOT NULL UNIQUE,   -- Midtrans order_id (uuid-based)
    plan_name       VARCHAR(255) NOT NULL,
    amount          BIGINT       NOT NULL,
    snap_token      TEXT         NOT NULL DEFAULT '',
    payment_type    VARCHAR(100) NOT NULL DEFAULT '',
    status          VARCHAR(50)  NOT NULL DEFAULT 'pending', -- pending|success|failed|expired
    midtrans_status VARCHAR(50)  NOT NULL DEFAULT '',
    paid_at         TIMESTAMP    NULL,
    created_at      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    is_deleted      BOOLEAN      NOT NULL DEFAULT false
);

CREATE INDEX IF NOT EXISTS idx_payment_transactions_user_id  ON payment_transactions(user_id);
CREATE INDEX IF NOT EXISTS idx_payment_transactions_order_id ON payment_transactions(order_id);
