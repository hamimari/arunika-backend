TRUNCATE TABLE children RESTART IDENTITY CASCADE;
TRUNCATE TABLE parents RESTART IDENTITY CASCADE;

-- Insert Parents
INSERT INTO parents (id, name, phone_number, email_address, password, address, city)
VALUES
    ('b5fed1d0-eecc-4a1e-86f9-bfbc778e3617','John Doe', '081234567890', 'john@example.com', 'hashedpassword1', 'Indonesia', 'Jakarta'),
    ('6da22c2a-07a9-4d4b-91d1-b100991f565c','Jane Smith', '081298765432', 'jane@example.com', 'hashedpassword2', 'Indonesia', 'Bandung')
    ON CONFLICT (email_address) DO NOTHING;

-- Insert Children (assuming parent_id is resolved from the inserted parents)
INSERT INTO children (id, name, gender, date_of_birth, parent_id)
VALUES
    ('df771dea-f93b-4a7f-bd96-023d80a25887','Alice Doe', 'female', '2015-05-20', (SELECT id FROM parents WHERE email_address = 'john@example.com')),
    ('459fa006-75ba-4ba8-98cb-eaa0f1c55917','Bob Doe', 'male', '2018-08-15', (SELECT id FROM parents WHERE email_address = 'john@example.com')),
    ('27abd12f-32fc-4bfd-8af8-6590062cbb3c','Clara Smith', 'female', '2017-03-10', (SELECT id FROM parents WHERE email_address = 'jane@example.com'))
    ON CONFLICT DO NOTHING;
