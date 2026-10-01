-- Dead schema from V1__init.sql: zero Go references, fully superseded.
-- `subscriptions` (old) was replaced by `user_subscriptions` (V11) long ago;
-- `plans`/`plan_features`/`vouchers`/`voucher_redemption` were never wired
-- to any service/handler.
DROP TABLE IF EXISTS voucher_redemption;
DROP TABLE IF EXISTS vouchers;
DROP TABLE IF EXISTS plan_features;
DROP TABLE IF EXISTS subscriptions;
DROP TABLE IF EXISTS plans;
