-- Midtrans is only allowed on Google Play as the alternative under User
-- Choice Billing (enrolled, offered alongside Google Play Billing), and not
-- at all for digital content on iOS. It stays off unless an admin turns it
-- on; while off, /payment/create and /payment/create-product are refused.
INSERT INTO app_feature_flags (key, name, description, is_enabled) VALUES
    ('alternative_billing', 'Midtrans (alternative billing)',
     'Offer Midtrans alongside Google Play Billing (User Choice Billing). Keep OFF unless enrolled in User Choice Billing in Play Console.',
     FALSE)
ON CONFLICT (key) DO NOTHING;
