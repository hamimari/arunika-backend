-- Dongeng categories with parent/child structure, mirrors ar_card_categories.
-- parent_id NULL  → top-level category  (e.g. "Fairy Tales", "Islamic")
-- parent_id set   → sub-category
CREATE TABLE IF NOT EXISTS dongeng_categories (
    id         UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(100) NOT NULL,
    emoji      VARCHAR(20)  NOT NULL DEFAULT '',
    image_url  TEXT         NOT NULL DEFAULT '',
    parent_id  UUID         REFERENCES dongeng_categories(id) ON DELETE SET NULL,
    sort_order INT          NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    is_deleted BOOLEAN      NOT NULL DEFAULT false
);

CREATE INDEX IF NOT EXISTS idx_dongeng_categories_parent ON dongeng_categories(parent_id);

-- Additive FK columns on dongengs — the pre-existing `category_id` (generic
-- `categories` table) is untouched; these are new, separate columns.
ALTER TABLE dongengs
    ADD COLUMN IF NOT EXISTS dongeng_category_id UUID REFERENCES dongeng_categories(id) ON DELETE SET NULL;

ALTER TABLE dongengs
    ADD COLUMN IF NOT EXISTS dongeng_sub_category_id UUID REFERENCES dongeng_categories(id) ON DELETE SET NULL;
