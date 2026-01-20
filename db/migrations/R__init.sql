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

insert into ar_cards (id, title, type, file_url, short_code, created_at)
values
    ('830d1532-e51a-41c7-8f2f-b841883e5e1c', 'Truck', '3D', '/ar/cards/830d1532-e51a-41c7-8f2f-b841883e5e1c', '', now());

insert into categories(id, name, image_url, created_at, updated_at, is_deleted)
values
    ('4d1bd9b0-c147-40ef-ada3-4fe0b051d149', 'Numbers', 'https://storage.googleapis.com/a1aa/image/KTco9PlO4VnxwpIOmykV40oSaTChaGPp_g0n39XWZDs.jpg', now(), now(), false),
    ('6c532f32-058b-4f88-8389-bc8f738dcecb', 'Shapes', 'https://storage.googleapis.com/a1aa/image/qSzsuL39YBR5CfR01URSfbRouZ7Q_2tImmwg-1eoKkI.jpg', now(), now(), false),
    ('9335ff4b-5080-430f-be3e-7935375d2ae1', 'Vocab', 'https://storage.googleapis.com/a1aa/image/Vt4DLo0qLYSGDuRSmGEizcrwpaywqfa5vjg5z55fQ5c.jpg', now(), now(), false),
    ('87cb06d3-48ef-49fb-8c5e-661fd55e1ba7', 'Dongeng', 'https://storage.googleapis.com/a1aa/image/hrtjK7BpRwCPJeq8wKrawJrq6dYsWJFSGknFf-ZeKxk.jpg', now(), now(), false);


INSERT INTO dongengs (title, age_start, age_ent, image_url, is_free, category_id)
VALUES
    ('Poor Pluto', 6, 9, 'https://storage.googleapis.com/a1aa/image/2KbLEXPe53yJBZZcTS3MrE4GC4mmaJ6k0zqQT3Fw4Wc.jpg', false, NULL),
    ('Hansel & Grate', 9, 12, 'https://storage.googleapis.com/a1aa/image/2KbLEXPe53yJBZZcTS3MrE4GC4mmaJ6k0zqQT3Fw4Wc.jpg', true, NULL);