-- Cutover complete: ar_cards/dongengs access is now computed per-request from
-- products/user_entitlements/user_subscriptions (see ArService/DongengService).
-- These stored flags were never read by any server-side gating logic even
-- before this change (see design.md) — safe to drop now that the app/backend
-- no longer read them for access control.
ALTER TABLE ar_cards DROP COLUMN is_unlocked;
ALTER TABLE animals DROP COLUMN is_unlocked;
