-- AR card categories with parent/child structure
-- parent_id NULL  → top-level category  (e.g. "Binatang", "Kendaraan")
-- parent_id set   → sub-category        (e.g. "Ternak", "Laut", "Hutan")
CREATE TABLE IF NOT EXISTS ar_card_categories (
    id         UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(100) NOT NULL,
    emoji      VARCHAR(20)  NOT NULL DEFAULT '',
    parent_id  UUID         REFERENCES ar_card_categories(id) ON DELETE SET NULL,
    sort_order INT          NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    is_deleted BOOLEAN      NOT NULL DEFAULT false
);

CREATE INDEX IF NOT EXISTS idx_ar_card_categories_parent ON ar_card_categories(parent_id);

-- Add category_id FK to ar_cards (references top-level category)
ALTER TABLE ar_cards
    ADD COLUMN IF NOT EXISTS category_id UUID REFERENCES ar_card_categories(id) ON DELETE SET NULL;

-- Add sub_category_id FK to ar_cards (references child category)
ALTER TABLE ar_cards
    ADD COLUMN IF NOT EXISTS sub_category_id UUID REFERENCES ar_card_categories(id) ON DELETE SET NULL;
