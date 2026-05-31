-- V13: Add image_url column to ar_card_categories
ALTER TABLE ar_card_categories
    ADD COLUMN IF NOT EXISTS image_url TEXT NOT NULL DEFAULT '';
