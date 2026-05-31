TRUNCATE TABLE children RESTART IDENTITY CASCADE;
TRUNCATE TABLE parents RESTART IDENTITY CASCADE;
TRUNCATE TABLE animals CASCADE;

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


INSERT INTO dongengs (id, title, age_start, age_ent, age_end, image_url, is_free, category_id)
VALUES
    ('a1b2c3d4-0001-0001-0001-000000000001', 'Si Kancil dan Buaya', 3, 6, 6, 'https://storage.googleapis.com/a1aa/image/2KbLEXPe53yJBZZcTS3MrE4GC4mmaJ6k0zqQT3Fw4Wc.jpg', true, NULL),
    ('a1b2c3d4-0002-0002-0002-000000000002', 'Hansel dan Gretel', 5, 9, 9, 'https://storage.googleapis.com/a1aa/image/2KbLEXPe53yJBZZcTS3MrE4GC4mmaJ6k0zqQT3Fw4Wc.jpg', false, NULL),
    ('a1b2c3d4-0003-0003-0003-000000000003', 'Timun Mas', 4, 8, 8, 'https://storage.googleapis.com/a1aa/image/2KbLEXPe53yJBZZcTS3MrE4GC4mmaJ6k0zqQT3Fw4Wc.jpg', false, NULL)
ON CONFLICT (id) DO NOTHING;

-- Dongeng pages for Si Kancil dan Buaya
INSERT INTO dongeng_pages (dongeng_id, page_number, image_url, text)
VALUES
    ('a1b2c3d4-0001-0001-0001-000000000001', 1,
     'https://storage.googleapis.com/a1aa/image/2KbLEXPe53yJBZZcTS3MrE4GC4mmaJ6k0zqQT3Fw4Wc.jpg',
     'Di sebuah hutan yang lebat, hiduplah seekor kancil yang cerdik bernama Kancil. Ia sangat pandai dan suka berpetualang.'),
    ('a1b2c3d4-0001-0001-0001-000000000001', 2,
     'https://storage.googleapis.com/a1aa/image/2KbLEXPe53yJBZZcTS3MrE4GC4mmaJ6k0zqQT3Fw4Wc.jpg',
     'Suatu hari, Kancil sangat ingin menyeberangi sungai untuk mendapatkan buah-buahan lezat di seberang. Namun sungai itu penuh dengan buaya yang lapar.'),
    ('a1b2c3d4-0001-0001-0001-000000000001', 3,
     'https://storage.googleapis.com/a1aa/image/2KbLEXPe53yJBZZcTS3MrE4GC4mmaJ6k0zqQT3Fw4Wc.jpg',
     'Dengan kecerdikannya, Kancil berkata kepada para buaya, "Wahai buaya, Raja Hutan ingin menghitung jumlah kalian untuk memberi hadiah!" Para buaya pun berbaris rapi.'),
    ('a1b2c3d4-0001-0001-0001-000000000001', 4,
     'https://storage.googleapis.com/a1aa/image/2KbLEXPe53yJBZZcTS3MrE4GC4mmaJ6k0zqQT3Fw4Wc.jpg',
     'Kancil melompat dari satu buaya ke buaya lainnya sambil menghitung, hingga ia berhasil sampai di seberang sungai. "Terima kasih, buaya!" seru Kancil sambil tertawa.')
ON CONFLICT (dongeng_id, page_number) DO NOTHING;

-- Dongeng pages for Hansel dan Gretel
INSERT INTO dongeng_pages (dongeng_id, page_number, image_url, text)
VALUES
    ('a1b2c3d4-0002-0002-0002-000000000002', 1,
     'https://storage.googleapis.com/a1aa/image/2KbLEXPe53yJBZZcTS3MrE4GC4mmaJ6k0zqQT3Fw4Wc.jpg',
     'Hansel dan Gretel adalah dua bersaudara yang tinggal di tepi hutan bersama ayah dan ibu tiri mereka. Kehidupan mereka sangat miskin.'),
    ('a1b2c3d4-0002-0002-0002-000000000002', 2,
     'https://storage.googleapis.com/a1aa/image/2KbLEXPe53yJBZZcTS3MrE4GC4mmaJ6k0zqQT3Fw4Wc.jpg',
     'Suatu malam, ibu tiri mereka berencana meninggalkan mereka di hutan. Hansel yang cerdas mengumpulkan kerikil putih untuk menandai jalan pulang.'),
    ('a1b2c3d4-0002-0002-0002-000000000002', 3,
     'https://storage.googleapis.com/a1aa/image/2KbLEXPe53yJBZZcTS3MrE4GC4mmaJ6k0zqQT3Fw4Wc.jpg',
     'Di dalam hutan, mereka menemukan sebuah rumah yang terbuat dari kue dan permen. Mereka sangat lapar dan mulai memakan bagian-bagian dari rumah itu.')
ON CONFLICT (dongeng_id, page_number) DO NOTHING;

-- Animal seed data
INSERT INTO animals (id, name, emoji, category, image_url, bg_color, fact, is_unlocked)
VALUES
    ('00000000-0000-0000-0001-000000000001', 'Sapi',    '🐄', 'ternak', 'https://storage.googleapis.com/a1aa/image/KTco9PlO4VnxwpIOmykV40oSaTChaGPp_g0n39XWZDs.jpg', '#FFF8E1', 'Sapi bisa menghasilkan hingga 200.000 gelas susu dalam hidupnya.', true),
    ('00000000-0000-0000-0001-000000000002', 'Kambing', '🐐', 'ternak', 'https://storage.googleapis.com/a1aa/image/KTco9PlO4VnxwpIOmykV40oSaTChaGPp_g0n39XWZDs.jpg', '#F3E5F5', 'Kambing memiliki pupil persegi panjang yang membantu mereka melihat 320 derajat.', true),
    ('00000000-0000-0000-0001-000000000003', 'Ayam',    '🐔', 'ternak', 'https://storage.googleapis.com/a1aa/image/KTco9PlO4VnxwpIOmykV40oSaTChaGPp_g0n39XWZDs.jpg', '#FFF3E0', 'Ayam adalah hewan peliharaan paling banyak di dunia, lebih banyak dari manusia.', true),
    ('00000000-0000-0000-0001-000000000004', 'Bebek',   '🦆', 'ternak', 'https://storage.googleapis.com/a1aa/image/KTco9PlO4VnxwpIOmykV40oSaTChaGPp_g0n39XWZDs.jpg', '#E3F2FD', 'Bebek bisa tidur dengan satu mata terbuka untuk waspada terhadap predator.', false),
    ('00000000-0000-0000-0002-000000000001', 'Harimau', '🐯', 'hutan',  'https://storage.googleapis.com/a1aa/image/qSzsuL39YBR5CfR01URSfbRouZ7Q_2tImmwg-1eoKkI.jpg', '#FFF3E0', 'Harimau adalah kucing terbesar di dunia dan perenang yang handal.', true),
    ('00000000-0000-0000-0002-000000000002', 'Gajah',   '🐘', 'hutan',  'https://storage.googleapis.com/a1aa/image/qSzsuL39YBR5CfR01URSfbRouZ7Q_2tImmwg-1eoKkI.jpg', '#ECEFF1', 'Gajah adalah satu-satunya hewan yang tidak bisa melompat.', true),
    ('00000000-0000-0000-0002-000000000003', 'Rusa',    '🦌', 'hutan',  'https://storage.googleapis.com/a1aa/image/qSzsuL39YBR5CfR01URSfbRouZ7Q_2tImmwg-1eoKkI.jpg', '#E8F5E9', 'Tanduk rusa adalah jaringan tulang yang tumbuh paling cepat di dunia.', true),
    ('00000000-0000-0000-0002-000000000004', 'Monyet',  '🐒', 'hutan',  'https://storage.googleapis.com/a1aa/image/qSzsuL39YBR5CfR01URSfbRouZ7Q_2tImmwg-1eoKkI.jpg', '#FFF9C4', 'Monyet memiliki sidik jari yang unik, sama seperti manusia.', false),
    ('00000000-0000-0000-0003-000000000001', 'Ikan',    '🐟', 'laut',   'https://storage.googleapis.com/a1aa/image/Vt4DLo0qLYSGDuRSmGEizcrwpaywqfa5vjg5z55fQ5c.jpg', '#E3F2FD', 'Beberapa ikan bisa mengubah jenis kelaminnya sepanjang hidupnya.', true),
    ('00000000-0000-0000-0003-000000000002', 'Lumba-lumba', '🐬', 'laut', 'https://storage.googleapis.com/a1aa/image/Vt4DLo0qLYSGDuRSmGEizcrwpaywqfa5vjg5z55fQ5c.jpg', '#B2EBF2', 'Lumba-lumba tidur dengan setengah otaknya tetap terjaga.', true),
    ('00000000-0000-0000-0003-000000000003', 'Penyu',   '🐢', 'laut',   'https://storage.googleapis.com/a1aa/image/Vt4DLo0qLYSGDuRSmGEizcrwpaywqfa5vjg5z55fQ5c.jpg', '#E8F5E9', 'Penyu laut bisa hidup lebih dari 100 tahun.', false),
    ('00000000-0000-0000-0003-000000000004', 'Gurita',  '🐙', 'laut',   'https://storage.googleapis.com/a1aa/image/Vt4DLo0qLYSGDuRSmGEizcrwpaywqfa5vjg5z55fQ5c.jpg', '#F3E5F5', 'Gurita memiliki tiga jantung dan darah berwarna biru.', false)
ON CONFLICT (id) DO NOTHING;