-- R__seed_admin_user.sql
-- Repeatable Flyway migration: seeds the default admin user.
-- Only runs if no admin users exist yet.
-- Change the email/password before running in production.
-- Password: 'admin123' hashed with bcrypt cost 12.
-- To generate a new hash, run: htpasswd -bnBC 12 "" <password> | tr -d ':\n'
-- or use Go: bcrypt.GenerateFromPassword([]byte("password"), 12)

INSERT INTO admin_users (email, password_hash)
SELECT
    'admin@arunika.id',
    '$2a$12$Ei4ZMwoxaE.wKstC62/nGeNn5wf3Q3XHAWqvL2VFLlAJmxQCsH9au' -- admin123
WHERE NOT EXISTS (
    SELECT 1 FROM admin_users WHERE is_deleted = false
);
