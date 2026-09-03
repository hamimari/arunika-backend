-- V17: add hidden column to content tables for admin visibility toggle
-- hidden = true means the item is not returned to mobile app clients

ALTER TABLE dongengs          ADD COLUMN IF NOT EXISTS hidden BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE ar_cards          ADD COLUMN IF NOT EXISTS hidden BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE tracing_items     ADD COLUMN IF NOT EXISTS hidden BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE counting_questions ADD COLUMN IF NOT EXISTS hidden BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE badges            ADD COLUMN IF NOT EXISTS hidden BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE categories        ADD COLUMN IF NOT EXISTS hidden BOOLEAN NOT NULL DEFAULT false;
