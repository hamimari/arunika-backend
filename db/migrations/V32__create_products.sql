-- Products catalog: one row per purchasable content item, linked to a feature.
-- A content item (ar_card / dongeng) with no row in the mapping tables below
-- is implicitly free — there is nothing to buy.
CREATE TABLE products (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  feature_id UUID NOT NULL REFERENCES features(id),
  price_idr  BIGINT NOT NULL,
  is_active  BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE product_ar_cards (
  product_id UUID PRIMARY KEY REFERENCES products(id) ON DELETE CASCADE,
  ar_card_id UUID NOT NULL UNIQUE REFERENCES ar_cards(id) ON DELETE RESTRICT
);

CREATE TABLE product_dongengs (
  product_id UUID PRIMARY KEY REFERENCES products(id) ON DELETE CASCADE,
  dongeng_id UUID NOT NULL UNIQUE REFERENCES dongengs(id) ON DELETE RESTRICT
);
