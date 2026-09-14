-- Stores the Midtrans Snap payment link (redirect_url) for the order's
-- initial payment attempt, so it can be re-surfaced (e.g. resend link,
-- admin support lookups) without re-deriving it from Midtrans.
ALTER TABLE payments ADD COLUMN payment_url TEXT NULL;
