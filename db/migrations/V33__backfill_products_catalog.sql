-- Backfill: create a `products` row (+ mapping row) for every currently-paid
-- AR card and dongeng, so the entitlement cutover (see later migrations) never
-- flips a paid item to free just because it has no product row yet.
-- "Paid" = the flag the app already uses to decide this today:
--   ar_cards.is_unlocked = false, dongengs.is_free = false.
-- price_idr is a placeholder (0): individual items are only sold today as
-- part of a premium_packages bundle — there is no per-item purchase price yet.

WITH ar_feature AS (
  SELECT id FROM features WHERE code = 'AR_CARD'
),
to_migrate AS (
  SELECT a.id AS ar_card_id, gen_random_uuid() AS product_id
  FROM ar_cards a
  WHERE a.is_unlocked = false
),
inserted_products AS (
  INSERT INTO products (id, feature_id, price_idr, is_active)
  SELECT tm.product_id, f.id, 0, true
  FROM to_migrate tm, ar_feature f
  RETURNING id
)
INSERT INTO product_ar_cards (product_id, ar_card_id)
SELECT tm.product_id, tm.ar_card_id
FROM to_migrate tm
JOIN inserted_products ip ON ip.id = tm.product_id;

WITH dongeng_feature AS (
  SELECT id FROM features WHERE code = 'DONGENG'
),
to_migrate AS (
  SELECT d.id AS dongeng_id, gen_random_uuid() AS product_id
  FROM dongengs d
  WHERE d.is_free = false
),
inserted_products AS (
  INSERT INTO products (id, feature_id, price_idr, is_active)
  SELECT tm.product_id, f.id, 0, true
  FROM to_migrate tm, dongeng_feature f
  RETURNING id
)
INSERT INTO product_dongengs (product_id, dongeng_id)
SELECT tm.product_id, tm.dongeng_id
FROM to_migrate tm
JOIN inserted_products ip ON ip.id = tm.product_id;
