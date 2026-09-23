-- Maps a premium package to a Google Play Console in-app product /
-- subscription SKU so purchases can route through Google Play Billing.
-- Nullable: a package with no mapping simply isn't purchasable via Play
-- Billing yet. Existing rows need no backfill.
ALTER TABLE premium_packages ADD COLUMN IF NOT EXISTS play_product_id TEXT;
