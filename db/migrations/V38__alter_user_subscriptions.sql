-- Evolve user_subscriptions in place as the single source of truth for
-- subscription-type access (see design: standardizing on this table rather
-- than introducing a third parallel one, since `subscriptions` (V1) is
-- already fully dead).
ALTER TABLE user_subscriptions
  ADD COLUMN package_id UUID NULL REFERENCES premium_packages(id),
  ADD COLUMN start_date TIMESTAMPTZ NULL,
  ADD COLUMN auto_renew BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE user_subscriptions RENAME COLUMN midtrans_order_id TO provider_order_id;
