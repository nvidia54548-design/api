CREATE TABLE users_staff (
    id_staff SERIAL PRIMARY KEY,
    username VARCHAR(255) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE siswa (
    nis VARCHAR(20) PRIMARY KEY,
    nama_siswa VARCHAR(255) NOT NULL,
    jk VARCHAR(10) NOT NULL,
    jurusan VARCHAR(100),
    kelas VARCHAR(50),
    academic_year VARCHAR(9) DEFAULT '',
    current_semester SMALLINT DEFAULT 1,
    class_status VARCHAR(30) DEFAULT 'active',
    last_promotion_at TIMESTAMP NULL
);

CREATE TABLE student_class_transitions (
    id SERIAL PRIMARY KEY,
    nis VARCHAR(20) NOT NULL,
    action VARCHAR(30) NOT NULL,
    from_class VARCHAR(50),
    to_class VARCHAR(50),
    from_academic_year VARCHAR(9),
    to_academic_year VARCHAR(9),
    from_semester SMALLINT,
    to_semester SMALLINT,
    from_status VARCHAR(30),
    to_status VARCHAR(30),
    decided_by VARCHAR(255),
    note TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (nis) REFERENCES siswa(nis) ON DELETE CASCADE
);

CREATE TABLE admin (
    id_admin SERIAL PRIMARY KEY,
    id_staff INTEGER NOT NULL UNIQUE,
    nama_admin VARCHAR(255) NOT NULL,
    FOREIGN KEY (id_staff) REFERENCES users_staff(id_staff) ON DELETE CASCADE
);

CREATE TABLE guru (
    id_guru SERIAL PRIMARY KEY,
    id_staff INTEGER NOT NULL UNIQUE,
    nip VARCHAR(20) NOT NULL UNIQUE,
    nama_guru VARCHAR(255) NOT NULL,
    kelas_wali VARCHAR(50),
    FOREIGN KEY (id_staff) REFERENCES users_staff(id_staff) ON DELETE CASCADE
);

CREATE TABLE jadwal_sholat (
    id_jadwal SERIAL PRIMARY KEY,
    hari VARCHAR(20) NOT NULL,
    jenis_sholat VARCHAR(50) NOT NULL,
    waktu_mulai TIME NOT NULL,
    waktu_selesai TIME NOT NULL,
    jurusan VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE sesi_sholat (
    id_sholat SERIAL PRIMARY KEY,
    nama_sholat VARCHAR(255) NOT NULL,
    waktu_mulai TIME NOT NULL,
    waktu_selesai TIME NOT NULL,
    tanggal DATE NOT NULL
);

CREATE TABLE akun_login_siswa (
    nis VARCHAR(20) PRIMARY KEY,
    password VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL UNIQUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (nis) REFERENCES siswa(nis) ON DELETE CASCADE
);

CREATE TABLE absensi (
    id_absen SERIAL PRIMARY KEY,
    nis VARCHAR(20) NOT NULL,
    id_jadwal INTEGER NOT NULL,
    tanggal DATE NOT NULL,
    status VARCHAR(50) NOT NULL,
    deskripsi TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (nis) REFERENCES siswa(nis) ON DELETE CASCADE,
    FOREIGN KEY (id_jadwal) REFERENCES jadwal_sholat(id_jadwal) ON DELETE CASCADE
);

CREATE INDEX idx_absensi_nis ON absensi(nis);
CREATE INDEX idx_absensi_id_jadwal ON absensi(id_jadwal);
CREATE INDEX idx_absensi_tanggal ON absensi(tanggal);
CREATE INDEX idx_siswa_kelas ON siswa(kelas);
CREATE INDEX idx_siswa_academic_year ON siswa(academic_year);
CREATE INDEX idx_siswa_class_status ON siswa(class_status);
CREATE INDEX idx_student_class_transitions_nis ON student_class_transitions(nis);
CREATE INDEX idx_student_class_transitions_created_at ON student_class_transitions(created_at);
CREATE INDEX idx_jadwal_sholat_hari ON jadwal_sholat(hari);
CREATE INDEX idx_jadwal_sholat_jenis ON jadwal_sholat(jenis_sholat);
CREATE INDEX idx_sesi_sholat_tanggal ON sesi_sholat(tanggal);
CREATE INDEX idx_guru_nip ON guru(nip);
CREATE INDEX idx_users_staff_username ON users_staff(username);

CREATE TABLE refresh_tokens (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    token TEXT NOT NULL UNIQUE,
    role VARCHAR(50) NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_refresh_tokens_token ON refresh_tokens(token);
