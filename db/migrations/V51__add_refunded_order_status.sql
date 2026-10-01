-- REFUNDED marks a previously-PAID order whose purchase Google Play has
-- since voided (refund, chargeback, or its own automatic refund of a Play
-- Billing purchase left unacknowledged for 3 days) — see
-- PaymentService.ReconcileVoidedPurchases. Distinct from EXPIRED, which
-- means the order was never paid at all.
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_status_check;
ALTER TABLE orders ADD CONSTRAINT orders_status_check
  CHECK (status IN ('PENDING','PAID','FAILED','EXPIRED','REFUNDED'));
