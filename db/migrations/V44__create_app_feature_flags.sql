-- Remote on/off switches for app features, toggled from the backoffice and
-- read by the mobile app at startup (GET /app/feature-flags). Rows are keyed
-- by a stable string the app hard-codes; add a row here for each new flag.
CREATE TABLE IF NOT EXISTS app_feature_flags (
    key         VARCHAR(50)  PRIMARY KEY,
    name        VARCHAR(100) NOT NULL,
    description TEXT         NOT NULL DEFAULT '',
    is_enabled  BOOLEAN      NOT NULL DEFAULT TRUE,
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

INSERT INTO app_feature_flags (key, name, description, is_enabled) VALUES
    ('printable_cards', 'Kartu Printable', 'Printable AR card PDF download section on the home screen.', TRUE),
    ('qr_scan',         'Scan QR Kartu AR', 'Scan tab in the bottom navigation and other QR scan entry points.', TRUE)
ON CONFLICT (key) DO NOTHING;
