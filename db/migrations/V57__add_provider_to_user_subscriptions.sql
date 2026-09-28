-- Which payment rail manages a subscription. Google Play subscriptions renew
-- (and are cancelled/resubscribed) through Play itself, so they are never
-- renewed through a new in-app order; Midtrans subscriptions are renewed by
-- buying again inside the renewal window (see EntitlementService.CanRenew).
ALTER TABLE user_subscriptions
    ADD COLUMN IF NOT EXISTS provider VARCHAR(20) NOT NULL DEFAULT 'midtrans'
    CHECK (provider IN ('midtrans', 'google_play'));

-- auto_renew has existed since V1 but was never written. Backfill Play-managed
-- subscriptions — those whose latest PAID subscription order came through
-- Google Play — as auto-renewing, the Play default; the next RTDN
-- (SUBSCRIPTION_CANCELED / RESTARTED / RENEWED) corrects any that aren't.
UPDATE user_subscriptions us
SET provider = 'google_play', auto_renew = TRUE
FROM (
    SELECT DISTINCT ON (o.user_id) o.user_id, o.provider
    FROM orders o
    JOIN premium_packages p ON p.id = o.package_id
    WHERE o.status = 'PAID' AND p.type = 'subscription'
    ORDER BY o.user_id, o.created_at DESC
) latest
WHERE latest.user_id = us.user_id AND latest.provider = 'google_play';
