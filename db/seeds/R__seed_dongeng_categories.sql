-- R__seed_dongeng_categories.sql
-- Repeatable Flyway migration: seeds the two default top-level dongeng
-- categories. Idempotent — only inserts a row if one with that name doesn't
-- already exist as a top-level category.

INSERT INTO dongeng_categories (id, name, emoji, sort_order)
SELECT gen_random_uuid(), 'Fairy Tales', '🧚', 0
WHERE NOT EXISTS (
    SELECT 1 FROM dongeng_categories WHERE name = 'Fairy Tales' AND parent_id IS NULL
);

INSERT INTO dongeng_categories (id, name, emoji, sort_order)
SELECT gen_random_uuid(), 'Islamic', '🕌', 1
WHERE NOT EXISTS (
    SELECT 1 FROM dongeng_categories WHERE name = 'Islamic' AND parent_id IS NULL
);
