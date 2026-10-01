-- R__seed_admin_user.sql
-- Repeatable Flyway migration: seeds the default admin user.
-- Only runs if no admin users exist yet.
-- Change the email/password before running in production.
-- Password: 'admin123' hashed with bcrypt cost 12.
-- To generate a new hash, run: htpasswd -bnBC 12 "" <password> | tr -d ':\n'
-- or use Go: bcrypt.GenerateFromPassword([]byte("password"), 12)

-- The previous hash here did not actually match 'admin123' — bcrypt-verified
-- while wiring up the Flutter integration_test harness (Phase 5), which
-- needs a real admin login against a freshly migrated database. Since this
-- seed is the only way to get a first admin session on a brand-new
-- environment, a wrong hash here means every fresh install starts locked
-- out of its own backoffice.
INSERT INTO admin_users (email, password_hash)
SELECT
    'admin@arunika.id',
    '$2a$12$7V1XHWUys9phoSI3jWdpxe1qaksZhLDuTMPrVPMS9IabJSQq3hszO' -- admin123
WHERE NOT EXISTS (
    SELECT 1 FROM admin_users WHERE is_deleted = false
);
