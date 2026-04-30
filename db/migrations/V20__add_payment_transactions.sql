-- V20: Add payment_transactions table to record every Midtrans webhook callback

CREATE TABLE payment_transactions (
    id                  UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_id            VARCHAR(100) NOT NULL,
    user_id             UUID         REFERENCES parents(id) ON DELETE SET NULL,
    transaction_id      VARCHAR(100),
    transaction_status  VARCHAR(50)  NOT NULL,
    payment_type        VARCHAR(50),
    gross_amount        VARCHAR(20),
    status_code         VARCHAR(10),
    fraud_status        VARCHAR(20),
    raw_payload         JSONB        NOT NULL DEFAULT '{}',
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payment_transactions_order_id           ON payment_transactions(order_id);
CREATE INDEX idx_payment_transactions_user_id            ON payment_transactions(user_id);
CREATE INDEX idx_payment_transactions_transaction_status ON payment_transactions(transaction_status);
CREATE INDEX idx_payment_transactions_created_at         ON payment_transactions(created_at);
