CREATE TABLE premium_package_items (
  package_id UUID NOT NULL REFERENCES premium_packages(id) ON DELETE CASCADE,
  product_id UUID NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (package_id, product_id)
);

-- Backfill: Paket Hutan/Lautan/Ternak bundle every paid ar_card whose
-- (legacy free-text) category matches the pack's animal category.
-- ar_cards.category is the forward-looking home for "hutan"/"laut"/"ternak"
-- animal content (the older, unrouted `animals` table is not part of this
-- catalog and is left untouched). Today's seed data has no categorized
-- ar_cards yet, so this may insert zero rows until content is categorized —
-- expected, not a bug; new ar_cards tagged with these categories will be
-- picked up the next time this bundle is (re)configured via the admin UI.
INSERT INTO premium_package_items (package_id, product_id)
SELECT pp.id, pac.product_id
FROM premium_packages pp
JOIN ar_cards a ON a.category = CASE pp.name
    WHEN 'Paket Hutan' THEN 'hutan'
    WHEN 'Paket Lautan' THEN 'laut'
    WHEN 'Paket Ternak' THEN 'ternak'
  END
JOIN product_ar_cards pac ON pac.ar_card_id = a.id
WHERE pp.name IN ('Paket Hutan', 'Paket Lautan', 'Paket Ternak')
ON CONFLICT DO NOTHING;

-- ALL ACCESS PASS: every currently-paid product (ar_cards + dongengs).
INSERT INTO premium_package_items (package_id, product_id)
SELECT pp.id, p.id
FROM premium_packages pp, products p
WHERE pp.name = 'ALL ACCESS PASS'
ON CONFLICT DO NOTHING;
