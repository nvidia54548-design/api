-- ============================================================
-- SCHEMA DATABASE - SISTEM ABSENSI SHOLAT
-- Versi: 2.0 (sudah diperbaiki dari versi sebelumnya)
-- Perubahan utama:
--   - Hapus semua UNIQUE constraint tunggal yang salah
--   - Hanya gunakan UNIQUE constraint komposit yang logis
--   - Partial index untuk soft-delete (nis & email)
--   - Tidak ada index duplikat
--   - Tidak ada tabel refresh_token
-- ============================================================


-- ============================================================
-- EKSTENSI (opsional, aktifkan jika butuh UUID di masa depan)
-- ============================================================
-- CREATE EXTENSION IF NOT EXISTS "pgcrypto";


-- ============================================================
-- DROP URUTAN TERBALIK (jika ingin reset total)
-- Aktifkan blok ini jika perlu recreate dari awal
-- ============================================================
/*
DROP TABLE IF EXISTS "absensi"               CASCADE;
DROP TABLE IF EXISTS "rekap_absensi"         CASCADE;
DROP TABLE IF EXISTS "wali_kelas"            CASCADE;
DROP TABLE IF EXISTS "waktu_sholat"          CASCADE;
DROP TABLE IF EXISTS "jadwal_sholat_template" CASCADE;
DROP TABLE IF EXISTS "giliran_dhuha"         CASCADE;
DROP TABLE IF EXISTS "jenis_sholat"          CASCADE;
DROP TABLE IF EXISTS "semester_akademik"     CASCADE;
DROP TABLE IF EXISTS "siswa"                 CASCADE;
DROP TABLE IF EXISTS "kelas"                 CASCADE;
DROP TABLE IF EXISTS "staff"                 CASCADE;
DROP TABLE IF EXISTS "accounts"              CASCADE;
DROP TABLE IF EXISTS "tahun_masuk"           CASCADE;
*/


-- ============================================================
-- 1. accounts
-- Tabel autentikasi untuk semua pengguna (admin, guru, siswa).
-- Soft-delete: deleted_at diisi saat akun dinonaktifkan.
-- Email hanya unik untuk akun yang belum dihapus (partial index).
-- ============================================================
CREATE TABLE "accounts" (
    "id"         serial PRIMARY KEY,
    "email"      varchar(255) NOT NULL,
    "password"   varchar(255) NOT NULL,
    "role"       varchar(20)  NOT NULL,
    "created_at" timestamp    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamp    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" timestamp,
    CONSTRAINT "accounts_role_check"
        CHECK (role IN ('admin', 'guru', 'siswa'))
);

-- Email unik hanya untuk akun aktif (soft-delete safe)
CREATE UNIQUE INDEX "idx_accounts_email_active"
    ON "accounts" ("email")
    WHERE "deleted_at" IS NULL;


-- ============================================================
-- 2. tahun_masuk
-- Tahun ajaran dalam format "2024/2025".
-- is_active: hanya 1 tahun yang aktif pada satu waktu
--            (enforced di aplikasi, bukan di DB).
-- ============================================================
CREATE TABLE "tahun_masuk" (
    "id_tahun_masuk" serial    PRIMARY KEY,
    "tahun"          char(9)   NOT NULL,
    "is_active"      boolean   NOT NULL DEFAULT false,
    "created_at"     timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT "tahun_masuk_tahun_key"
        UNIQUE ("tahun"),
    CONSTRAINT "academic_year_format_check"
        CHECK (tahun ~ '^\d{4}/\d{4}$')
);


-- ============================================================
-- 3. kelas
-- Kelas sekolah: tingkatan (10-12), jurusan, dan bagian (A/B/dll).
-- UNIQUE komposit (tingkatan, jurusan, part) — bukan tunggal.
-- Contoh valid: 10-RPL-A, 10-RPL-B, 11-RPL-A bisa ada bersamaan.
-- ============================================================
CREATE TABLE "kelas" (
    "id_kelas"   serial      PRIMARY KEY,
    "tingkatan"  smallint    NOT NULL,
    "jurusan"    varchar(20) NOT NULL,
    "part"       varchar(3)  NOT NULL,
    "created_at" timestamp   NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamp   NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" timestamp,
    CONSTRAINT "kelas_tingkatan_jurusan_part_key"
        UNIQUE ("tingkatan", "jurusan", "part"),
    CONSTRAINT "kelas_tingkatan_check"
        CHECK (tingkatan BETWEEN 10 AND 12)
);


-- ============================================================
-- 4. staff
-- Data guru dan admin. Terhubung ke accounts via id_account.
-- Soft-delete: deleted_at.
-- ============================================================
CREATE TABLE "staff" (
    "id_staff"   serial      PRIMARY KEY,
    "id_account" integer     UNIQUE REFERENCES "accounts" ("id") ON DELETE CASCADE,
    "nip"        varchar(20) UNIQUE,
    "nama"       varchar(255) NOT NULL,
    "tipe_staff" varchar(10) NOT NULL,
    "created_at" timestamp   NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamp   NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" timestamp,
    CONSTRAINT "staff_tipe_staff_check"
        CHECK (tipe_staff IN ('admin', 'guru'))
);


-- ============================================================
-- 5. siswa
-- Data siswa. NIS unik hanya untuk siswa aktif (partial index).
-- Soft-delete: deleted_at.
-- class_status: active | inactive | graduated | transferred.
-- ============================================================
CREATE TABLE "siswa" (
    "id_siswa"          serial       PRIMARY KEY,
    "id_account"        integer      UNIQUE REFERENCES "accounts" ("id") ON DELETE CASCADE,
    "nis"               varchar(20)  NOT NULL,
    "nama_siswa"        varchar(255) NOT NULL,
    "jk"                char(1)      NOT NULL,
    "id_kelas"          integer      REFERENCES "kelas" ("id_kelas"),
    "id_tahun_masuk"    integer      REFERENCES "tahun_masuk" ("id_tahun_masuk"),
    "class_status"      varchar(20)  NOT NULL DEFAULT 'active',
    "last_promotion_at" timestamp,
    "academic_year"     char(9),
    "current_semester"  smallint     DEFAULT 1,
    "is_registered"     boolean      NOT NULL DEFAULT false,
    "created_at"        timestamp    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at"        timestamp    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "deleted_at"        timestamp,
    CONSTRAINT "siswa_jk_check"
        CHECK (jk IN ('L', 'P')),
    CONSTRAINT "siswa_class_status_check"
        CHECK (class_status IN ('active', 'inactive', 'graduated', 'transferred'))
);

-- NIS unik hanya untuk siswa yang belum dihapus
CREATE UNIQUE INDEX "idx_siswa_nis_active"
    ON "siswa" ("nis")
    WHERE "deleted_at" IS NULL;

-- Index bantu untuk lookup NIS + status registrasi
CREATE INDEX "idx_siswa_nis_registered"
    ON "siswa" ("nis", "is_registered");


-- ============================================================
-- 6. wali_kelas
-- Satu kelas hanya boleh punya satu wali kelas aktif.
-- Riwayat bisa disimpan (is_active = false untuk yang lama).
-- ============================================================
CREATE TABLE "wali_kelas" (
    "id_wali"      serial  PRIMARY KEY,
    "id_kelas"     integer NOT NULL UNIQUE REFERENCES "kelas" ("id_kelas"),
    "id_staff"     integer NOT NULL REFERENCES "staff" ("id_staff"),
    "is_active"    boolean NOT NULL DEFAULT true,
    "berlaku_mulai" date   NOT NULL DEFAULT CURRENT_DATE,
    "created_at"   timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP
);


-- ============================================================
-- 7. semester_akademik
-- Satu tahun ajaran punya semester 1 dan semester 2.
-- UNIQUE komposit (id_tahun, semester) — bukan tunggal.
-- ============================================================
CREATE TABLE "semester_akademik" (
    "id_semester"     serial   PRIMARY KEY,
    "id_tahun"        integer  NOT NULL REFERENCES "tahun_masuk" ("id_tahun_masuk"),
    "semester"        smallint NOT NULL,
    "tanggal_mulai"   date     NOT NULL,
    "tanggal_selesai" date     NOT NULL,
    "created_at"      timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT "semester_akademik_tahun_sem_key"
        UNIQUE ("id_tahun", "semester"),
    CONSTRAINT "semester_akademik_semester_check"
        CHECK (semester IN (1, 2)),
    CONSTRAINT "semester_akademik_tanggal_check"
        CHECK (tanggal_selesai > tanggal_mulai)
);


-- ============================================================
-- 8. jenis_sholat
-- Master jenis sholat: Subuh, Dzuhur, Ashar, Maghrib, Isya,
-- Dhuha, Jumat.
-- butuh_giliran: true hanya untuk Dhuha (ada jadwal giliran
--               per jurusan).
-- ============================================================
CREATE TABLE "jenis_sholat" (
    "id_jenis"     serial      PRIMARY KEY,
    "nama_jenis"   varchar(30) NOT NULL UNIQUE,
    "butuh_giliran" boolean    NOT NULL DEFAULT false,
    CONSTRAINT "jenis_sholat_nama_check"
        CHECK (nama_jenis IN ('Subuh','Dzuhur','Ashar','Maghrib','Isya','Dhuha','Jumat'))
);


-- ============================================================
-- 9. waktu_sholat
-- Jam mulai dan selesai per jenis sholat.
-- Bisa punya beberapa periode (berlaku_mulai s/d berlaku_sampai)
-- untuk mengakomodasi perubahan jadwal musiman.
-- berlaku_sampai NULL = masih berlaku hingga sekarang.
-- ============================================================
CREATE TABLE "waktu_sholat" (
    "id_waktu"      serial PRIMARY KEY,
    "id_jenis"      integer NOT NULL REFERENCES "jenis_sholat" ("id_jenis"),
    "waktu_mulai"   time    NOT NULL,
    "waktu_selesai" time    NOT NULL,
    "berlaku_mulai" date    NOT NULL DEFAULT CURRENT_DATE,
    "berlaku_sampai" date,
    CONSTRAINT "waktu_sholat_waktu_check"
        CHECK (waktu_selesai > waktu_mulai),
    CONSTRAINT "waktu_sholat_berlaku_check"
        CHECK (berlaku_sampai IS NULL OR berlaku_sampai > berlaku_mulai)
);


-- ============================================================
-- 10. jadwal_sholat_template
-- Template hari + jenis sholat yang dijadwalkan.
-- UNIQUE komposit (hari, id_jenis) — satu hari bisa punya
-- banyak jenis sholat (Senin: Subuh, Dzuhur, Ashar, dll).
-- ============================================================
CREATE TABLE "jadwal_sholat_template" (
    "id_template" serial      PRIMARY KEY,
    "hari"        varchar(10) NOT NULL,
    "id_jenis"    integer     NOT NULL REFERENCES "jenis_sholat" ("id_jenis"),
    "created_at"  timestamp   NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT "template_hari_jenis_key"
        UNIQUE ("hari", "id_jenis"),
    CONSTRAINT "template_hari_check"
        CHECK (hari IN ('Senin','Selasa','Rabu','Kamis','Jumat','Sabtu','Ahad'))
);


-- ============================================================
-- 11. giliran_dhuha
-- Jadwal giliran sholat Dhuha per jurusan per hari.
-- UNIQUE komposit (hari, jurusan) — satu hari bisa ada
-- banyak jurusan giliran (Senin: RPL dan TKJ, misalnya).
-- ============================================================
CREATE TABLE "giliran_dhuha" (
    "id_giliran" serial      PRIMARY KEY,
    "jurusan"    varchar(20) NOT NULL,
    "hari"       varchar(10) NOT NULL,
    "created_at" timestamp   NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT "giliran_hari_jurusan_key"
        UNIQUE ("hari", "jurusan"),
    CONSTRAINT "giliran_hari_check"
        CHECK (hari IN ('Senin','Selasa','Rabu','Kamis','Jumat'))
);


-- ============================================================
-- 12. absensi
-- Catatan kehadiran per siswa per sholat per tanggal.
-- UNIQUE komposit (id_siswa, tanggal, id_template) —
-- satu siswa hanya bisa absen 1x per jenis sholat per hari.
-- id_giliran: diisi hanya jika jenis sholatnya Dhuha.
-- ============================================================
CREATE TABLE "absensi" (
    "id_absen"    serial      PRIMARY KEY,
    "id_siswa"    integer     NOT NULL REFERENCES "siswa" ("id_siswa") ON DELETE CASCADE,
    "id_semester" integer     NOT NULL REFERENCES "semester_akademik" ("id_semester"),
    "id_template" integer     NOT NULL REFERENCES "jadwal_sholat_template" ("id_template") ON DELETE RESTRICT,
    "id_giliran"  integer     REFERENCES "giliran_dhuha" ("id_giliran") ON DELETE RESTRICT,
    "tanggal"     date        NOT NULL,
    "status"      varchar(15) NOT NULL,
    "created_at"  timestamp   NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT "absensi_siswa_tanggal_template_key"
        UNIQUE ("id_siswa", "tanggal", "id_template"),
    CONSTRAINT "absensi_status_check"
        CHECK (status IN ('hadir', 'izin', 'sakit', 'alpha'))
);

-- Index untuk query laporan per kelas/semester
CREATE INDEX "idx_absensi_semester_tanggal"
    ON "absensi" ("id_semester", "tanggal");


-- ============================================================
-- 13. rekap_absensi
-- Rekap jumlah hadir/izin/sakit/alpha per siswa per semester
-- per jenis sholat. Diupdate via trigger atau aplikasi.
-- UNIQUE komposit (id_siswa, id_semester, id_jenis).
-- ============================================================
CREATE TABLE "rekap_absensi" (
    "id_rekap"      serial  PRIMARY KEY,
    "id_siswa"      integer NOT NULL REFERENCES "siswa" ("id_siswa") ON DELETE CASCADE,
    "id_semester"   integer NOT NULL REFERENCES "semester_akademik" ("id_semester"),
    "id_jenis"      integer NOT NULL REFERENCES "jenis_sholat" ("id_jenis"),
    "jumlah_hadir"  integer NOT NULL DEFAULT 0,
    "jumlah_izin"   integer NOT NULL DEFAULT 0,
    "jumlah_sakit"  integer NOT NULL DEFAULT 0,
    "jumlah_alpha"  integer NOT NULL DEFAULT 0,
    "updated_at"    timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT "rekap_siswa_sem_jenis_key"
        UNIQUE ("id_siswa", "id_semester", "id_jenis")
);

-- Index untuk query rekap per kelas (join dengan siswa)
CREATE INDEX "idx_rekap_semester_jenis"
    ON "rekap_absensi" ("id_semester", "id_jenis");


-- ============================================================
-- SEED DATA AWAL — jenis_sholat (wajib diisi sebelum pakai)
-- ============================================================
INSERT INTO "jenis_sholat" ("nama_jenis", "butuh_giliran") VALUES
    ('Subuh',   false),
    ('Dzuhur',  false),
    ('Ashar',   false),
    ('Maghrib', false),
    ('Isya',    false),
    ('Dhuha',   true),
    ('Jumat',   false)
ON CONFLICT (nama_jenis) DO NOTHING;