-- Fix missing columns in dongengs table (added in model but missing from V1 migration)
ALTER TABLE dongengs ADD COLUMN IF NOT EXISTS age_end INT;
ALTER TABLE dongengs ADD COLUMN IF NOT EXISTS audio_url VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE dongengs ADD COLUMN IF NOT EXISTS duration BIGINT NOT NULL DEFAULT 0;

-- Create dongeng_pages table for book-style page-by-page reading
CREATE TABLE IF NOT EXISTS dongeng_pages (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dongeng_id  UUID NOT NULL,
    page_number INT NOT NULL,
    image_url   TEXT NOT NULL DEFAULT '',
    text        TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    is_deleted  BOOLEAN NOT NULL DEFAULT false,
    CONSTRAINT fk_dongeng_pages_dongeng
        FOREIGN KEY (dongeng_id) REFERENCES dongengs(id) ON DELETE CASCADE,
    CONSTRAINT uq_dongeng_page_number
        UNIQUE (dongeng_id, page_number)
);

CREATE INDEX IF NOT EXISTS idx_dongeng_pages_dongeng_id ON dongeng_pages(dongeng_id);
