-- Dongeng categories are represented by image_url (V43) only, matching
-- ar_card_categories (V47); the emoji column is no longer read or written.
ALTER TABLE dongeng_categories DROP COLUMN IF EXISTS emoji;
