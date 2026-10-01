-- db/seeds/test/seed.sql
--
-- The deterministic corpus the cross-system E2E suite runs against — design H
-- of the automation-testing-strategy proposal. Fixed UUIDs, not generated
-- ones, so a flow test can refer to "the free user" or "the paid bundle" by
-- name (see tests/e2e/seed.go, which exposes each id as a Go constant)
-- instead of having to look anything up first.
--
-- Deliberately NOT a Flyway migration: this is test-only data, re-applied by
-- tests/e2e's Up() helper against a database that versioned migrations and
-- the repeatable R__ seeds (admin user, dongeng categories) have already been
-- applied to. It has no version number and is never registered with Flyway,
-- so it can never accidentally run against a real environment.
--
-- Password for all three users below is 'e2e-test-password' — the hash was
-- generated and then verified with bcrypt.CompareHashAndPassword before
-- being pasted in here, the same check that would have caught the
-- R__seed_admin_user.sql defect this suite's own admin login depends on not
-- repeating (see design D... / Phase 5 notes).

-- ── Users ────────────────────────────────────────────────────────────────────

-- e2e_user_free: no entitlements, no subscription. Everything paid must show
-- as locked for this user.
INSERT INTO parents (id, name, phone_number, email_address, password, address, city, email_verified)
VALUES ('e2e00000-0000-0000-0000-000000000001', 'E2E Free User', '081100000001',
        'e2e-free@arunika.test',
        '$2a$10$TF9O/zXEHHSK1SgK1tjtBuJX0LIXkiZwMrAwquw0DJVIwMtJ6PTFy',
        'Jl. E2E', 'Jakarta', true)
ON CONFLICT (id) DO NOTHING;

-- e2e_user_entitled: owns exactly one paid AR card (e2e_ar_card_paid_1) via a
-- direct entitlement, granted below as if from a completed purchase.
INSERT INTO parents (id, name, phone_number, email_address, password, address, city, email_verified)
VALUES ('e2e00000-0000-0000-0000-000000000002', 'E2E Entitled User', '081100000002',
        'e2e-entitled@arunika.test',
        '$2a$10$TF9O/zXEHHSK1SgK1tjtBuJX0LIXkiZwMrAwquw0DJVIwMtJ6PTFy',
        'Jl. E2E', 'Jakarta', true)
ON CONFLICT (id) DO NOTHING;

-- e2e_user_subscriber: active blanket subscription — every paid product must
-- show as unlocked for this user, without a per-product entitlement row.
INSERT INTO parents (id, name, phone_number, email_address, password, address, city, email_verified)
VALUES ('e2e00000-0000-0000-0000-000000000003', 'E2E Subscriber', '081100000003',
        'e2e-subscriber@arunika.test',
        '$2a$10$TF9O/zXEHHSK1SgK1tjtBuJX0LIXkiZwMrAwquw0DJVIwMtJ6PTFy',
        'Jl. E2E', 'Jakarta', true)
ON CONFLICT (id) DO NOTHING;

-- ── Feature taxonomy (idempotent — V31 may already have seeded these) ─────────

INSERT INTO features (code, name, description, is_active)
VALUES
  ('AR_CARD', 'AR Card', 'Augmented-reality card content', true),
  ('DONGENG', 'Dongeng', 'Fairy tale / story content', true)
ON CONFLICT (code) DO NOTHING;

-- ── AR cards: 3 free, 2 paid ───────────────────────────────────────────────────

INSERT INTO ar_cards (id, title, type, file_url, short_code)
VALUES
  ('e2e0ac00-0000-0000-0000-000000000001', 'E2E Free Card 1', 'animal', 'https://e2e.test/model.glb', 'E2EAC01'),
  ('e2e0ac00-0000-0000-0000-000000000002', 'E2E Free Card 2', 'animal', 'https://e2e.test/model.glb', 'E2EAC02'),
  ('e2e0ac00-0000-0000-0000-000000000003', 'E2E Free Card 3', 'animal', 'https://e2e.test/model.glb', 'E2EAC03'),
  ('e2e0ac00-0000-0000-0000-000000000004', 'E2E Paid Card 1', 'animal', 'https://e2e.test/model.glb', 'E2EAC04'),
  ('e2e0ac00-0000-0000-0000-000000000005', 'E2E Paid Card 2', 'animal', 'https://e2e.test/model.glb', 'E2EAC05')
ON CONFLICT (id) DO NOTHING;

-- Only cards 4 and 5 get a linked product — a card with no product is free
-- by construction (ArService.applyUnlocked), so cards 1-3 need no further
-- rows to be free.
INSERT INTO products (id, feature_id, price_idr, play_product_id)
SELECT 'e2e0ac00-0000-0000-0000-0000000000a4', id, 25000, 'e2e_sku_ar_card_1'
FROM features WHERE code = 'AR_CARD'
ON CONFLICT (id) DO NOTHING;

INSERT INTO products (id, feature_id, price_idr, play_product_id)
SELECT 'e2e0ac00-0000-0000-0000-0000000000a5', id, 30000, 'e2e_sku_ar_card_2'
FROM features WHERE code = 'AR_CARD'
ON CONFLICT (id) DO NOTHING;

INSERT INTO product_ar_cards (product_id, ar_card_id)
VALUES
  ('e2e0ac00-0000-0000-0000-0000000000a4', 'e2e0ac00-0000-0000-0000-000000000004'),
  ('e2e0ac00-0000-0000-0000-0000000000a5', 'e2e0ac00-0000-0000-0000-000000000005')
ON CONFLICT (ar_card_id) DO NOTHING;

-- The entitled user already owns Paid Card 1, as if from a purchase that
-- settled before this test run started.
INSERT INTO user_entitlements (user_id, product_id, starts_at)
VALUES ('e2e00000-0000-0000-0000-000000000002', 'e2e0ac00-0000-0000-0000-0000000000a4', NOW())
ON CONFLICT (user_id, product_id) DO NOTHING;

-- ── Dongeng: 2 free, 2 paid ────────────────────────────────────────────────────

INSERT INTO dongengs (id, title, image_url, audio_url, is_free, duration)
VALUES
  ('e2e0d000-0000-0000-0000-000000000001', 'E2E Free Dongeng 1', 'https://e2e.test/img.png', 'https://e2e.test/audio.mp3', true, 300),
  ('e2e0d000-0000-0000-0000-000000000002', 'E2E Free Dongeng 2', 'https://e2e.test/img.png', 'https://e2e.test/audio.mp3', true, 300),
  ('e2e0d000-0000-0000-0000-000000000003', 'E2E Paid Dongeng 1', 'https://e2e.test/img.png', 'https://e2e.test/audio.mp3', false, 300),
  ('e2e0d000-0000-0000-0000-000000000004', 'E2E Paid Dongeng 2', 'https://e2e.test/img.png', 'https://e2e.test/audio.mp3', false, 300)
ON CONFLICT (id) DO NOTHING;

-- ── Packages: 1 content bundle (2 items), 1 subscription ──────────────────────

INSERT INTO premium_packages (id, name, subtitle, price_idr, type, play_product_id)
VALUES ('e2e0b000-0000-0000-0000-000000000001', 'E2E Bundle', 'Bundle of paid AR cards', 45000, 'content', 'e2e_sku_bundle')
ON CONFLICT (id) DO NOTHING;

INSERT INTO premium_package_items (package_id, product_id)
VALUES
  ('e2e0b000-0000-0000-0000-000000000001', 'e2e0ac00-0000-0000-0000-0000000000a4'),
  ('e2e0b000-0000-0000-0000-000000000001', 'e2e0ac00-0000-0000-0000-0000000000a5')
ON CONFLICT (package_id, product_id) DO NOTHING;

INSERT INTO premium_packages (id, name, subtitle, price_idr, type, duration_days, play_product_id)
VALUES ('e2e0b000-0000-0000-0000-000000000002', 'E2E Monthly', 'Unlimited access, billed monthly', 39000, 'subscription', 30, 'e2e_sku_subscription')
ON CONFLICT (id) DO NOTHING;

-- The subscriber's blanket access — modelling a subscription purchased
-- before this run started, exactly like the entitled user's direct grant
-- above models a one-off purchase.
INSERT INTO user_subscriptions (user_id, status, expires_at, package_id, start_date)
VALUES ('e2e00000-0000-0000-0000-000000000003', 'premium', NOW() + INTERVAL '30 days',
        'e2e0b000-0000-0000-0000-000000000002', NOW())
ON CONFLICT (user_id) DO NOTHING;
