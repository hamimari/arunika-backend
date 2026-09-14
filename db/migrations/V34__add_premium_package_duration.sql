-- duration_days lets subscription packs have distinct durations instead of a
-- hardcoded "+1 month" in the webhook handler. NULL for type='content' packs.
ALTER TABLE premium_packages ADD COLUMN duration_days INTEGER NULL;

UPDATE premium_packages SET duration_days = 30  WHERE name = 'Bulanan' AND type = 'subscription';
UPDATE premium_packages SET duration_days = 365 WHERE name = 'Tahunan' AND type = 'subscription';

ALTER TABLE premium_packages
  ADD CONSTRAINT chk_premium_packages_duration_days_required
  CHECK (type != 'subscription' OR duration_days IS NOT NULL);
