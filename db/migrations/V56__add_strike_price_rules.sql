-- Promotional strike prices ("harga coret"): a display-only crossed-out
-- price shown next to the real price. Never charged — orders always use
-- price_idr. Every active promo must have an end date (see
-- StrikePriceService: at most 90 days), so a strike price is always a
-- time-limited promo, never a permanent fake "normal price".

-- One global rule per scope. AR_CARD/DONGENG apply to products by their
-- feature code; PACKAGE applies to every premium package.
CREATE TABLE IF NOT EXISTS strike_price_rules (
    scope      VARCHAR(20) PRIMARY KEY CHECK (scope IN ('AR_CARD', 'DONGENG', 'PACKAGE')),
    mode       VARCHAR(10) NOT NULL DEFAULT 'NONE' CHECK (mode IN ('NONE', 'PERCENT', 'FIXED')),
    value      INTEGER     NOT NULL DEFAULT 0,
    starts_at  TIMESTAMPTZ,
    ends_at    TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT strike_price_rules_end_required CHECK (mode = 'NONE' OR ends_at IS NOT NULL)
);

INSERT INTO strike_price_rules (scope, mode, value) VALUES
    ('AR_CARD', 'NONE', 0),
    ('DONGENG', 'NONE', 0),
    ('PACKAGE', 'NONE', 0)
ON CONFLICT (scope) DO NOTHING;

-- Per-item override. strike_mode NULL = inherit the scope's global rule;
-- 'NONE' = never show a strike price for this item.
ALTER TABLE products
    ADD COLUMN IF NOT EXISTS strike_mode      VARCHAR(10) CHECK (strike_mode IN ('NONE', 'PERCENT', 'FIXED')),
    ADD COLUMN IF NOT EXISTS strike_value     INTEGER,
    ADD COLUMN IF NOT EXISTS strike_starts_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS strike_ends_at   TIMESTAMPTZ;
ALTER TABLE products ADD CONSTRAINT products_strike_end_required
    CHECK (strike_mode IS NULL OR strike_mode = 'NONE' OR strike_ends_at IS NOT NULL);

ALTER TABLE premium_packages
    ADD COLUMN IF NOT EXISTS strike_mode      VARCHAR(10) CHECK (strike_mode IN ('NONE', 'PERCENT', 'FIXED')),
    ADD COLUMN IF NOT EXISTS strike_value     INTEGER,
    ADD COLUMN IF NOT EXISTS strike_starts_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS strike_ends_at   TIMESTAMPTZ;
ALTER TABLE premium_packages ADD CONSTRAINT premium_packages_strike_end_required
    CHECK (strike_mode IS NULL OR strike_mode = 'NONE' OR strike_ends_at IS NOT NULL);
