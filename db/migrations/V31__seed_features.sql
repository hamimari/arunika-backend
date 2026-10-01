-- Seed the feature taxonomy used by the products catalog.
-- `features` already exists (V1) but has been unused/empty until now.
INSERT INTO features (code, name, description, is_active)
VALUES
  ('AR_CARD', 'AR Card', 'Augmented-reality card content', true),
  ('DONGENG', 'Dongeng', 'Fairy tale / story content', true)
ON CONFLICT (code) DO NOTHING;
