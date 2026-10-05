-- V65: rollout switch for Belajar Huruf in the app (off until content is published).
INSERT INTO app_feature_flags (key, name, description, is_enabled) VALUES
    ('belajar_huruf', 'Belajar Huruf', 'Huruf card on Belajar and Beranda.', FALSE)
ON CONFLICT (key) DO NOTHING;
