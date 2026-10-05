-- V67: rollout switch for Belajar Angka in the app (off until content is published).
INSERT INTO app_feature_flags (key, name, description, is_enabled) VALUES
    ('belajar_angka', 'Belajar Angka', 'Angka card on Belajar and Beranda.', FALSE)
ON CONFLICT (key) DO NOTHING;
