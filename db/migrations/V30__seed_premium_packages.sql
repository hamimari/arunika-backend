-- Migration: Seed default premium packages (mirrors static PremiumPacks data)
INSERT INTO premium_packages (name, subtitle, price_idr, type, badge_label, is_best_value, is_active, sort_order)
VALUES
  ('Paket Hutan',   '8 Hewan Hutan + 2 Dongeng',               29000, 'content',      NULL,         FALSE, TRUE, 1),
  ('Paket Lautan',  '8 Hewan Laut + 2 Dongeng',                29000, 'content',      NULL,         FALSE, TRUE, 2),
  ('Paket Ternak',  '8 Hewan Ternak + 2 Dongeng',              29000, 'content',      NULL,         FALSE, TRUE, 3),
  ('ALL ACCESS PASS', 'Semua hewan, semua dongeng, akses offline', 79000, 'content',  'BEST VALUE', TRUE,  TRUE, 4),
  ('Bulanan',       'Akses penuh selama 1 bulan',               39000, 'subscription', NULL,         FALSE, TRUE, 1),
  ('Tahunan',       'Akses penuh selama 12 bulan',             299000, 'subscription', 'HEMAT 36%', TRUE,  TRUE, 2);
