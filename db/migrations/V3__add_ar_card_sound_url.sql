-- Add sound_url column to ar_cards so each AR card can carry an optional
-- audio file URL (played in the mobile AR viewer after model placement).
ALTER TABLE ar_cards ADD COLUMN IF NOT EXISTS sound_url TEXT NOT NULL DEFAULT '';
