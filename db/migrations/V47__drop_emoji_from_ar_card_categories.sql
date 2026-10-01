-- AR card categories are represented by image_url (V27) only; the emoji
-- column is no longer read or written.
ALTER TABLE ar_card_categories DROP COLUMN IF EXISTS emoji;
