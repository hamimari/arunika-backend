-- R__seed_dongeng_categories.sql
-- Repeatable Flyway migration: seeds the two default top-level dongeng
-- categories. Idempotent — only inserts a row if one with that name doesn't
-- already exist as a top-level category.
--
-- The emoji column these inserts used to set was dropped by V48 (dongeng
-- categories are represented by image_url from V43 only). Because Flyway runs
-- repeatables after the whole versioned set, keeping it here made every
-- fresh `flyway migrate` fail with "column emoji does not exist".

INSERT INTO dongeng_categories (id, name, sort_order)
SELECT gen_random_uuid(), 'Fairy Tales', 0
WHERE NOT EXISTS (
    SELECT 1 FROM dongeng_categories WHERE name = 'Fairy Tales' AND parent_id IS NULL
);

INSERT INTO dongeng_categories (id, name, sort_order)
SELECT gen_random_uuid(), 'Islamic', 1
WHERE NOT EXISTS (
    SELECT 1 FROM dongeng_categories WHERE name = 'Islamic' AND parent_id IS NULL
);
