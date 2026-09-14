-- Orders: created before a Midtrans transaction, drives payment settlement
-- and entitlement granting. Exactly one of product_id/package_id is set.
CREATE TABLE orders (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    UUID NOT NULL REFERENCES parents(id),
  product_id UUID NULL REFERENCES products(id),
  package_id UUID NULL REFERENCES premium_packages(id),
  amount_idr BIGINT NOT NULL,
  status     VARCHAR(20) NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','PAID','FAILED','EXPIRED')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK ((product_id IS NOT NULL)::int + (package_id IS NOT NULL)::int = 1)
);

CREATE INDEX idx_orders_user_id ON orders(user_id);

-- payment_transactions (V6) never actually matched the columns the app code
-- writes/reads (models/payment_transaction.go expects transaction_id,
-- transaction_status, gross_amount, status_code, fraud_status, raw_payload —
-- none of which V6 created; those only ever existed in the commented-out
-- V20). This is pre-launch, so rather than pretend to migrate rows out of a
-- table shape the code never actually matched, replace it outright with the
-- shape the code has always expected, plus the new order linkage.
--
-- The pre-existing `order_id` column held Midtrans's own order identifier
-- string; renamed to provider_order_id (mirroring the same
-- midtrans_order_id -> provider_order_id rename on user_subscriptions) to
-- free up `order_id` for the new internal FK to orders(id).
DROP TABLE IF EXISTS payment_transactions;

CREATE TABLE payments (
  id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id           UUID NOT NULL REFERENCES orders(id),
  provider_order_id  VARCHAR(100) NOT NULL,
  user_id            UUID REFERENCES parents(id),
  transaction_id     VARCHAR(100),
  transaction_status VARCHAR(50) NOT NULL,
  payment_type       VARCHAR(50),
  gross_amount       VARCHAR(20),
  status_code        VARCHAR(10),
  fraud_status       VARCHAR(20),
  raw_payload        JSONB NOT NULL DEFAULT '{}',
  created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payments_order_id ON payments(order_id);
CREATE INDEX idx_payments_user_id ON payments(user_id);
CREATE INDEX idx_payments_provider_order_id ON payments(provider_order_id);
