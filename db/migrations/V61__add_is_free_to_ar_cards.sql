-- Lets an admin make a card free without touching its product, orders or
-- entitlements (dongengs already have this flag). A card is free when it is
-- flagged here OR has no product; existing cards keep their behaviour.
ALTER TABLE ar_cards ADD COLUMN IF NOT EXISTS is_free BOOLEAN NOT NULL DEFAULT false;
