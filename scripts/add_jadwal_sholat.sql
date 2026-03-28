-- Add proper jadwal sholat for Dhuha, Dzuhur, and Jumat
-- For all jurusan: TKJ, RPL, TEI, BC, DKV, ANM, TAV, TMT
-- For days: Senin to Jumat (weekdays except weekend)

-- Jurusan list
-- TKJ, RPL, TEI, BC, DKV, ANM, TAV, TMT

-- Days: Senin, Selasa, Rabu, Kamis, Jumat

-- Dhuha: 07:00 - 08:00 for all weekdays
-- Dzuhur: 12:00 - 13:00 for Senin-Thu
-- Jumat: 12:00 - 13:00 for Jumat

-- Insert Dhuha for all jurusan and weekdays
INSERT INTO jadwal_sholat (hari, jenis_sholat, waktu_mulai, waktu_selesai, jurusan) VALUES
-- TKJ
('Senin', 'Dhuha', '07:00:00', '08:00:00', 'TKJ'),
('Selasa', 'Dhuha', '07:00:00', '08:00:00', 'TKJ'),
('Rabu', 'Dhuha', '07:00:00', '08:00:00', 'TKJ'),
('Kamis', 'Dhuha', '07:00:00', '08:00:00', 'TKJ'),
('Jumat', 'Dhuha', '07:00:00', '08:00:00', 'TKJ'),

-- RPL
('Senin', 'Dhuha', '07:00:00', '08:00:00', 'RPL'),
('Selasa', 'Dhuha', '07:00:00', '08:00:00', 'RPL'),
('Rabu', 'Dhuha', '07:00:00', '08:00:00', 'RPL'),
('Kamis', 'Dhuha', '07:00:00', '08:00:00', 'RPL'),
('Jumat', 'Dhuha', '07:00:00', '08:00:00', 'RPL'),

-- TEI
('Senin', 'Dhuha', '07:00:00', '08:00:00', 'TEI'),
('Selasa', 'Dhuha', '07:00:00', '08:00:00', 'TEI'),
('Rabu', 'Dhuha', '07:00:00', '08:00:00', 'TEI'),
('Kamis', 'Dhuha', '07:00:00', '08:00:00', 'TEI'),
('Jumat', 'Dhuha', '07:00:00', '08:00:00', 'TEI'),

-- BC
('Senin', 'Dhuha', '07:00:00', '08:00:00', 'BC'),
('Selasa', 'Dhuha', '07:00:00', '08:00:00', 'BC'),
('Rabu', 'Dhuha', '07:00:00', '08:00:00', 'BC'),
('Kamis', 'Dhuha', '07:00:00', '08:00:00', 'BC'),
('Jumat', 'Dhuha', '07:00:00', '08:00:00', 'BC'),

-- DKV
('Senin', 'Dhuha', '07:00:00', '08:00:00', 'DKV'),
('Selasa', 'Dhuha', '07:00:00', '08:00:00', 'DKV'),
('Rabu', 'Dhuha', '07:00:00', '08:00:00', 'DKV'),
('Kamis', 'Dhuha', '07:00:00', '08:00:00', 'DKV'),
('Jumat', 'Dhuha', '07:00:00', '08:00:00', 'DKV'),

-- ANM
('Senin', 'Dhuha', '07:00:00', '08:00:00', 'ANM'),
('Selasa', 'Dhuha', '07:00:00', '08:00:00', 'ANM'),
('Rabu', 'Dhuha', '07:00:00', '08:00:00', 'ANM'),
('Kamis', 'Dhuha', '07:00:00', '08:00:00', 'ANM'),
('Jumat', 'Dhuha', '07:00:00', '08:00:00', 'ANM'),

-- TAV
('Senin', 'Dhuha', '07:00:00', '08:00:00', 'TAV'),
('Selasa', 'Dhuha', '07:00:00', '08:00:00', 'TAV'),
('Rabu', 'Dhuha', '07:00:00', '08:00:00', 'TAV'),
('Kamis', 'Dhuha', '07:00:00', '08:00:00', 'TAV'),
('Jumat', 'Dhuha', '07:00:00', '08:00:00', 'TAV'),

-- TMT
('Senin', 'Dhuha', '07:00:00', '08:00:00', 'TMT'),
('Selasa', 'Dhuha', '07:00:00', '08:00:00', 'TMT'),
('Rabu', 'Dhuha', '07:00:00', '08:00:00', 'TMT'),
('Kamis', 'Dhuha', '07:00:00', '08:00:00', 'TMT'),
('Jumat', 'Dhuha', '07:00:00', '08:00:00', 'TMT');

-- Insert Dzuhur for Senin to Kamis for all jurusan
INSERT INTO jadwal_sholat (hari, jenis_sholat, waktu_mulai, waktu_selesai, jurusan) VALUES
-- TKJ
('Senin', 'Dzuhur', '12:00:00', '13:00:00', 'TKJ'),
('Selasa', 'Dzuhur', '12:00:00', '13:00:00', 'TKJ'),
('Rabu', 'Dzuhur', '12:00:00', '13:00:00', 'TKJ'),
('Kamis', 'Dzuhur', '12:00:00', '13:00:00', 'TKJ'),

-- RPL
('Senin', 'Dzuhur', '12:00:00', '13:00:00', 'RPL'),
('Selasa', 'Dzuhur', '12:00:00', '13:00:00', 'RPL'),
('Rabu', 'Dzuhur', '12:00:00', '13:00:00', 'RPL'),
('Kamis', 'Dzuhur', '12:00:00', '13:00:00', 'RPL'),

-- TEI
('Senin', 'Dzuhur', '12:00:00', '13:00:00', 'TEI'),
('Selasa', 'Dzuhur', '12:00:00', '13:00:00', 'TEI'),
('Rabu', 'Dzuhur', '12:00:00', '13:00:00', 'TEI'),
('Kamis', 'Dzuhur', '12:00:00', '13:00:00', 'TEI'),

-- BC
('Senin', 'Dzuhur', '12:00:00', '13:00:00', 'BC'),
('Selasa', 'Dzuhur', '12:00:00', '13:00:00', 'BC'),
('Rabu', 'Dzuhur', '12:00:00', '13:00:00', 'BC'),
('Kamis', 'Dzuhur', '12:00:00', '13:00:00', 'BC'),

-- DKV
('Senin', 'Dzuhur', '12:00:00', '13:00:00', 'DKV'),
('Selasa', 'Dzuhur', '12:00:00', '13:00:00', 'DKV'),
('Rabu', 'Dzuhur', '12:00:00', '13:00:00', 'DKV'),
('Kamis', 'Dzuhur', '12:00:00', '13:00:00', 'DKV'),

-- ANM
('Senin', 'Dzuhur', '12:00:00', '13:00:00', 'ANM'),
('Selasa', 'Dzuhur', '12:00:00', '13:00:00', 'ANM'),
('Rabu', 'Dzuhur', '12:00:00', '13:00:00', 'ANM'),
('Kamis', 'Dzuhur', '12:00:00', '13:00:00', 'ANM'),

-- TAV
('Senin', 'Dzuhur', '12:00:00', '13:00:00', 'TAV'),
('Selasa', 'Dzuhur', '12:00:00', '13:00:00', 'TAV'),
('Rabu', 'Dzuhur', '12:00:00', '13:00:00', 'TAV'),
('Kamis', 'Dzuhur', '12:00:00', '13:00:00', 'TAV'),

-- TMT
('Senin', 'Dzuhur', '12:00:00', '13:00:00', 'TMT'),
('Selasa', 'Dzuhur', '12:00:00', '13:00:00', 'TMT'),
('Rabu', 'Dzuhur', '12:00:00', '13:00:00', 'TMT'),
('Kamis', 'Dzuhur', '12:00:00', '13:00:00', 'TMT');

-- Insert Jumat for Jumat day for all jurusan
INSERT INTO jadwal_sholat (hari, jenis_sholat, waktu_mulai, waktu_selesai, jurusan) VALUES
('Jumat', 'Jumat', '12:00:00', '13:00:00', 'TKJ'),
('Jumat', 'Jumat', '12:00:00', '13:00:00', 'RPL'),
('Jumat', 'Jumat', '12:00:00', '13:00:00', 'TEI'),
('Jumat', 'Jumat', '12:00:00', '13:00:00', 'BC'),
('Jumat', 'Jumat', '12:00:00', '13:00:00', 'DKV'),
('Jumat', 'Jumat', '12:00:00', '13:00:00', 'ANM'),
('Jumat', 'Jumat', '12:00:00', '13:00:00', 'TAV'),
('Jumat', 'Jumat', '12:00:00', '13:00:00', 'TMT');