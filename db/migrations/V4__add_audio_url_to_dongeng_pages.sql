-- Add audio_url to dongeng_pages so each page can carry its own narration file.
ALTER TABLE dongeng_pages ADD COLUMN IF NOT EXISTS audio_url TEXT NOT NULL DEFAULT '';
