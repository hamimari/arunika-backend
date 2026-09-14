CREATE TABLE user_entitlements (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id         UUID NOT NULL REFERENCES parents(id),
  product_id      UUID NOT NULL REFERENCES products(id),
  starts_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  expires_at      TIMESTAMPTZ NULL,
  source_order_id UUID NULL REFERENCES orders(id),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, product_id)
);

CREATE INDEX idx_user_entitlements_user_id ON user_entitlements(user_id);
