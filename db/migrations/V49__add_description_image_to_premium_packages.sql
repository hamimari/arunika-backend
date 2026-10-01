-- Nullable, purely presentational fields for the landing page's product
-- showcase. Existing rows need no backfill.
ALTER TABLE premium_packages ADD COLUMN IF NOT EXISTS description TEXT;
ALTER TABLE premium_packages ADD COLUMN IF NOT EXISTS image_url TEXT;
