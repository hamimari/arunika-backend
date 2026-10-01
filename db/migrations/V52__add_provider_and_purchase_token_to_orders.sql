-- provider flags which payment rail an order was created against, so the
-- backoffice can show the one relevant "sync" action per order instead of
-- guessing from payment rows. purchase_token stores the Google Play
-- purchase token as soon as the app reports it (see
-- PaymentService.VerifyPlayPurchase), independent of whether verification
-- against Google succeeds — so a later admin-triggered re-sync never needs
-- it typed in by hand.
ALTER TABLE orders ADD COLUMN provider VARCHAR(20) NOT NULL DEFAULT 'midtrans'
  CHECK (provider IN ('midtrans', 'google_play'));
ALTER TABLE orders ADD COLUMN purchase_token VARCHAR(300);

-- Backfill: any existing order whose payment rows are already tagged
-- google_play was in fact a Play Billing order, not Midtrans.
UPDATE orders o SET provider = 'google_play'
WHERE EXISTS (
  SELECT 1 FROM payments p WHERE p.order_id = o.id AND p.payment_type LIKE 'google_play%'
);
