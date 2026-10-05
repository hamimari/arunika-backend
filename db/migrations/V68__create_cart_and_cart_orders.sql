-- Keranjang Belanja: a per-account cart, multi-item ("cart") orders paid
-- with one consumable Google Play product per possible total, and the
-- Rp 1.000 price step that keeps the list of totals short.

-- Item prices must be whole Rp 1.000 so every cart total maps to one store
-- product. Fail loudly (naming the offenders) instead of rounding a price.
DO $$
DECLARE
    bad TEXT;
BEGIN
    SELECT string_agg(id::text || '=' || price_idr::text, ', ')
      INTO bad
      FROM products
     WHERE price_idr % 1000 <> 0;
    IF bad IS NOT NULL THEN
        RAISE EXCEPTION 'products with a price that is not a whole Rp 1.000: %', bad;
    END IF;
END $$;

ALTER TABLE products
    ADD CONSTRAINT products_price_step CHECK (price_idr % 1000 = 0);

-- One cart per account; an item is in it at most once.
-- seen_price_idr is the price the parent last confirmed, so a later price
-- rise can be shown as "Harga naik dari ...".
CREATE TABLE cart_items (
    user_id        UUID        NOT NULL REFERENCES parents(id) ON DELETE CASCADE,
    product_id     UUID        NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    seen_price_idr BIGINT      NOT NULL,
    added_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, product_id)
);

-- One consumable store product per chargeable total (arunika.cart.t<rupiah>).
CREATE TABLE store_products (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    store           VARCHAR(10) NOT NULL DEFAULT 'google' CHECK (store IN ('google')),
    play_product_id TEXT        NOT NULL UNIQUE,
    price_idr       BIGINT      NOT NULL CHECK (price_idr > 0 AND price_idr % 1000 = 0),
    active          BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (store, price_idr)
);

-- Cart orders carry their items in order_items instead of product_id /
-- package_id. granted_at marks the grant; consumed_at the server-side
-- consume of the Play purchase (retried until it succeeds).
ALTER TABLE orders
    ADD COLUMN is_cart            BOOLEAN     NOT NULL DEFAULT FALSE,
    ADD COLUMN subtotal_idr       BIGINT,
    ADD COLUMN charge_idr         BIGINT,
    ADD COLUMN price_locked_until TIMESTAMPTZ,
    ADD COLUMN store_product_id   UUID REFERENCES store_products(id),
    ADD COLUMN idempotency_key    VARCHAR(100),
    ADD COLUMN granted_at         TIMESTAMPTZ,
    ADD COLUMN consumed_at        TIMESTAMPTZ;

ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_check;
ALTER TABLE orders ADD CONSTRAINT orders_target_check CHECK (
    CASE WHEN is_cart
         THEN product_id IS NULL AND package_id IS NULL AND store_product_id IS NOT NULL
         ELSE (product_id IS NOT NULL)::int + (package_id IS NOT NULL)::int = 1
    END
);

CREATE UNIQUE INDEX uq_orders_user_idempotency_key
    ON orders (user_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX idx_orders_cart_unsettled
    ON orders (created_at) WHERE is_cart AND status = 'PENDING' AND purchase_token IS NOT NULL;
CREATE INDEX idx_orders_cart_unconsumed
    ON orders (granted_at) WHERE is_cart AND granted_at IS NOT NULL AND consumed_at IS NULL;

UPDATE orders SET granted_at = updated_at WHERE status IN ('PAID', 'REFUNDED');

CREATE TABLE order_items (
    order_id   UUID   NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id UUID   NOT NULL REFERENCES products(id),
    title      TEXT   NOT NULL,
    item_type  VARCHAR(10) NOT NULL CHECK (item_type IN ('ar_card', 'dongeng')),
    normal_idr BIGINT NOT NULL,
    price_idr  BIGINT NOT NULL,
    PRIMARY KEY (order_id, product_id)
);

INSERT INTO app_feature_flags (key, name, description, is_enabled) VALUES
    ('cart', 'Keranjang Belanja', 'Add Kartu AR and Dongeng to a cart and pay for them in one Google Play payment.', FALSE)
ON CONFLICT (key) DO NOTHING;
