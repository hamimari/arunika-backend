CREATE TABLE IF NOT EXISTS banners (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title       VARCHAR(200) NOT NULL DEFAULT '',
    image_url   TEXT         NOT NULL DEFAULT '',
    type        VARCHAR(50)  NOT NULL DEFAULT 'promo',  -- 'promo' | 'daily_animal' | 'feature'
    is_active   BOOLEAN      NOT NULL DEFAULT true,
    sort_order  INT          NOT NULL DEFAULT 0,
    cta_url     TEXT,
    emoji       VARCHAR(20),
    fact        TEXT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    is_deleted  BOOLEAN      NOT NULL DEFAULT false
);

CREATE INDEX IF NOT EXISTS idx_banners_active ON banners (is_active, sort_order) WHERE is_deleted = false;
