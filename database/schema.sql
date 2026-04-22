CREATE TABLE IF NOT EXISTS accounts (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL,
    role VARCHAR(20) NOT NULL CHECK (role IN ('admin', 'guru', 'siswa')),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS kelas (
    id_kelas SERIAL PRIMARY KEY,
    tingkatan SMALLINT NOT NULL CHECK (tingkatan BETWEEN 10 AND 12),
    jurusan VARCHAR(20) NOT NULL,
    part VARCHAR(3) NOT NULL,
    CONSTRAINT kelas_unique UNIQUE (tingkatan, jurusan, part)
);

CREATE TABLE IF NOT EXISTS staff (
    id_staff SERIAL PRIMARY KEY,
    id_account INTEGER NOT NULL UNIQUE REFERENCES accounts(id) ON DELETE CASCADE,
    nip VARCHAR(20) UNIQUE,
    nama VARCHAR(255) NOT NULL,
    tipe_staff VARCHAR(10) NOT NULL CHECK (tipe_staff IN ('admin', 'guru'))
);

CREATE TABLE IF NOT EXISTS siswa (
    id_siswa SERIAL PRIMARY KEY,
    id_account INTEGER NOT NULL UNIQUE REFERENCES accounts(id) ON DELETE CASCADE,
    nis VARCHAR(20) NOT NULL UNIQUE,
    nama_siswa VARCHAR(255) NOT NULL,
    jk CHAR(1) NOT NULL CHECK (jk IN ('L', 'P')),
    id_kelas INTEGER NOT NULL REFERENCES kelas(id_kelas) ON DELETE RESTRICT,
    jurusan VARCHAR(20),
    kelas VARCHAR(50),
    academic_year CHAR(9),
    current_semester SMALLINT NOT NULL DEFAULT 1 CHECK (current_semester IN (1, 2)),
    class_status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (class_status IN ('active', 'inactive', 'graduated', 'transferred')),
    last_promotion_at TIMESTAMP NULL
);

CREATE TABLE IF NOT EXISTS jadwal_sholat (
    id_jadwal SERIAL PRIMARY KEY,
    id_kelas INTEGER NOT NULL REFERENCES kelas(id_kelas) ON DELETE CASCADE,
    jurusan VARCHAR(100),
    kelas VARCHAR(50),
    hari VARCHAR(10) NOT NULL CHECK (hari IN ('Senin', 'Selasa', 'Rabu', 'Kamis', 'Jumat', 'Sabtu', 'Ahad')),
    jenis_sholat VARCHAR(30) NOT NULL CHECK (jenis_sholat IN ('Subuh', 'Dzuhur', 'Ashar', 'Maghrib', 'Isya')),
    waktu_mulai TIME NOT NULL,
    waktu_selesai TIME NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT jadwal_waktu_check CHECK (waktu_selesai > waktu_mulai),
    CONSTRAINT jadwal_unique UNIQUE (id_kelas, hari, jenis_sholat)
);

CREATE TABLE IF NOT EXISTS sesi_sholat (
    id_sholat SERIAL PRIMARY KEY,
    nama_sholat VARCHAR(255) NOT NULL,
    waktu_mulai TIME NOT NULL,
    waktu_selesai TIME NOT NULL,
    tanggal DATE NOT NULL
);

CREATE TABLE IF NOT EXISTS absensi (
    id_absen SERIAL PRIMARY KEY,
    id_siswa INTEGER NOT NULL REFERENCES siswa(id_siswa) ON DELETE CASCADE,
    nis VARCHAR(20),
    id_jadwal INTEGER NOT NULL REFERENCES jadwal_sholat(id_jadwal) ON DELETE CASCADE,
    tanggal DATE NOT NULL,
    status VARCHAR(15) NOT NULL CHECK (status IN ('hadir', 'izin', 'sakit', 'alpha')),
    deskripsi TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT absensi_unique UNIQUE (id_jadwal, id_siswa, tanggal)
);

CREATE TABLE IF NOT EXISTS wali_kelas (
    id_wali SERIAL PRIMARY KEY,
    id_kelas INTEGER NOT NULL UNIQUE REFERENCES kelas(id_kelas) ON DELETE CASCADE,
    id_staff INTEGER NOT NULL REFERENCES staff(id_staff) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS student_class_transitions (
    id SERIAL PRIMARY KEY,
    id_siswa INTEGER NOT NULL REFERENCES siswa(id_siswa) ON DELETE CASCADE,
    action VARCHAR(30) NOT NULL CHECK (action IN ('naik_kelas', 'tinggal_kelas', 'lulus', 'pindah', 'masuk')),
    from_class INTEGER REFERENCES kelas(id_kelas),
    to_class INTEGER REFERENCES kelas(id_kelas),
    from_academic_year VARCHAR(9),
    to_academic_year VARCHAR(9),
    from_semester SMALLINT,
    to_semester SMALLINT,
    from_status VARCHAR(30),
    to_status VARCHAR(30),
    decided_by INTEGER REFERENCES staff(id_staff),
    note TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS refresh_tokens (
    id SERIAL PRIMARY KEY,
    account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    token TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS schema_migrations (
    version BIGINT PRIMARY KEY,
    dirty BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS idx_accounts_role ON accounts(role);
CREATE INDEX IF NOT EXISTS idx_staff_nip ON staff(nip);
CREATE INDEX IF NOT EXISTS idx_siswa_nis ON siswa(nis);
CREATE INDEX IF NOT EXISTS idx_siswa_id_kelas ON siswa(id_kelas);
CREATE INDEX IF NOT EXISTS idx_siswa_academic_year ON siswa(academic_year);
CREATE INDEX IF NOT EXISTS idx_siswa_class_status ON siswa(class_status);
CREATE INDEX IF NOT EXISTS idx_jadwal_sholat_hari ON jadwal_sholat(hari);
CREATE INDEX IF NOT EXISTS idx_jadwal_sholat_jenis ON jadwal_sholat(jenis_sholat);
CREATE INDEX IF NOT EXISTS idx_absensi_id_siswa ON absensi(id_siswa);
CREATE INDEX IF NOT EXISTS idx_absensi_id_jadwal ON absensi(id_jadwal);
CREATE INDEX IF NOT EXISTS idx_absensi_tanggal ON absensi(tanggal);
CREATE INDEX IF NOT EXISTS idx_student_class_transitions_id_siswa ON student_class_transitions(id_siswa);
CREATE INDEX IF NOT EXISTS idx_student_class_transitions_created_at ON student_class_transitions(created_at);
CREATE INDEX IF NOT EXISTS idx_sesi_sholat_tanggal ON sesi_sholat(tanggal);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_token ON refresh_tokens(token);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_account_id ON refresh_tokens(account_id);
